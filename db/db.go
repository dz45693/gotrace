package db

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"tracedemo/logger"
	"unicode"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"sync"

	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/opentracing/opentracing-go"
)

// DB连接配置信息
type Config struct {
	DbHost string
	DbPort int
	DbUser string
	DbPass string
	DbName string
	Debug  bool
}

// 连接的数据库类型
const (
	dbMaster         string = "master"
	jaegerContextKey        = "jeager:context"
	callbackPrefix          = "jeager"
	startTime               = "start:time"
)

func init() {
	connMap = make(map[string]*gorm.DB)
}

var (
	connMap  map[string]*gorm.DB
	connLock sync.RWMutex
)

// 初始化DB
func InitDb(siteCode string, cfg *Config) (err error) {
	url := url.Values{}
	url.Add("parseTime", "True")
	url.Add("loc", "Local")
	url.Add("charset", "utf8mb4")
	url.Add("collation", "utf8mb4_unicode_ci")
	url.Add("readTimeout", "0s")
	url.Add("writeTimeout", "0s")
	url.Add("timeout", "0s")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%v)/%s?%s", cfg.DbUser, cfg.DbPass, cfg.DbHost, cfg.DbPort, cfg.DbName, url.Encode())

	conn, err := gorm.Open("mysql", dsn)
	if err != nil {
		return errors.Wrap(err, "fail to connect db")
	}

	//新增gorm插件
	if cfg.Debug == true {
		registerCallbacks(conn)
	}
	//打印日志
	//conn.LogMode(true)

	conn.DB().SetMaxIdleConns(30)
	conn.DB().SetMaxOpenConns(200)
	conn.DB().SetConnMaxLifetime(60 * time.Second)

	if err := conn.DB().Ping(); err != nil {
		return errors.Wrap(err, "fail to ping db")
	}

	connLock.Lock()
	dbName := fmt.Sprintf("%s-%s", siteCode, dbMaster)
	connMap[dbName] = conn
	connLock.Unlock()

	go mysqlHeart(conn)

	return nil
}

func GetMaster(ctx context.Context) *gorm.DB {
	connLock.RLock()
	defer connLock.RUnlock()

	siteCode := fmt.Sprintf("%v", ctx.Value("SiteCode"))
	if strings.Contains(siteCode, "nil") {
		panic(errors.New("当前上下文没有找到DB"))
	}

	dbName := fmt.Sprintf("%s-%s", siteCode, dbMaster)

	ctx = context.WithValue(ctx, "DbName", dbName)

	db := connMap[dbName]
	if db == nil {
		panic(errors.New(fmt.Sprintf("当前上下文没有找到DB：%s", dbName)))
	}

	return db.Set(jaegerContextKey, ctx)
}

func mysqlHeart(conn *gorm.DB) {
	for {
		if conn != nil {
			err := conn.DB().Ping()
			if err != nil {
				fmt.Println(fmt.Sprintf("mysqlHeart has err:%v", err))
			}
		}

		time.Sleep(3 * time.Minute)
	}
}

func registerCallbacks(db *gorm.DB) {
	driverName := db.Dialect().GetName()
	switch driverName {
	case "postgres":
		driverName = "postgresql"
	}
	spanTypePrefix := fmt.Sprintf("gorm.db.%s.", driverName)
	querySpanType := spanTypePrefix + "query"
	execSpanType := spanTypePrefix + "exec"

	type params struct {
		spanType  string
		processor func() *gorm.CallbackProcessor
	}
	callbacks := map[string]params{
		"gorm:create": {
			spanType:  execSpanType,
			processor: func() *gorm.CallbackProcessor { return db.Callback().Create() },
		},
		"gorm:delete": {
			spanType:  execSpanType,
			processor: func() *gorm.CallbackProcessor { return db.Callback().Delete() },
		},
		"gorm:query": {
			spanType:  querySpanType,
			processor: func() *gorm.CallbackProcessor { return db.Callback().Query() },
		},
		"gorm:update": {
			spanType:  execSpanType,
			processor: func() *gorm.CallbackProcessor { return db.Callback().Update() },
		},
		"gorm:row_query": {
			spanType:  querySpanType,
			processor: func() *gorm.CallbackProcessor { return db.Callback().RowQuery() },
		},
	}
	for name, params := range callbacks {
		params.processor().Before(name).Register(
			fmt.Sprintf("%s:before:%s", callbackPrefix, name),
			newBeforeCallback(params.spanType),
		)
		params.processor().After(name).Register(
			fmt.Sprintf("%s:after:%s", callbackPrefix, name),
			newAfterCallback(),
		)
	}
}

func newBeforeCallback(spanType string) func(*gorm.Scope) {
	return func(scope *gorm.Scope) {
		ctx, ok := scopeContext(scope)
		if !ok {
			return
		}
		//新增链路追踪
		span, ctx := opentracing.StartSpanFromContext(ctx, spanType)
		if span.Tracer() == nil {
			span.Finish()
			ctx = nil
		}
		scope.Set(jaegerContextKey, ctx)
		scope.Set(startTime, time.Now().UnixNano())
	}
}

func newAfterCallback() func(*gorm.Scope) {
	return func(scope *gorm.Scope) {
		ctx, ok := scopeContext(scope)
		if !ok {
			return
		}
		span := opentracing.SpanFromContext(ctx)
		if span == nil {
			return
		}
		defer span.Finish()

		duration := int64(0)
		if t, ok := scopeStartTime(scope); ok {
			duration = (time.Now().UnixNano() - t) / 1e6
		}

		logger.Debug(ctx, "[gorm] [%vms] [RowsReturned(%v)] %v  ", duration, scope.DB().RowsAffected, gormSQL(scope.SQL, scope.SQLVars))

		for _, err := range scope.DB().GetErrors() {
			if gorm.IsRecordNotFoundError(err) || err == errors.New("sql: no rows in result set") {
				continue
			}
			//打印错误日志
			logger.Error(ctx, "%v", err.Error())
		}
		//span.LogFields(traceLog.String("sql", scope.SQL))
	}
}

func scopeContext(scope *gorm.Scope) (context.Context, bool) {
	value, ok := scope.Get(jaegerContextKey)
	if !ok {
		return nil, false
	}
	ctx, _ := value.(context.Context)
	return ctx, ctx != nil
}

func scopeStartTime(scope *gorm.Scope) (int64, bool) {
	value, ok := scope.Get(startTime)
	if !ok {
		return 0, false
	}
	t, ok := value.(int64)
	return t, ok
}

/*===============Log=======================================*/
var (
	sqlRegexp                = regexp.MustCompile(`\?`)
	numericPlaceHolderRegexp = regexp.MustCompile(`\$\d+`)
)

func gormSQL(inputSql interface{}, value interface{}) string {
	var sql string
	var formattedValues []string
	for _, value := range value.([]interface{}) {
		indirectValue := reflect.Indirect(reflect.ValueOf(value))
		if indirectValue.IsValid() {
			value = indirectValue.Interface()
			if t, ok := value.(time.Time); ok {
				if t.IsZero() {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", "0000-00-00 00:00:00"))
				} else {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format("2006-01-02 15:04:05")))
				}
			} else if b, ok := value.([]byte); ok {
				if str := string(b); isPrintable(str) {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
				} else {
					formattedValues = append(formattedValues, "'<binary>'")
				}
			} else if r, ok := value.(driver.Valuer); ok {
				if value, err := r.Value(); err == nil && value != nil {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				} else {
					formattedValues = append(formattedValues, "NULL")
				}
			} else {
				switch value.(type) {
				case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
					formattedValues = append(formattedValues, fmt.Sprintf("%v", value))
				default:
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				}
			}
		} else {
			formattedValues = append(formattedValues, "NULL")
		}
	}

	if formattedValues == nil || len(formattedValues) < 1 {
		sql = fmt.Sprintf("%v",inputSql)
		return sql
	}

	// differentiate between $n placeholders or else treat like ?
	if numericPlaceHolderRegexp.MatchString(inputSql.(string)) {
		sql = inputSql.(string)
		for index, value := range formattedValues {
			placeholder := fmt.Sprintf(`\$%d([^\d]|$)`, index+1)
			sql = regexp.MustCompile(placeholder).ReplaceAllString(sql, value+"$1")
		}
	} else {
		formattedValuesLength := len(formattedValues)
		for index, value := range sqlRegexp.Split(inputSql.(string), -1) {
			sql += value
			if index < formattedValuesLength {
				sql += formattedValues[index]
			}
		}
	}

	return sql
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

package db

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"strings"
	"tracedemo/logger"

	"fmt"
	"net/url"

	"github.com/pkg/errors"

	"sync"

	"time"

	_ "github.com/go-sql-driver/mysql"
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
	dbMaster  string = "master"
	startTime        = "start:time"
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
	config := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		SkipDefaultTransaction: true,
	}
	config.Logger = newLogger()
	config.Logger.LogMode(gormLogger.Info)

	conn, err := gorm.Open(mysql.Open(dsn), config)
	if err != nil {
		return errors.Wrap(err, "fail to connect db")
	}

	//打印日志
	//conn.LogMode(true)
	db, err := conn.DB()
	if err != nil {
		return errors.Wrap(err, "fail to connect db")
	} else {
		db.SetMaxIdleConns(30)
		db.SetMaxOpenConns(200)
		db.SetConnMaxLifetime(60 * time.Second)
	}

	if err := db.Ping(); err != nil {
		return errors.Wrap(err, "fail to ping db")
	}

	connLock.Lock()
	dbName := fmt.Sprintf("%s-%s", siteCode, dbMaster)
	connMap[dbName] = conn
	connLock.Unlock()

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

	return db.WithContext(ctx)
}

type sqlLogger struct {
	silent bool
}

func newLogger() *sqlLogger {
	return &sqlLogger{}
}

func (s *sqlLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	switch level {
	case gormLogger.Silent:
		s.silent = true
	}
	return s
}

func (s *sqlLogger) Info(ctx context.Context, template string, args ...interface{}) {
	if !s.silent {
		logger.Info(ctx, template, args)
	}
}

func (s *sqlLogger) Warn(ctx context.Context, template string, args ...interface{}) {
	if !s.silent {
		logger.Warn(ctx, template, args)
	}
}
func (s *sqlLogger) Error(ctx context.Context, template string, args ...interface{}) {
	if !s.silent {
		logger.Error(ctx, template, args)
	}
}
func (s *sqlLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if s.silent {
		return
	}
	sql, rows := fc()
	elapsed := time.Since(begin)
	logger.Debug(ctx, "[gorm] [%vms] [RowsReturned(%v)] %v  ", elapsed, rows, sql)

}

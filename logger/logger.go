package logger

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logTimeFormat = "2006-01-02T15:04:05.000+08:00"
	zapLogger     *zap.Logger
)

//配置默认初始化
func init() {
	c := zap.NewProductionConfig()
	c.EncoderConfig.LevelKey = ""
	c.EncoderConfig.CallerKey = ""
	c.EncoderConfig.MessageKey = "logModel"
	c.EncoderConfig.TimeKey = ""
	c.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	zapLogger, _ = c.Build()
}

//初始化 Jaeger client
func NewJaegerTracer(serviceName string, agentHost string) (tracer opentracing.Tracer, closer io.Closer, err error) {
	cfg := config.Configuration{
		ServiceName: serviceName,
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeRateLimiting,
			Param: 10,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            false,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  agentHost,
		},
	}

	jLogger := log.StdLogger
	jMetricsFactory := metrics.NullFactory

	tracer, closer, err = cfg.NewTracer(config.Logger(jLogger), config.Metrics(jMetricsFactory))
	if err == nil {
		opentracing.SetGlobalTracer(tracer)
	}

	return tracer, closer, err
}

func Error(ctx context.Context, format interface{}, args ...interface{}) {
	msg := ""
	if e, ok := format.(error); ok {
		msg = fmt.Sprintf(e.Error(), args...)
	} else if e, ok := format.(string); ok {
		msg = fmt.Sprintf(e, args...)
	}

	jsonStdOut(ctx, zap.ErrorLevel, msg)
}

func Warn(ctx context.Context, format string, args ...interface{}) {
	jsonStdOut(ctx, zap.WarnLevel, fmt.Sprintf(format, args...))
}

func Info(ctx context.Context, format string, args ...interface{}) {
	jsonStdOut(ctx, zap.InfoLevel, fmt.Sprintf(format, args...))
}

func Debug(ctx context.Context, format string, args ...interface{}) {
	jsonStdOut(ctx, zap.DebugLevel, fmt.Sprintf(format, args...))
}

//本地打印 Json
func jsonStdOut(ctx context.Context, level zapcore.Level, msg string) {
	traceId, spanId := getTraceId(ctx)
	if ce := zapLogger.Check(level, "zap"); ce != nil {
		ce.Write(
			zap.Any("message", JsonLogger{
				LogTime:  time.Now().Format(logTimeFormat),
				Level:    level,
				Content:  msg,
				CallPath: getCallPath(),
				TraceId:  traceId,
				SpanId:   spanId,
			}),
		)
	}
}

type JsonLogger struct {
	TraceId  string        `json:"traceId"`
	SpanId   uint64        `json:"spanId"`
	Content  interface{}   `json:"content"`
	CallPath interface{}   `json:"callPath"`
	LogTime  string        `json:"logDate"` //日志时间
	Level    zapcore.Level `json:"level"`   //日志级别
}

func getTraceId(ctx context.Context) (string, uint64) {
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return "", 0
	}

	if sc, ok := span.Context().(jaeger.SpanContext); ok {

		return fmt.Sprintf("%v", sc.TraceID()), uint64(sc.SpanID())
	}
	return "", 0
}

func getCallPath() string {
	_, file, lineno, ok := runtime.Caller(2)
	if ok {
		return strings.Replace(fmt.Sprintf("%s:%d", stringTrim(file, ""), lineno), "%2e", ".", -1)
	}
	return ""
}

func stringTrim(s, cut string) string {
	ss := strings.SplitN(s, cut, 2)
	if len(ss) == 1 {
		return ss[0]
	}
	return ss[1]
}

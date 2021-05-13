package middleware

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"tracedemo/logger"
)

type MDCarrier struct {
	metadata.MD
}

func (m MDCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, strs := range m.MD {
		for _, v := range strs {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m MDCarrier) Set(key, val string) {
	m.MD[key] = append(m.MD[key], val)
}

// ClientInterceptor 客户端拦截器
func ClientInterceptor(tracer opentracing.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		//一个RPC调用的服务端的span，和RPC服务客户端的span构成ChildOf关系
		var parentCtx opentracing.SpanContext
		parentSpan := opentracing.SpanFromContext(ctx)
		if parentSpan != nil {
			parentCtx = parentSpan.Context()
		}
		span := tracer.StartSpan(
			method,
			opentracing.ChildOf(parentCtx),
			opentracing.Tag{Key: string(ext.Component), Value: "gRPC Client"},
			ext.SpanKindRPCClient,
		)

		defer span.Finish()
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		err := tracer.Inject(
			span.Context(),
			opentracing.TextMap,
			MDCarrier{md}, // 自定义 carrier
		)

		if err != nil {
			logger.Error(ctx, "ClientTracing inject span error :%v", err.Error())
		}

		///SiteCode
		siteCode := fmt.Sprintf("%v", ctx.Value("SiteCode"))
		if len(siteCode) < 1 || strings.Contains(siteCode, "nil") {
			siteCode = "001"
		}
		md.Set("SiteCode", siteCode)
		//
		newCtx := metadata.NewOutgoingContext(ctx, md)
		err = invoker(newCtx, method, request, reply, cc, opts...)

		if err != nil {
			logger.Error(ctx, "ClientTracing call error : %v", err.Error())
		}
		return err
	}
}

// ServerInterceptor Server 端的拦截器
func ServerTracing(tracer opentracing.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		spanContext, err := tracer.Extract(
			opentracing.TextMap,
			MDCarrier{md},
		)

		if err != nil && err != opentracing.ErrSpanContextNotFound {
			logger.Error(ctx, "ServerInterceptor extract from metadata err: %v", err)
		} else {
			span := tracer.StartSpan(
				info.FullMethod,
				ext.RPCServerOption(spanContext),
				opentracing.Tag{Key: string(ext.Component), Value: "(gRPC Server)"},
				ext.SpanKindRPCServer,
			)
			defer span.Finish()

			ctx = opentracing.ContextWithSpan(ctx, span)
		}

		return handler(ctx, req)
	}

}

//新增日志打印
func ServerSiteCode() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		//读取siteCode
		incomingContext, ok := metadata.FromIncomingContext(ctx)
		siteCode := ""
		if ok {
			siteCodeArr := incomingContext.Get("SiteCode")
			if siteCodeArr != nil && len(siteCodeArr) > 0 {
				siteCode = siteCodeArr[0]
			}
		} else {
			incomingContext = metadata.New(nil)
		}

		if len(siteCode) < 1 {
			siteCode = "001"
		}

		//设置siteCode到上下文
		c2 := context.WithValue(ctx, "001", siteCode)

		return handler(c2, req)
	}
}

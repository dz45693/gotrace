package grpcserver

import (
	"context"
	"log"
	"net"
	"tracedemo/logger"
	"tracedemo/middleware"
	pb "tracedemo/protos"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGrpcServer() {
	addr := ":9090"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[activeServer] field to listen %v,", err)
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(
		grpc_middleware.ChainUnaryServer(
			grpc_prometheus.UnaryServerInterceptor,
			middleware.ServerTracing(opentracing.GlobalTracer()), //jaeger
			middleware.ServerSiteCode(),                          //jaeger
			middleware.ServerTimeLog(),
			//grpc_recovery.UnaryServerInterceptor(),               //panic recover
		),
	))

	// 注册服务
	pb.RegisterGreeterServer(s, &server{}) // 在GRPC服务端注册服务

	reflection.Register(s)

	logger.Info(context.Background(), "[rpcServer]开始监听rpc %v,", addr)
	err = s.Serve(lis)
	if err != nil {
		logger.Error(context.Background(), "[rpcServer] 开始监听%v错误 %v", addr, err)
	}
}

/*===========================================*/
type server struct{}

func NewServer() *server {
	return &server{}
}

func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	msg := "Resuest By:" + in.Name + " Response By :" + LocalIp()
	logger.Debug(ctx,"GRPC Send: %s", msg)
	return &pb.HelloReply{Message: msg}, nil
}

func LocalIp() string {
	addrs, _ := net.InterfaceAddrs()
	var ip string = "localhost"
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	return ip
}

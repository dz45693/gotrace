package userinfo

import (
	"fmt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"tracedemo/middleware"
	pb "tracedemo/protos"
	"tracedemo/service"

	"google.golang.org/grpc"

	"github.com/kataras/iris/v12"
)

type ApiServer struct{}

func (t *ApiServer) TestUserInfo(ctx iris.Context) {
	err := service.TestUserInfo(ctx.Request().Context())
	if err != nil {
		ctx.WriteString("err:" + err.Error())
	} else {
		ctx.WriteString("ok")
	}
}

func (t *ApiServer) TestRpc(ctx iris.Context) {
	addr := "localhost:9090"
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			middleware.ClientTracing(opentracing.GlobalTracer()),
			middleware.ClientSiteCode(),
			middleware.ClientTimeLog(),
			)),
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		fmt.Println(err)
	}

	defer conn.Close()

	client := pb.NewGreeterClient(conn)
	request := &pb.HelloRequest{Name: "gavin"}
	response, err := client.SayHello(ctx.Request().Context(), request)
	if err != nil {
		ctx.WriteString("err:" + err.Error())
	} else {
		ctx.WriteString("rpc:" + response.Message)
	}
}

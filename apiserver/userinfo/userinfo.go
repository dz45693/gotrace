package userinfo

import (
	"fmt"
	pb "tracedemo/protos"
	"tracedemo/service"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"tracedemo/middleware"

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
		grpc.WithUnaryInterceptor(middleware.ClientInterceptor(opentracing.GlobalTracer())),
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

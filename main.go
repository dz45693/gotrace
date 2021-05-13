package main

import (
	"fmt"
	"os"
	"tracedemo/apiserver"
	"tracedemo/db"
	"tracedemo/grpcserver"
	"tracedemo/logger"
)

func main() {
	//init log
	jaegerHost := "192.168.100.30:6831"
	serverName, _ := os.Hostname()
	serverName = "trace-" + serverName
	_, _, err := logger.NewJaegerTracer(serverName, jaegerHost)
	if err != nil {
		fmt.Println(fmt.Sprintf("初始化JaegerTracer错误%v", err))
	}

	//初始化DB
	dbConfig := db.Config{
		DbHost: "192.168.100.30",
		DbPort: 3306,
		DbUser: "root",
		DbPass: "root",
		DbName: "demo",
		Debug:  true,
	}
	err = db.InitDb("001", &dbConfig)
	if err != nil {
		fmt.Println(fmt.Sprintf("初始化Db错误%v", err))
	}

	//启动api
	go apiserver.StartApiServerr()

	//启动GRPC
	go grpcserver.StartGrpcServer()

	select {}
}

#!/bin/bash
#cd $WORKSPACE

export GOPROXY=https://goproxy.io
 
 #根据 go.mod 文件来处理依赖关系。
go mod tidy
 
# linux环境编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main
 
# 构建docker镜像，项目中需要在当前目录下有dockerfile，否则构建失败
docker build -t trace .
docker tag  trace 192.168.100.30:8080/go/trace:2021

docker login -u admin -p '123456' 192.168.100.30:8080
docker push 192.168.100.30:8080/go/trace
 
docker rmi  trace
docker rmi 192.168.100.30:8080/go/trace:2021






package apiserver

import (
	contextV2 "context"
	"tracedemo/apiserver/userinfo"
	"tracedemo/logger"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	log "github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/opentracing/opentracing-go"
)

func StartApiServerr() {
	addr := ":8080"

	app := iris.New()
	app.Use(recover.New())
	app.Use(log.New())
	app.Use(openTracing())
	app.Use(withSiteCode())

	app.Get("/", func(c context.Context) {
		c.WriteString("pong")
	})

	initIris(app)
	logger.Info(contextV2.Background(),  "[apiServer]开始监听%s,", addr)

	err := app.Run(iris.Addr(addr), iris.WithoutInterruptHandler)
	if err != nil {
		logger.Error(contextV2.Background(), "[apiServer]开始监听%s 错误%v,", addr,err)
	}
}

func initIris(app *iris.Application) {
   api:= userinfo.ApiServer{}
	userGroup := app.Party("/user")
	{
		userGroup.Get("/test",api.TestUserInfo)
		userGroup.Get("/rpc",api.TestRpc)
	}
}

func openTracing() context.Handler {
	return func(c iris.Context) {
		span := opentracing.GlobalTracer().StartSpan("apiServer")
		c.ResetRequest(c.Request().WithContext(opentracing.ContextWithSpan(c.Request().Context(), span)))
		logger.Info(c.Request().Context(), "Api请求地址%v", c.Request().URL)
		c.Next()
	}
}

func withSiteCode() context.Handler {
	return func(c iris.Context) {
		siteCode := c.GetHeader("SiteCode")
		if len(siteCode) < 1 {
			siteCode = "001"
		}
		ctx := contextV2.WithValue(c.Request().Context(), "SiteCode", siteCode)
		c.ResetRequest(c.Request().WithContext(ctx))

		c.Next()
	}
}

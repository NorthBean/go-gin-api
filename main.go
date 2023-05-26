package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xinliangnote/go-gin-api/configs"
	"github.com/xinliangnote/go-gin-api/internal/router"
	"github.com/xinliangnote/go-gin-api/pkg/env"
	"github.com/xinliangnote/go-gin-api/pkg/logger"
	"github.com/xinliangnote/go-gin-api/pkg/shutdown"
	"github.com/xinliangnote/go-gin-api/pkg/timeutil"

	"go.uber.org/zap"
)

// @title swagger 接口文档
// @version 2.0
// @description

// @contact.name
// @contact.url
// @contact.email

// @license.name MIT
// @license.url https://github.com/xinliangnote/go-gin-api/blob/master/LICENSE

// @securityDefinitions.apikey  LoginToken
// @in                          header
// @name                        token

// @BasePath /
func main() {
	// 初始化 access logger
	accessLogger, err := logger.NewJSONLogger(
		// 禁用控制台输出
		logger.WithDisableConsole(),
		// 日志添加domain字段，添加项目名和环境标识
		logger.WithField("domain", fmt.Sprintf("%s[%s]", configs.ProjectName, env.Active().Value())),
		// 指定日志输出格式
		logger.WithTimeLayout(timeutil.CSTLayout),
		// 日志输出到文件
		logger.WithFileP(configs.ProjectAccessLogFile),
	)
	if err != nil {
		panic(err)
	}

	// 初始化 cron logger
	cronLogger, err := logger.NewJSONLogger(
		logger.WithDisableConsole(),
		logger.WithField("domain", fmt.Sprintf("%s[%s]", configs.ProjectName, env.Active().Value())),
		logger.WithTimeLayout(timeutil.CSTLayout),
		logger.WithFileP(configs.ProjectCronLogFile),
	)

	if err != nil {
		panic(err)
	}
	// 服务关闭时，手动调用zap.Logger.Sync方法将缓冲区的日志追加到文件中
	defer func() {
		_ = accessLogger.Sync()
		_ = cronLogger.Sync()
	}()

	// 初始化 HTTP 服务
	s, err := router.NewHTTPServer(accessLogger, cronLogger)
	if err != nil {
		panic(err)
	}

	server := &http.Server{
		Addr:    configs.ProjectPort,
		Handler: s.Mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			accessLogger.Fatal("http server startup err", zap.Error(err))
		}
	}()

	// 优雅关闭
	shutdown.NewHook().Close(
		// 关闭 http server
		func() {
			// 设置 10 秒超时时间的ctx
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			// TODO 后面的一系列关闭操作是否可以使用 server.RegisterOnShutdown() 来实现,RegisterOnShutdown注册的方法会在shutdown的最后执行
			if err := server.Shutdown(ctx); err != nil {
				accessLogger.Error("server shutdown err", zap.Error(err))
			}
		},

		// 关闭 db
		func() {
			if s.Db != nil {
				if err := s.Db.DbWClose(); err != nil {
					accessLogger.Error("dbw close err", zap.Error(err))
				}

				if err := s.Db.DbRClose(); err != nil {
					accessLogger.Error("dbr close err", zap.Error(err))
				}
			}
		},

		// 关闭 cache
		func() {
			if s.Cache != nil {
				if err := s.Cache.Close(); err != nil {
					accessLogger.Error("cache close err", zap.Error(err))
				}
			}
		},

		// 关闭 cron Server
		func() {
			if s.CronServer != nil {
				s.CronServer.Stop()
			}
		},
	)
}

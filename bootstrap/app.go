// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bootstrap

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/seakee/dockmon/app"
	"github.com/seakee/dockmon/app/http/middleware"
	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/sk-pkg/feishu"
	"github.com/sk-pkg/i18n"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/mysql"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type App struct {
	Config     *app.Config
	Logger     *logger.Manager
	Redis      map[string]*redis.Manager
	I18n       *i18n.Manager
	MysqlDB    map[string]*gorm.DB
	Middleware middleware.Middleware
	Mux        *gin.Engine
	Feishu     *feishu.Manager
	TraceID    *trace.ID
}

func NewApp(config *app.Config) (*App, error) {
	a := &App{Config: config, MysqlDB: map[string]*gorm.DB{}, Redis: map[string]*redis.Manager{}}

	a.loadTrace()

	ctx := context.WithValue(context.Background(), logger.TraceIDKey, a.TraceID.New())

	err := a.loadLogger(ctx)
	if err != nil {
		return nil, err
	}

	err = a.loadRedis(ctx)
	if err != nil {
		return nil, err
	}

	err = a.loadFeishu(ctx)
	if err != nil {
		return nil, err
	}

	err = a.loadI18n(ctx)
	if err != nil {
		return nil, err
	}

	err = a.loadDB(ctx)
	if err != nil {
		return nil, err
	}

	a.loadHTTPMiddlewares(ctx)
	a.loadMux(ctx)

	return a, nil
}

// Start 启动应用
func (a *App) Start() {
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, a.TraceID.New())
	// 启动HTTP服务
	go a.startHTTPServer(ctx)
	// 启动调度任务
	go a.startSchedule(ctx)
	// 启动日志采集器
	go a.startCollector(ctx)
}

// loadTrace 加载 TraceID
func (a *App) loadTrace() {
	a.TraceID = trace.NewTraceID()
}

// loadLogger 加载日志模块
func (a *App) loadLogger(ctx context.Context) error {
	var err error
	a.Logger, err = logger.New(
		logger.WithLevel(a.Config.Log.Level),
		logger.WithDriver(a.Config.Log.Driver),
		logger.WithLogPath(a.Config.Log.LogPath),
	)

	if err == nil {
		a.Logger.Info(ctx, "Loggers loaded successfully")
	}

	return err
}

// loadRedis 加载Redis模块
func (a *App) loadRedis(ctx context.Context) error {
	for _, cfg := range a.Config.Redis {
		if cfg.Enable {
			r, err := redis.New(
				redis.WithPrefix(cfg.Prefix),
				redis.WithAddress(cfg.Host),
				redis.WithPassword(cfg.Auth),
				redis.WithIdleTimeout(cfg.IdleTimeout*time.Minute),
				redis.WithMaxActive(cfg.MaxActive),
				redis.WithMaxIdle(cfg.MaxIdle),
				redis.WithDB(cfg.DB),
			)

			if err != nil {
				return err
			}

			a.Redis[cfg.Name] = r
		}
	}

	a.Logger.Info(ctx, "Redis loaded successfully")

	return nil
}

// loadI18n 加载国际化模块
func (a *App) loadI18n(ctx context.Context) error {
	var err error
	a.I18n, err = i18n.New(
		i18n.WithDebugMode(a.Config.System.DebugMode),
		i18n.WithEnvKey(a.Config.System.EnvKey),
		i18n.WithDefaultLang(a.Config.System.DefaultLang),
		i18n.WithLangDir(a.Config.System.LangDir),
	)

	if err == nil {
		a.Logger.Info(ctx, "I18n loaded successfully")
	}

	return err
}

// loadDB 加载数据库模块
func (a *App) loadDB(ctx context.Context) error {

	for _, db := range a.Config.Databases {
		if db.Enable {
			switch db.DbType {
			case "mysql":
				mysqlLogger := mysql.NewLog(a.Logger.CallerSkipMode(4))
				d, err := mysql.New(mysql.WithConfigs(
					mysql.Config{
						User:     db.DbUsername,
						Password: db.DbPassword,
						Host:     db.DbHost,
						DBName:   db.DbName,
					}),
					mysql.WithConnMaxLifetime(db.DbMaxLifetime*time.Hour),
					mysql.WithMaxIdleConn(db.DbMaxIdleConn),
					mysql.WithMaxOpenConn(db.DbMaxOpenConn),
					mysql.WithGormConfig(gorm.Config{Logger: mysqlLogger}),
				)

				if err != nil {
					return err
				}

				// if debug mode and not prod, enable gorm debug mode
				if a.Config.System.DebugMode && a.Config.System.Env != "prod" {
					d = d.Debug()
				}

				a.MysqlDB[db.DbName] = d
			case "mongo":
				// TODO mongo初始化逻辑
			}
		}
	}

	a.Logger.Info(ctx, "Databases loaded successfully")

	return nil
}

// loadFeishu 加载飞书模块
func (a *App) loadFeishu(ctx context.Context) error {
	var err error

	if a.Config.Feishu.Enable {
		a.Feishu, err = feishu.New(
			feishu.WithGroupWebhook(a.Config.Feishu.GroupWebhook),
			feishu.WithAppID(a.Config.Feishu.AppID),
			feishu.WithAppSecret(a.Config.Feishu.AppSecret),
			feishu.WithEncryptKey(a.Config.Feishu.EncryptKey),
			feishu.WithRedis(a.Redis["dockmon"]),
			feishu.WithLog(a.Logger.Zap),
		)

		if err == nil {
			a.Logger.Info(ctx, "Feishu loaded successfully")
		}
	}

	return err
}

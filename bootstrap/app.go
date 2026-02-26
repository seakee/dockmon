// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package bootstrap initializes service dependencies and starts runtime workers.
package bootstrap

import (
	"context"

	"go.uber.org/zap"
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

// App stores initialized dependencies required by HTTP APIs, schedulers, and
// collectors.
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

// NewApp creates a fully initialized application container.
//
// Parameters:
//   - config: parsed runtime configuration loaded from JSON files.
//
// Returns:
//   - *App: initialized app with logger, redis, i18n, DB, middleware, and router.
//   - error: returned when any dependency initialization step fails.
//
// Example:
//
//	cfg, _ := app.LoadConfig()
//	a, err := bootstrap.NewApp(cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
func NewApp(config *app.Config) (*App, error) {
	a := &App{Config: config, MysqlDB: map[string]*gorm.DB{}, Redis: map[string]*redis.Manager{}}

	// Trace IDs must be ready before logger initialization.
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

// Start launches all background subsystems of the application.
//
// Returns:
//   - None.
//
// Behavior:
//   - Starts HTTP server, schedule loop, and container log collector
//     concurrently.
func (a *App) Start() {
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, a.TraceID.New())
	// Start the HTTP API server.
	go a.startHTTPServer(ctx)
	// Start the cron-like scheduler.
	go a.startSchedule(ctx)
	// Start Docker container log collection.
	go a.startCollector(ctx)
}

// loadTrace initializes the trace ID generator.
//
// Returns:
//   - None.
func (a *App) loadTrace() {
	a.TraceID = trace.NewTraceID()
}

// loadLogger initializes the logger manager.
//
// Parameters:
//   - ctx: trace-aware context used for initialization logs.
//
// Returns:
//   - error: returned when logger initialization fails.
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

// loadRedis initializes configured Redis clients and stores them by name.
//
// Parameters:
//   - ctx: trace-aware context used for initialization logs.
//
// Returns:
//   - error: returned when creating any enabled Redis client fails.
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

// loadI18n initializes the i18n manager from runtime configuration.
//
// Parameters:
//   - ctx: trace-aware context used for initialization logs.
//
// Returns:
//   - error: returned when i18n initialization fails.
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

// loadDB initializes all enabled databases.
//
// Parameters:
//   - ctx: trace-aware context used for initialization logs.
//
// Returns:
//   - error: returned when any configured database cannot be initialized.
func (a *App) loadDB(ctx context.Context) error {
	for _, dbConfig := range a.Config.Databases {
		if !dbConfig.Enable {
			continue
		}

		switch dbConfig.DbType {
		case "mysql":
			// Use retry logic because containerized services may start slowly.
			d, err := a.newMysqlDBWithRetry(ctx, dbConfig)
			if err != nil {
				return err
			}

			// Enable verbose SQL logs only in non-production debug mode.
			if a.Config.System.DebugMode && a.Config.System.Env != "prod" {
				d = d.Debug()
			}

			a.MysqlDB[dbConfig.DbName] = d
		case "mongo":
			// TODO: Add MongoDB initialization logic when Mongo support is enabled.
		}
	}

	a.Logger.Info(ctx, "Databases loaded successfully")

	return nil
}

// newMysqlDBWithRetry creates a MySQL connection with configurable retry
// behavior.
//
// Parameters:
//   - ctx: trace-aware context for retry logs and cancellation.
//   - dbConfig: database configuration including DSN parts and retry policy.
//
// Returns:
//   - *gorm.DB: initialized GORM client.
//   - error: returned when all retry attempts fail or context is canceled.
//
// Behavior:
//   - Defaults to 3 retries with 3-second intervals when not configured.
//   - Stops early when context cancellation is received.
func (a *App) newMysqlDBWithRetry(ctx context.Context, dbConfig app.Databases) (*gorm.DB, error) {
	retryCount := dbConfig.DbConnectRetryCount
	if retryCount <= 0 {
		retryCount = 3
	}

	retryInterval := dbConfig.DbConnectRetryInterval
	if retryInterval <= 0 {
		retryInterval = 3
	}

	mysqlLogger := mysql.NewLog(a.Logger.CallerSkipMode(4))
	var (
		d   *gorm.DB
		err error
	)

	for attempt := 1; attempt <= retryCount; attempt++ {
		d, err = mysql.New(mysql.WithConfigs(
			mysql.Config{
				User:     dbConfig.DbUsername,
				Password: dbConfig.DbPassword,
				Host:     dbConfig.DbHost,
				DBName:   dbConfig.DbName,
			}),
			mysql.WithConnMaxLifetime(dbConfig.DbMaxLifetime*time.Hour),
			mysql.WithMaxIdleConn(dbConfig.DbMaxIdleConn),
			mysql.WithMaxOpenConn(dbConfig.DbMaxOpenConn),
			mysql.WithGormConfig(gorm.Config{Logger: mysqlLogger}),
		)
		if err == nil {
			return d, nil
		}

		if attempt == retryCount {
			break
		}

		waitTime := time.Duration(retryInterval) * time.Second
		a.Logger.Warn(
			ctx, "database connection failed, preparing retry",
			zap.String("dbName", dbConfig.DbName),
			zap.String("host", dbConfig.DbHost),
			zap.Int("attempt", attempt),
			zap.Int("maxAttempts", retryCount),
			zap.Duration("retryAfter", waitTime),
			zap.Error(err),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return nil, err
}

// loadFeishu initializes Feishu integration when enabled.
//
// Parameters:
//   - ctx: trace-aware context used for initialization logs.
//
// Returns:
//   - error: returned when Feishu initialization fails.
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

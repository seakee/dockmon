// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package middleware provides shared Gin middleware used by dockmon APIs.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/seakee/dockmon/app/pkg/trace"
	"github.com/sk-pkg/i18n"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	// Middleware groups all middleware factories used by routers.
	Middleware interface {
		// CheckAppAuth validates app-level JWT tokens for protected endpoints.
		CheckAppAuth() gin.HandlerFunc

		// Cors adds CORS headers and handles preflight requests.
		Cors() gin.HandlerFunc

		// RequestLogger emits structured logs for incoming requests.
		RequestLogger() gin.HandlerFunc

		// SetTraceID attaches trace IDs to requests and responses.
		SetTraceID() gin.HandlerFunc
	}

	// middleware is the default Middleware implementation.
	middleware struct {
		logger  *logger.Manager
		i18n    *i18n.Manager
		db      map[string]*gorm.DB
		redis   map[string]*redis.Manager
		traceID *trace.ID
	}
)

// New creates a middleware factory with shared runtime dependencies.
//
// Parameters:
//   - logger: structured logger manager.
//   - i18n: i18n manager used by auth middleware responses.
//   - db: database map used by downstream middlewares.
//   - redis: redis map used by downstream middlewares.
//   - traceID: trace ID generator.
//
// Returns:
//   - Middleware: middleware factory ready to register into Gin.
func New(logger *logger.Manager, i18n *i18n.Manager, db map[string]*gorm.DB, redis map[string]*redis.Manager, traceID *trace.ID) Middleware {
	return &middleware{logger: logger, i18n: i18n, db: db, redis: redis, traceID: traceID}
}

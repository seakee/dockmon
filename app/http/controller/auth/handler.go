// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package auth provides HTTP handlers for server app authentication endpoints.
package auth

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/seakee/dockmon/app/repository/auth"
	"github.com/sk-pkg/i18n"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	// Handler defines HTTP handlers for app management and token issuance.
	Handler interface {
		// i is an unexported marker method used to seal this interface.
		i()
		// ctx builds a request-scoped context with trace metadata.
		ctx(c *gin.Context) context.Context
		// Create handles app registration.
		Create() gin.HandlerFunc
		// GetToken handles token issuance for server apps.
		GetToken() gin.HandlerFunc
	}

	// handler is the concrete implementation of Handler.
	handler struct {
		logger *logger.Manager
		redis  *redis.Manager
		i18n   *i18n.Manager
		repo   auth.Repo
	}
)

// ctx builds a context carrying the trace ID from Gin context.
//
// Parameters:
//   - c: current Gin context for one HTTP request.
//
// Returns:
//   - context.Context: background-derived context with trace metadata.
func (h handler) ctx(c *gin.Context) context.Context {
	traceID, _ := c.Get("trace_id")

	return context.WithValue(context.Background(), logger.TraceIDKey, traceID.(string))
}

// i is a marker method that prevents external implementations.
//
// Returns:
//   - None.
func (h handler) i() {}

// New creates an auth handler with repository and infrastructure dependencies.
//
// Parameters:
//   - logger: structured logger manager.
//   - redis: redis manager for repository/service integration.
//   - i18n: i18n manager for localized API responses.
//   - db: GORM database client for auth persistence.
//
// Returns:
//   - Handler: initialized auth HTTP handler.
//
// Example:
//
//	h := auth.New(logger, redis, i18n, db)
func New(logger *logger.Manager, redis *redis.Manager, i18n *i18n.Manager, db *gorm.DB) Handler {
	return &handler{
		logger: logger,
		redis:  redis,
		i18n:   i18n,
		repo:   auth.NewAppRepo(db, redis),
	}
}

// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package router wires HTTP route groups and registers controller handlers.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/seakee/dockmon/app/http/middleware"
	"github.com/sk-pkg/i18n"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type Core struct {
	Logger     *logger.Manager
	Redis      map[string]*redis.Manager
	I18n       *i18n.Manager
	MysqlDB    map[string]*gorm.DB
	Middleware middleware.Middleware
}

// New registers internal and external API groups under /dockmon.
//
// Parameters:
//   - mux: gin engine that receives route registrations.
//   - core: shared dependency container for handlers.
//
// Returns:
//   - *gin.Engine: the same engine after route registration.
//
// Example:
//
//	router.New(mux, core)
func New(mux *gin.Engine, core *Core) *gin.Engine {
	api := mux.Group("dockmon")
	// Register internal APIs used by trusted services.
	internal(api.Group("internal"), core)
	// Register external APIs exposed to app clients.
	external(api.Group("external"), core)

	return mux
}

// external registers routes intended for external callers.
//
// Parameters:
//   - api: route group for external endpoints.
//   - core: shared dependency container.
//
// Returns:
//   - None.
func external(api *gin.RouterGroup, core *Core) {
	api.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})

	// App health-check endpoints.
	appGroup := api.Group("app")
	appGroup.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})

	// Service health-check endpoints.
	serviceGroup := api.Group("service")
	serviceGroup.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})
}

// internal registers routes intended for internal service calls.
//
// Parameters:
//   - api: route group for internal endpoints.
//   - core: shared dependency container.
//
// Returns:
//   - None.
func internal(api *gin.RouterGroup, core *Core) {
	api.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})

	// Admin health-check endpoints.
	adminGroup := api.Group("admin")
	adminGroup.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})

	// Service endpoints, including auth APIs.
	serviceGroup := api.Group("service")
	serviceGroup.GET("ping", func(c *gin.Context) {
		core.I18n.JSON(c, 0, nil, nil)
	})

	authGroup(serviceGroup.Group("server/auth"), core)
}

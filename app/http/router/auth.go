package router

import (
	"github.com/gin-gonic/gin"
	"github.com/seakee/dockmon/app/http/controller/auth"
)

func authGroup(api *gin.RouterGroup, core *Core) {
	authHandler := auth.New(core.Logger, core.Redis["dockmon"], core.I18n, core.MysqlDB["dockmon"])
	{
		api.POST("app", core.Middleware.CheckAppAuth(), authHandler.Create())
		api.POST("token", authHandler.GetToken())
	}
}

package router

import (
	v1 "1Panel/backend/app/api/v1"
	"1Panel/backend/middleware"
	"github.com/gin-gonic/gin"
)

type WebsiteGroupRouter struct {
}

func (a *WebsiteGroupRouter) InitWebsiteGroupRouter(Router *gin.RouterGroup) {
	groupRouter := Router.Group("groups")
	groupRouter.Use(middleware.JwtAuth()).Use(middleware.SessionAuth()).Use(middleware.PasswordExpired())

	baseApi := v1.ApiGroupApp.BaseApi
	{
		groupRouter.POST("", baseApi.CreateGroup)
		groupRouter.POST("/del", baseApi.DeleteGroup)
		groupRouter.POST("/update", baseApi.UpdateGroup)
		groupRouter.POST("/search", baseApi.ListGroup)
	}
}

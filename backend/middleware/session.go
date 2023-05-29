package middleware

import (
	"errors"
	"strconv"

	"1Panel/backend/app/api/v1/helper"
	"1Panel/backend/app/repo"
	"1Panel/backend/constant"
	"1Panel/backend/global"
	"github.com/gin-gonic/gin"
)

func SessionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		xPanelUsername := c.GetHeader("X-Panel-Username")
		xPanelPassword := c.GetHeader("X-Panel-Password")
		if xPanelUsername != "" && xPanelPassword != "" {
			if global.CONF.System.Username != xPanelUsername || global.CONF.System.Password != xPanelPassword {
				helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, errors.New("X Panel Error"))
			} else {
				c.Next()
			}
			return
		}

		if method, exist := c.Get("authMethod"); exist && method == constant.AuthMethodJWT {
			c.Next()
			return
		}
		sId, err := c.Cookie(constant.SessionName)
		if err != nil {
			helper.ErrorWithDetail(c, constant.CodeErrUnauthorized, constant.ErrTypeNotLogin, nil)
			return
		}
		psession, err := global.SESSION.Get(sId)
		if err != nil {
			helper.ErrorWithDetail(c, constant.CodeErrUnauthorized, constant.ErrTypeNotLogin, nil)
			return
		}
		settingRepo := repo.NewISettingRepo()
		setting, err := settingRepo.Get(settingRepo.WithByKey("SessionTimeout"))
		if err != nil {
			global.LOG.Errorf("create operation record failed, err: %v", err)
		}
		lifeTime, _ := strconv.Atoi(setting.Value)
		_ = global.SESSION.Set(sId, psession, lifeTime)
		c.Next()
	}
}

package middleware

import (
	"1Panel/backend/global"
	"errors"
	"strconv"
	"time"

	"1Panel/backend/app/api/v1/helper"
	"1Panel/backend/app/repo"
	"1Panel/backend/constant"
	"1Panel/backend/utils/common"
	"github.com/gin-gonic/gin"
)

func PasswordExpired() gin.HandlerFunc {
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

		settingRepo := repo.NewISettingRepo()
		setting, err := settingRepo.Get(settingRepo.WithByKey("ExpirationDays"))
		if err != nil {
			helper.ErrorWithDetail(c, constant.CodePasswordExpired, constant.ErrTypePasswordExpired, err)
			return
		}
		expiredDays, _ := strconv.Atoi(setting.Value)
		if expiredDays == 0 {
			c.Next()
			return
		}

		extime, err := settingRepo.Get(settingRepo.WithByKey("ExpirationTime"))
		if err != nil {
			helper.ErrorWithDetail(c, constant.CodePasswordExpired, constant.ErrTypePasswordExpired, err)
			return
		}
		loc, _ := time.LoadLocation(common.LoadTimeZone())
		expiredTime, err := time.ParseInLocation("2006-01-02 15:04:05", extime.Value, loc)
		if err != nil {
			helper.ErrorWithDetail(c, constant.CodePasswordExpired, constant.ErrTypePasswordExpired, err)
			return
		}
		if time.Now().After(expiredTime) {
			helper.ErrorWithDetail(c, constant.CodePasswordExpired, constant.ErrTypePasswordExpired, nil)
			return
		}
		c.Next()
	}
}

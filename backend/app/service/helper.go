package service

import (
	"context"
	"1Panel/backend/constant"
	"1Panel/backend/global"
	"gorm.io/gorm"
)

func getTxAndContext() (tx *gorm.DB, ctx context.Context) {
	tx = global.DB.Begin()
	ctx = context.WithValue(context.Background(), constant.DB, tx)
	return
}

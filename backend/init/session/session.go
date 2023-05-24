package session

import (
	"1Panel/backend/global"
	"1Panel/backend/init/session/psession"
)

func Init() {
	global.SESSION = psession.NewPSession(global.CACHE)
	global.LOG.Info("init session successfully")
}

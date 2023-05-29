package constant

import (
	"path"

	"1Panel/backend/global"
)

var (
	DataDir             = global.CONF.System.DataDir
	ResourceDir         = path.Join(DataDir, "resource")
	AppResourceDir      = path.Join(ResourceDir, "apps")
	AppInstallDir       = path.Join(DataDir, "apps")
	LocalAppResourceDir = path.Join(ResourceDir, "localApps")
	LocalAppInstallDir  = path.Join(DataDir, "localApps")
	RuntimeDir          = path.Join(DataDir, "runtime")
)

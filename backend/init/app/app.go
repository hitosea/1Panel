package app

import (
	"1Panel/backend/utils/docker"
	"path"

	"1Panel/backend/constant"
	"1Panel/backend/global"
	"1Panel/backend/utils/files"
)

func Init() {
	constant.DataDir = global.CONF.System.DataDir
	constant.ResourceDir = path.Join(constant.DataDir, "resource")
	constant.AppResourceDir = path.Join(constant.ResourceDir, "apps")
	constant.AppInstallDir = path.Join(constant.DataDir, "apps")
	constant.RuntimeDir = path.Join(constant.DataDir, "runtime")
	constant.LocalAppResourceDir = path.Join(constant.ResourceDir, "localApps")
	constant.LocalAppInstallDir = path.Join(constant.DataDir, "localApps")

	dirs := []string{constant.DataDir, constant.ResourceDir, constant.AppResourceDir, constant.AppInstallDir, global.CONF.System.Backup, constant.RuntimeDir, constant.LocalAppResourceDir}

	fileOp := files.NewFileOp()
	for _, dir := range dirs {
		createDir(fileOp, dir)
	}

	_ = docker.CreateDefaultDockerNetwork()
}

func createDir(fileOp files.FileOp, dirPath string) {
	if !fileOp.Stat(dirPath) {
		_ = fileOp.CreateDir(dirPath, 0755)
	}
}

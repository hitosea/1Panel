package service

import (
	"1Panel/backend/app/api/v1/helper"
	"context"
	"encoding/json"
	"fmt"
	"github.com/compose-spec/compose-go/types"
	"github.com/subosito/gotenv"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strconv"
	"strings"

	"1Panel/backend/app/repo"
	"1Panel/backend/utils/env"

	"1Panel/backend/app/dto/response"
	"1Panel/backend/buserr"

	"1Panel/backend/app/dto"
	"1Panel/backend/app/model"
	"1Panel/backend/constant"
	"1Panel/backend/global"
	"1Panel/backend/utils/common"
	"1Panel/backend/utils/compose"
	composeV2 "1Panel/backend/utils/docker"
	"1Panel/backend/utils/files"
	"github.com/pkg/errors"
)

type DatabaseOp string

var (
	Add    DatabaseOp = "add"
	Delete DatabaseOp = "delete"
)

func checkPort(key string, params map[string]interface{}) (int, error) {
	port, ok := params[key]
	if ok {
		portN := int(math.Ceil(port.(float64)))

		oldInstalled, _ := appInstallRepo.ListBy(appInstallRepo.WithPort(portN))
		if len(oldInstalled) > 0 {
			var apps []string
			for _, install := range oldInstalled {
				apps = append(apps, install.App.Name)
			}
			return portN, buserr.WithMap(constant.ErrPortInOtherApp, map[string]interface{}{"port": portN, "apps": apps}, nil)
		}
		if common.ScanPort(portN) {
			return portN, buserr.WithDetail(constant.ErrPortInUsed, portN, nil)
		} else {
			return portN, nil
		}
	}
	return 0, nil
}

func createLink(ctx context.Context, app model.App, appInstall *model.AppInstall, params map[string]interface{}) error {
	var dbConfig dto.AppDatabase
	if app.Type == "runtime" {
		var authParam dto.AuthParam
		paramByte, err := json.Marshal(params)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(paramByte, &authParam); err != nil {
			return err
		}
		if authParam.RootPassword != "" {
			authByte, err := json.Marshal(authParam)
			if err != nil {
				return err
			}
			appInstall.Param = string(authByte)
		}
	}
	if app.Type == "website" || app.Type == "tool" {
		paramByte, err := json.Marshal(params)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(paramByte, &dbConfig); err != nil {
			return err
		}
	}

	if !reflect.DeepEqual(dbConfig, dto.AppDatabase{}) && dbConfig.ServiceName != "" {
		dbInstall, err := appInstallRepo.GetFirst(appInstallRepo.WithServiceName(dbConfig.ServiceName))
		if err != nil {
			return err
		}
		var resourceId uint
		if dbConfig.DbName != "" && dbConfig.DbUser != "" && dbConfig.Password != "" {
			iMysqlRepo := repo.NewIMysqlRepo()
			oldMysqlDb, _ := iMysqlRepo.Get(commonRepo.WithByName(dbConfig.DbName))
			resourceId = oldMysqlDb.ID
			if oldMysqlDb.ID > 0 {
				if oldMysqlDb.Username != dbConfig.DbUser || oldMysqlDb.Password != dbConfig.Password {
					return buserr.New(constant.ErrDbUserNotValid)
				}
			} else {
				var createMysql dto.MysqlDBCreate
				createMysql.Name = dbConfig.DbName
				createMysql.Username = dbConfig.DbUser
				createMysql.Format = "utf8mb4"
				createMysql.Permission = "%"
				createMysql.Password = dbConfig.Password
				mysqldb, err := NewIMysqlService().Create(ctx, createMysql)
				if err != nil {
					return err
				}
				resourceId = mysqldb.ID
			}
		}
		var installResource model.AppInstallResource
		installResource.ResourceId = resourceId
		installResource.AppInstallId = appInstall.ID
		installResource.LinkId = dbInstall.ID
		installResource.Key = dbInstall.App.Key
		if err := appInstallResourceRepo.Create(ctx, &installResource); err != nil {
			return err
		}
	}
	return nil
}

func handleAppInstallErr(ctx context.Context, install *model.AppInstall) error {
	op := files.NewFileOp()
	appDir := install.GetPath()
	dir, _ := os.Stat(appDir)
	if dir != nil {
		_, _ = compose.Down(install.GetComposePath())
		if err := op.DeleteDir(appDir); err != nil {
			return err
		}
	}
	if err := deleteLink(ctx, install, true, true); err != nil {
		return err
	}
	return nil
}

func replaceYamlArgs(service map[string]interface{}, changeKeys map[string]string, arg string) {
	if v1, ok1 := service[arg]; ok1 {
		bs, err := json.Marshal(v1)
		if err != nil {
			return
		}
		var args []string
		err = json.Unmarshal(bs, &args)
		if err != nil {
			return
		}
		var tmpArgs []interface{}
		for _, arg1 := range args {
			for k2, v2 := range changeKeys {
				if arg1 == k2 {
					tmpArgs = append(tmpArgs, v2)
				}
			}
		}
		service[arg] = tmpArgs
	}
}

func replaceEnvironment(service map[string]interface{}, changeKeys map[string]string) {
	if v1, ok1 := service["environment"]; ok1 {
		envs := v1.(map[string]interface{})
		for k, v := range envs {
			value := v.(string)
			for k2, v2 := range changeKeys {
				if value == k2 {
					envs[k] = v2
				}
			}
		}
	}
}

func deleteAppInstall(install model.AppInstall, deleteBackup bool, forceDelete bool, deleteDB bool) error {
	op := files.NewFileOp()
	appDir := install.GetPath()
	dir, _ := os.Stat(appDir)
	if dir != nil {
		out, err := compose.Down(install.GetComposePath())
		if err != nil && !forceDelete {
			return handleErr(install, err, out)
		}
	}
	tx, ctx := helper.GetTxAndContext()
	defer tx.Rollback()
	if err := appInstallRepo.Delete(ctx, install); err != nil {
		return err
	}
	if err := deleteLink(ctx, &install, deleteDB, forceDelete); err != nil && !forceDelete {
		return err
	}
	_ = backupRepo.DeleteRecord(ctx, commonRepo.WithByType("app"), commonRepo.WithByName(install.App.Key), backupRepo.WithByDetailName(install.Name))
	_ = backupRepo.DeleteRecord(ctx, commonRepo.WithByType(install.App.Key))
	if install.App.Key == constant.AppMysql {
		_ = mysqlRepo.DeleteAll(ctx)
	}
	uploadDir := fmt.Sprintf("%s/1panel/uploads/app/%s/%s", global.CONF.System.BaseDir, install.App.Key, install.Name)
	if _, err := os.Stat(uploadDir); err == nil {
		_ = os.RemoveAll(uploadDir)
	}
	if deleteBackup {
		localDir, _ := loadLocalDir()
		backupDir := fmt.Sprintf("%s/app/%s/%s", localDir, install.App.Key, install.Name)
		if _, err := os.Stat(backupDir); err == nil {
			_ = os.RemoveAll(backupDir)
		}
		global.LOG.Infof("delete app %s-%s backups successful", install.App.Key, install.Name)
	}
	_ = op.DeleteDir(appDir)
	tx.Commit()
	return nil
}

func deleteLink(ctx context.Context, install *model.AppInstall, deleteDB bool, forceDelete bool) error {
	resources, _ := appInstallResourceRepo.GetBy(appInstallResourceRepo.WithAppInstallId(install.ID))
	if len(resources) == 0 {
		return nil
	}
	for _, re := range resources {
		mysqlService := NewIMysqlService()
		if re.Key == "mysql" && deleteDB {
			database, _ := mysqlRepo.Get(commonRepo.WithByID(re.ResourceId))
			if reflect.DeepEqual(database, model.DatabaseMysql{}) {
				continue
			}
			if err := mysqlService.Delete(ctx, dto.MysqlDBDelete{
				ID:          database.ID,
				ForceDelete: forceDelete,
			}); err != nil && !forceDelete {
				return err
			}
		}
	}
	return appInstallResourceRepo.DeleteBy(ctx, appInstallResourceRepo.WithAppInstallId(install.ID))
}

func upgradeInstall(installId uint, detailId uint) error {
	install, err := appInstallRepo.GetFirst(commonRepo.WithByID(installId))
	if err != nil {
		return err
	}
	detail, err := appDetailRepo.GetFirst(commonRepo.WithByID(detailId))
	if err != nil {
		return err
	}
	if install.Version == detail.Version {
		return errors.New("two version is same")
	}
	if err := NewIBackupService().AppBackup(dto.CommonBackup{Name: install.App.Key, DetailName: install.Name}); err != nil {
		return err
	}

	detailDir := path.Join(constant.ResourceDir, "apps", install.App.Key, "versions", detail.Version)
	if install.App.Resource == constant.AppResourceLocal {
		detailDir = path.Join(constant.ResourceDir, "localApps", strings.TrimPrefix(install.App.Key, "local"), "versions", detail.Version)
	}

	cmd := exec.Command("/bin/bash", "-c", fmt.Sprintf("cp -rf %s/* %s", detailDir, install.GetPath()))
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		if stdout != nil {
			return errors.New(string(stdout))
		}
		return err
	}

	if out, err := compose.Down(install.GetComposePath()); err != nil {
		if out != "" {
			return errors.New(out)
		}
		return err
	}
	install.DockerCompose = detail.DockerCompose
	install.Version = detail.Version
	install.AppDetailId = detailId

	fileOp := files.NewFileOp()
	if err := fileOp.WriteFile(install.GetComposePath(), strings.NewReader(install.DockerCompose), 0775); err != nil {
		return err
	}
	if out, err := compose.Up(install.GetComposePath()); err != nil {
		if out != "" {
			return errors.New(out)
		}
		return err
	}
	return appInstallRepo.Save(context.Background(), &install)
}

func getContainerNames(install model.AppInstall) ([]string, error) {
	envStr, err := coverEnvJsonToStr(install.Env)
	if err != nil {
		return nil, err
	}
	project, err := composeV2.GetComposeProject(install.Name, install.GetPath(), []byte(install.DockerCompose), []byte(envStr), true)
	if err != nil {
		return nil, err
	}
	containerMap := make(map[string]struct{})
	containerMap[install.ContainerName] = struct{}{}
	for _, service := range project.AllServices() {
		if service.ContainerName == "${CONTAINER_NAME}" || service.ContainerName == "" {
			continue
		}
		containerMap[service.ContainerName] = struct{}{}
	}
	var containerNames []string
	for k := range containerMap {
		containerNames = append(containerNames, k)
	}
	return containerNames, nil
}

func coverEnvJsonToStr(envJson string) (string, error) {
	envMap := make(map[string]interface{})
	_ = json.Unmarshal([]byte(envJson), &envMap)
	newEnvMap := make(map[string]string, len(envMap))
	handleMap(envMap, newEnvMap)
	envStr, err := gotenv.Marshal(newEnvMap)
	if err != nil {
		return "", err
	}
	return envStr, nil
}

func checkLimit(app model.App) error {
	if app.Limit > 0 {
		installs, err := appInstallRepo.ListBy(appInstallRepo.WithAppId(app.ID))
		if err != nil {
			return err
		}
		if len(installs) >= app.Limit {
			return buserr.New(constant.ErrAppLimit)
		}
	}
	return nil
}

func checkRequiredAndLimit(app model.App) error {
	if err := checkLimit(app); err != nil {
		return err
	}
	if app.Required != "" {
		var requiredArray []string
		if err := json.Unmarshal([]byte(app.Required), &requiredArray); err != nil {
			return err
		}
		for _, key := range requiredArray {
			if key == "" {
				continue
			}
			requireApp, err := appRepo.GetFirst(appRepo.WithKey(key))
			if err != nil {
				return err
			}
			details, err := appDetailRepo.GetBy(appDetailRepo.WithAppId(requireApp.ID))
			if err != nil {
				return err
			}
			var detailIds []uint
			for _, d := range details {
				detailIds = append(detailIds, d.ID)
			}

			_, err = appInstallRepo.GetFirst(appInstallRepo.WithDetailIdsIn(detailIds))
			if err != nil {
				return buserr.WithDetail(constant.ErrAppRequired, requireApp.Name, nil)
			}
		}
	}
	return nil
}

func handleMap(params map[string]interface{}, envParams map[string]string) {
	for k, v := range params {
		switch t := v.(type) {
		case string:
			envParams[k] = t
		case float64:
			envParams[k] = strconv.FormatFloat(t, 'f', -1, 32)
		default:
			envParams[k] = t.(string)
		}
	}
}

func copyAppData(key, version, installName string, params map[string]interface{}, isLocal bool) (err error) {
	fileOp := files.NewFileOp()
	appResourceDir := constant.AppResourceDir
	installAppDir := path.Join(constant.AppInstallDir, key)
	appKey := key
	if isLocal {
		appResourceDir = constant.LocalAppResourceDir
		appKey = strings.TrimPrefix(key, "local")
		installAppDir = path.Join(constant.LocalAppInstallDir, appKey)
	}
	resourceDir := path.Join(appResourceDir, appKey, "versions", version)

	if !fileOp.Stat(installAppDir) {
		if err = fileOp.CreateDir(installAppDir, 0755); err != nil {
			return
		}
	}
	appDir := path.Join(installAppDir, installName)
	if fileOp.Stat(appDir) {
		if err = fileOp.DeleteDir(appDir); err != nil {
			return
		}
	}
	if err = fileOp.Copy(resourceDir, installAppDir); err != nil {
		return
	}
	versionDir := path.Join(installAppDir, version)
	if err = fileOp.Rename(versionDir, appDir); err != nil {
		return
	}
	envPath := path.Join(appDir, ".env")

	envParams := make(map[string]string, len(params))
	handleMap(params, envParams)
	if err = env.Write(envParams, envPath); err != nil {
		return
	}
	return
}

// 处理文件夹权限等问题
func upAppPre(app model.App, appInstall *model.AppInstall) error {
	if app.Key == "nexus" {
		dataPath := path.Join(appInstall.GetPath(), "data")
		if err := files.NewFileOp().Chown(dataPath, 200, 0); err != nil {
			return err
		}
	}
	return nil
}

func getServiceFromInstall(appInstall *model.AppInstall) (service *composeV2.ComposeService, err error) {
	var (
		project *types.Project
		envStr  string
	)
	envStr, err = coverEnvJsonToStr(appInstall.Env)
	if err != nil {
		return
	}
	project, err = composeV2.GetComposeProject(appInstall.Name, appInstall.GetPath(), []byte(appInstall.DockerCompose), []byte(envStr), true)
	if err != nil {
		return
	}
	service, err = composeV2.NewComposeService()
	if err != nil {
		return
	}
	service.SetProject(project)
	return
}

func upApp(appInstall *model.AppInstall) {
	upProject := func(appInstall *model.AppInstall) (err error) {
		if err == nil {
			var composeService *composeV2.ComposeService
			composeService, err = getServiceFromInstall(appInstall)
			if err != nil {
				return err
			}
			err = composeService.ComposeUp()
			if err != nil {
				return err
			}
			return
		} else {
			return
		}
	}
	if err := upProject(appInstall); err != nil {
		appInstall.Status = constant.Error
		appInstall.Message = err.Error()
	} else {
		appInstall.Status = constant.Running
	}
	exist, _ := appInstallRepo.GetFirst(commonRepo.WithByID(appInstall.ID))
	if exist.ID > 0 {
		_ = appInstallRepo.Save(context.Background(), appInstall)
	}
}

func rebuildApp(appInstall model.AppInstall) error {
	dockerComposePath := appInstall.GetComposePath()
	out, err := compose.Down(dockerComposePath)
	if err != nil {
		return handleErr(appInstall, err, out)
	}
	out, err = compose.Up(dockerComposePath)
	if err != nil {
		return handleErr(appInstall, err, out)
	}
	return syncById(appInstall.ID)
}

func getAppDetails(details []model.AppDetail, versions []string) map[string]model.AppDetail {
	appDetails := make(map[string]model.AppDetail, len(details))
	for _, old := range details {
		old.Status = constant.AppTakeDown
		appDetails[old.Version] = old
	}

	for _, v := range versions {
		detail, ok := appDetails[v]
		if ok {
			detail.Status = constant.AppNormal
			appDetails[v] = detail
		} else {
			appDetails[v] = model.AppDetail{
				Version: v,
				Status:  constant.AppNormal,
			}
		}
	}
	return appDetails
}

func getApps(oldApps []model.App, items []dto.AppDefine, isLocal bool) map[string]model.App {
	apps := make(map[string]model.App, len(oldApps))
	for _, old := range oldApps {
		old.Status = constant.AppTakeDown
		apps[old.Key] = old
	}
	for _, item := range items {
		key := item.Key
		if isLocal {
			key = "local" + key
		}
		app, ok := apps[key]
		if !ok {
			app = model.App{}
		}
		if isLocal {
			app.Resource = constant.AppResourceLocal
		} else {
			app.Resource = constant.AppResourceRemote
		}
		app.Name = item.Name
		app.Limit = item.Limit
		app.Key = key
		app.ShortDescZh = item.ShortDescZh
		app.ShortDescEn = item.ShortDescEn
		app.Website = item.Website
		app.Document = item.Document
		app.Github = item.Github
		app.Type = item.Type
		app.CrossVersionUpdate = item.CrossVersionUpdate
		app.Required = item.GetRequired()
		app.Status = constant.AppNormal
		apps[key] = app
	}
	return apps
}

func handleErr(install model.AppInstall, err error, out string) error {
	reErr := err
	install.Message = err.Error()
	if out != "" {
		install.Message = out
		reErr = errors.New(out)
		install.Status = constant.Error
	}
	_ = appInstallRepo.Save(context.Background(), &install)
	return reErr
}

func getAppFromRepo(downloadPath, version string) error {
	downloadUrl := downloadPath
	appDir := constant.AppResourceDir

	global.LOG.Infof("download file from %s", downloadUrl)
	fileOp := files.NewFileOp()
	if _, err := fileOp.CopyAndBackup(appDir); err != nil {
		return err
	}
	packagePath := path.Join(constant.ResourceDir, path.Base(downloadUrl))
	if err := fileOp.DownloadFile(downloadUrl, packagePath); err != nil {
		return err
	}
	if err := fileOp.Decompress(packagePath, constant.ResourceDir, files.TarGz); err != nil {
		return err
	}
	_ = NewISettingService().Update("AppStoreVersion", version)
	defer func() {
		_ = fileOp.DeleteFile(packagePath)
	}()
	return nil
}

func downloadAppFromCustomRepo() error {
	var (
		customRepo  = global.CONF.System.CustomRepo
		versionUrl  = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", customRepo)
		localAppDir = constant.LocalAppResourceDir
	)
	if customRepo == "" {
		return nil
	}
	resp, err := http.Get(versionUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	type versionInfo struct {
		TagName string `json:"tag_name"`
	}
	var version versionInfo
	err = json.Unmarshal(content, &version)
	if err != nil {
		return err
	}
	downloadUrl := fmt.Sprintf("https://github.com/%s/releases/download/%s/localApps.tar.gz", customRepo, version.TagName)
	global.LOG.Infof("download file from %s", downloadUrl)
	fileOp := files.NewFileOp()
	// 备份 resource/localApps 目录到 resource/localApps_bak/localApps
	if _, err = fileOp.CopyAndBackup(localAppDir); err != nil {
		return err
	}
	// 下载 xxx/localApps.tar.gz 到 resource/localApps.tar.gz
	packagePath := path.Join(constant.ResourceDir, path.Base(downloadUrl))
	if err = fileOp.DownloadFile(downloadUrl, packagePath); err != nil {
		return err
	}
	// resource/localApps.tar.gz 为 resource/localApps 目录，且包内根目录有 list.json 和 全部本地应用
	if err = fileOp.Decompress(packagePath, constant.LocalAppResourceDir, files.TarGz); err != nil {
		return err
	}
	defer func() {
		_ = fileOp.DeleteFile(packagePath)
	}()
	return nil
}

func handleInstalled(appInstallList []model.AppInstall, updated bool) ([]response.AppInstalledDTO, error) {
	var res []response.AppInstalledDTO
	for _, installed := range appInstallList {
		if updated && installed.App.Type == "php" {
			continue
		}
		installDTO := response.AppInstalledDTO{
			AppInstall: installed,
		}
		app, err := appRepo.GetFirst(commonRepo.WithByID(installed.AppId))
		if err != nil {
			return nil, err
		}
		details, err := appDetailRepo.GetBy(appDetailRepo.WithAppId(app.ID))
		if err != nil {
			return nil, err
		}
		var versions []string
		for _, detail := range details {
			versions = append(versions, detail.Version)
		}
		versions = common.GetSortedVersions(versions)
		lastVersion := versions[0]
		if common.IsCrossVersion(installed.Version, lastVersion) {
			installDTO.CanUpdate = app.CrossVersionUpdate
		} else {
			installDTO.CanUpdate = common.CompareVersion(lastVersion, installed.Version)
		}
		if updated {
			if installDTO.CanUpdate {
				res = append(res, installDTO)
			}
		} else {
			res = append(res, installDTO)
		}
	}
	return res, nil
}

func getAppInstallByKey(key string) (model.AppInstall, error) {
	app, err := appRepo.GetFirst(appRepo.WithKey(key))
	if err != nil {
		return model.AppInstall{}, err
	}
	appInstall, err := appInstallRepo.GetFirst(appInstallRepo.WithAppId(app.ID))
	if err != nil {
		return model.AppInstall{}, err
	}
	return appInstall, nil
}

func updateToolApp(installed *model.AppInstall) {
	tooKey, ok := dto.AppToolMap[installed.App.Key]
	if !ok {
		return
	}
	toolInstall, _ := getAppInstallByKey(tooKey)
	if reflect.DeepEqual(toolInstall, model.AppInstall{}) {
		return
	}
	paramMap := make(map[string]string)
	_ = json.Unmarshal([]byte(installed.Param), &paramMap)
	envMap := make(map[string]interface{})
	_ = json.Unmarshal([]byte(toolInstall.Env), &envMap)
	if password, ok := paramMap["PANEL_DB_ROOT_PASSWORD"]; ok {
		envMap["PANEL_DB_ROOT_PASSWORD"] = password
	}
	if _, ok := envMap["PANEL_REDIS_HOST"]; ok {
		envMap["PANEL_REDIS_HOST"] = installed.ServiceName
	}
	if _, ok := envMap["PANEL_DB_HOST"]; ok {
		envMap["PANEL_DB_HOST"] = installed.ServiceName
	}

	envPath := path.Join(toolInstall.GetPath(), ".env")
	contentByte, err := json.Marshal(envMap)
	if err != nil {
		global.LOG.Errorf("update tool app [%s] error : %s", toolInstall.Name, err.Error())
		return
	}
	envFileMap := make(map[string]string)
	handleMap(envMap, envFileMap)
	if err = env.Write(envFileMap, envPath); err != nil {
		global.LOG.Errorf("update tool app [%s] error : %s", toolInstall.Name, err.Error())
		return
	}
	toolInstall.Env = string(contentByte)
	if err := appInstallRepo.Save(context.Background(), &toolInstall); err != nil {
		global.LOG.Errorf("update tool app [%s] error : %s", toolInstall.Name, err.Error())
		return
	}
	if out, err := compose.Down(toolInstall.GetComposePath()); err != nil {
		global.LOG.Errorf("update tool app [%s] error : %s", toolInstall.Name, out)
		return
	}
	if out, err := compose.Up(toolInstall.GetComposePath()); err != nil {
		global.LOG.Errorf("update tool app [%s] error : %s", toolInstall.Name, out)
		return
	}
}

package v1

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"1Panel/backend/app/api/v1/helper"
	"1Panel/backend/constant"
	"1Panel/backend/global"
	"1Panel/backend/utils/cmd"
	"1Panel/backend/utils/copier"
	"1Panel/backend/utils/ssh"
	"1Panel/backend/utils/terminal"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func (b *BaseApi) WsSsh(c *gin.Context) {
	id, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	cols, err := strconv.Atoi(c.DefaultQuery("cols", "80"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	rows, err := strconv.Atoi(c.DefaultQuery("rows", "40"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	host, err := hostService.GetHostInfo(uint(id))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	var connInfo ssh.ConnInfo
	_ = copier.Copy(&connInfo, &host)
	connInfo.PrivateKey = []byte(host.PrivateKey)
	if len(host.PassPhrase) != 0 {
		connInfo.PassPhrase = []byte(host.PassPhrase)
	}

	wsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		global.LOG.Errorf("gin context http handler failed, err: %v", err)
		return
	}
	defer wsConn.Close()

	client, err := connInfo.NewClient()
	if wshandleError(wsConn, errors.WithMessage(err, "failed to set up the connection. Please check the host information")) {
		return
	}
	defer client.Close()
	ssConn, err := connInfo.NewSshConn(cols, rows)
	if wshandleError(wsConn, err) {
		return
	}
	defer ssConn.Close()

	sws, err := terminal.NewLogicSshWsSession(cols, rows, true, connInfo.Client, wsConn)
	if wshandleError(wsConn, err) {
		return
	}
	defer sws.Close()

	quitChan := make(chan bool, 3)
	sws.Start(quitChan)
	go sws.Wait(quitChan)

	<-quitChan

	if wshandleError(wsConn, err) {
		return
	}
}

func (b *BaseApi) RedisWsSsh(c *gin.Context) {
	cols, err := strconv.Atoi(c.DefaultQuery("cols", "80"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	rows, err := strconv.Atoi(c.DefaultQuery("rows", "40"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	redisConf, err := redisService.LoadConf()
	if err != nil {
		global.LOG.Errorf("load redis container failed, err: %v", err)
		helper.ErrorWithDetail(c, constant.CodeErrInternalServer, constant.ErrTypeInternalServer, err)
		return
	}

	wsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		global.LOG.Errorf("gin context http handler failed, err: %v", err)
		return
	}
	defer wsConn.Close()
	commands := "redis-cli"
	if len(redisConf.Requirepass) != 0 {
		commands = fmt.Sprintf("redis-cli -a %s --no-auth-warning", redisConf.Requirepass)
	}
	slave, err := terminal.NewCommand(fmt.Sprintf("docker exec -it %s %s", redisConf.ContainerName, commands))
	if wshandleError(wsConn, err) {
		return
	}
	defer killBash(redisConf.ContainerName, commands)
	defer slave.Close()

	tty, err := terminal.NewLocalWsSession(cols, rows, wsConn, slave)
	if wshandleError(wsConn, err) {
		return
	}

	quitChan := make(chan bool, 3)
	tty.Start(quitChan)
	go slave.Wait(quitChan)

	<-quitChan

	global.LOG.Info("websocket finished")
	if wshandleError(wsConn, err) {
		return
	}
}

func (b *BaseApi) ContainerWsSsh(c *gin.Context) {
	containerID := c.Query("containerid")
	command := c.Query("command")
	user := c.Query("user")
	if len(command) == 0 || len(containerID) == 0 {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, errors.New("error param of command or containerID"))
		return
	}
	cols, err := strconv.Atoi(c.DefaultQuery("cols", "80"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}
	rows, err := strconv.Atoi(c.DefaultQuery("rows", "40"))
	if err != nil {
		helper.ErrorWithDetail(c, constant.CodeErrBadRequest, constant.ErrTypeInvalidParams, err)
		return
	}

	wsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		global.LOG.Errorf("gin context http handler failed, err: %v", err)
		return
	}
	defer wsConn.Close()

	cmds := fmt.Sprintf("docker exec %s %s", containerID, command)
	if len(user) != 0 {
		cmds = fmt.Sprintf("docker exec -u %s %s %s", user, containerID, command)
	}
	stdout, err := cmd.Exec(cmds)
	if wshandleError(wsConn, errors.WithMessage(err, stdout)) {
		return
	}

	commands := fmt.Sprintf("docker exec -it %s %s", containerID, command)
	if len(user) != 0 {
		commands = fmt.Sprintf("docker exec -it -u %s %s %s", user, containerID, command)
	}
	slave, err := terminal.NewCommand(commands)
	if wshandleError(wsConn, err) {
		return
	}
	defer killBash(containerID, command)
	defer slave.Close()

	tty, err := terminal.NewLocalWsSession(cols, rows, wsConn, slave)
	if wshandleError(wsConn, err) {
		return
	}

	quitChan := make(chan bool, 3)
	tty.Start(quitChan)
	go slave.Wait(quitChan)

	<-quitChan

	global.LOG.Info("websocket finished")
	if wshandleError(wsConn, err) {
		return
	}
}

func wshandleError(ws *websocket.Conn, err error) bool {
	if err != nil {
		global.LOG.Errorf("handler ws faled:, err: %v", err)
		dt := time.Now().Add(time.Second)
		if ctlerr := ws.WriteControl(websocket.CloseMessage, []byte(err.Error()), dt); ctlerr != nil {
			wsData, err := json.Marshal(terminal.WsMsg{
				Type: terminal.WsMsgCmd,
				Data: base64.StdEncoding.EncodeToString([]byte(err.Error())),
			})
			if err != nil {
				_ = ws.WriteMessage(websocket.TextMessage, []byte("{\"type\":\"cmd\",\"data\":\"failed to encoding to json\"}"))
			} else {
				_ = ws.WriteMessage(websocket.TextMessage, wsData)
			}
		}
		return true
	}
	return false
}

func killBash(containerID, comm string) {
	sudo := ""
	if cmd.HasNoPasswordSudo() {
		sudo = "sudo"
	}
	command := exec.Command("sh", "-c", fmt.Sprintf("%s kill -9 $(docker top %s -eo pid,command | grep '%s' | awk '{print $1}')", sudo, containerID, comm))
	_, _ = command.CombinedOutput()
}

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 10,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

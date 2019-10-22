package api

import (
	"github.com/idena-network/idena-test-go/process"
	"net/http"
	"strconv"
	"time"
)

type Api struct {
	process   *process.Process
	apiServer server
}

type server interface {
	Start(port int)
}

func NewApi(process *process.Process, serverPort int) *Api {
	api := &Api{
		process: process,
	}
	httpServer := NewHttpServer(api)
	httpServer.Start(serverPort)
	api.apiServer = httpServer
	return api
}

func (api *Api) createServer() server {
	return NewHttpServer(api)
}

type reqCtx struct {
	id        string
	timestamp time.Time
}

func (api *Api) getGodAddress(r *http.Request) (string, error) {
	return api.process.GetGodAddress()
}

func (api *Api) getBootNode(r *http.Request) (string, error) {
	return api.process.GetBootNode()
}

func (api *Api) getIpfsBootNode(r *http.Request) (string, error) {
	return api.process.GetIpfsBootNode()
}

func (api *Api) createInvite(r *http.Request) (string, error) {
	addr := r.FormValue("address")
	err := api.process.RequestInvite(addr)
	if err != nil {
		return "", err
	}
	return "OK", nil
}

func (api *Api) getCeremonyTime(r *http.Request) (string, error) {
	ceremonyTime, err := api.process.GetCeremonyTime()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(ceremonyTime, 10), nil
}

func (api *Api) getEpoch(r *http.Request) (string, error) {
	epoch, err := api.process.GetEpoch()
	if err != nil {
		return "", err
	}
	return strconv.Itoa(int(epoch)), nil
}

func (api *Api) sendFailNotification(r *http.Request) (string, error) {
	message := r.FormValue("message")
	api.process.SendFailNotification(message, r.RemoteAddr)
	return "OK", nil
}

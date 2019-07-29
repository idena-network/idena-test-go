package api

import (
	"errors"
	"github.com/idena-network/idena-test-go/process"
	"net/http"
	"strconv"
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
	addrs := r.Form["address"]
	if len(addrs) == 0 {
		return "", errors.New("parameter 'address' is absent")
	}
	addr := addrs[0]
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

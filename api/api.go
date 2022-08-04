package api

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/process"
	"github.com/idena-network/idena-test-go/scenario"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Api struct {
	process   *process.Process
	apiServer server
	poolsCtx  *poolsCtx
}

type server interface {
	Start(port int)
}

type poolsCtx struct {
	poolSizes     []int
	rate          float64
	poolAddresses []string
	mutex         sync.Mutex
}

func NewApi(process *process.Process, multiBotBools *scenario.MultiBotPools, serverPort int) *Api {
	api := &Api{
		process: process,
	}
	if multiBotBools != nil {
		api.poolsCtx = &poolsCtx{
			poolSizes: append([]int{}, multiBotBools.Sizes...),
			rate:      multiBotBools.BotDelegatorsRate,
		}
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

func (api *Api) getIpfsBootNode(r *http.Request) (string, error) {
	return api.process.GetIpfsBootNode()
}

func (api *Api) createInvites(r *http.Request) (string, error) {
	addresses := strings.Split(r.FormValue("addresses"), ",")
	err := api.process.RequestInvites(addresses)
	if err != nil {
		return "", err
	}
	resp := "OK"
	if api.poolsCtx == nil {
		return resp, nil
	}
	api.poolsCtx.mutex.Lock()
	defer api.poolsCtx.mutex.Unlock()
	if len(api.poolsCtx.poolAddresses) < len(api.poolsCtx.poolSizes) {
		api.poolsCtx.poolAddresses = append(api.poolsCtx.poolAddresses, addresses[0])
		log.Info(fmt.Sprintf("Set pool owner %v, size %v", addresses[0], api.poolsCtx.poolSizes[len(api.poolsCtx.poolAddresses)-1]))
	} else {
		var delegatees []string
		n := int(float64(len(addresses)) * api.poolsCtx.rate)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			for idx, poolSize := range api.poolsCtx.poolSizes {
				if poolSize > 0 {
					api.poolsCtx.poolSizes[idx]--
					delegatee := api.poolsCtx.poolAddresses[idx]
					delegator := addresses[i]
					delegatees = append(delegatees, delegatee)
					log.Info(fmt.Sprintf("Set delegatee %v for %v", delegatee, delegator))
					break
				}
			}
		}
		if len(delegatees) > 0 {
			resp = strings.Join(delegatees, ",")
		}
	}
	return resp, nil
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

func (api *Api) sendWarnNotification(r *http.Request) (string, error) {
	message := r.FormValue("message")
	api.process.SendWarnNotification(message, r.RemoteAddr)
	return "OK", nil
}

func (api *Api) identities(r *http.Request) (string, error) {
	exportKey := func(privateKey *ecdsa.PrivateKey, password string) string {
		if privateKey == nil {
			return ""
		}
		key := crypto.FromECDSA(privateKey)
		encrypted, err := crypto.Encrypt(key, password)
		if err != nil {
			return ""
		}
		return hex.EncodeToString(encrypted)
	}
	var res string
	userDetails := api.process.UserDetails()
	const pass = "123"
	for i, u := range userDetails {
		encryptedKey := exportKey(u.PrivateKey, pass)
		res += fmt.Sprintf("%v. %v\r\n", i+1, encryptedKey)
	}
	return res, nil
}

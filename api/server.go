package api

import (
	"fmt"
	"idena-test-go/log"
	"net/http"
)

type httpServer struct {
	handlers map[string]handler
}

func NewHttpServer(api *Api) *httpServer {
	handlers := make(map[string]handler)
	handlers["/api/GetGodAddress"] = api.getGodAddress
	handlers["/api/GetBootNode"] = api.getBootNode
	handlers["/api/GetIpfsBootNode"] = api.getIpfsBootNode
	handlers["/api/CreateInvite"] = api.createInvite
	handlers["/api/GetCeremonyTime"] = api.getCeremonyTime
	return &httpServer{handlers: handlers}
}

type handler func(r *http.Request) (string, error)

func (server *httpServer) Start(port int) {
	http.HandleFunc("/", server.handleRequest)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		panic(err)
	}
}

func (server *httpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Error(fmt.Sprintf("Unable to parse API request: %v", err))
		w.WriteHeader(500)
		return
	}
	path := r.URL.Path
	if path == "/favicon.ico" {
		return
	}
	handler := server.handlers[path]
	if handler == nil {
		log.Error(fmt.Sprintf("Theres is no API handler for path %v", path))
		w.WriteHeader(500)
		return
	}
	resp, err := handler(r)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to handle API request: %v", err))
		w.WriteHeader(500)
		return
	}
	if len(resp) == 0 {
		log.Error("Empty API response")
		w.WriteHeader(500)
		return
	}
	_, err = fmt.Fprintf(w, resp)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to write API response: %v", err))
		return
	}
}

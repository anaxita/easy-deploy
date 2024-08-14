package api

import (
	"net/http"
	"strconv"
	"time"

	"gitlab.com/anaxita-server/easy-deploy/internal/service"
)

func NewServer(port int, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadTimeout:       time.Second * 5,
		WriteTimeout:      time.Second * 5,
		ReadHeaderTimeout: time.Second * 5,
	}
}

func NewMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()

	return mux
}

type Handler struct {
	deploy *service.Deploy
}

func NewHandler(deploy *service.Deploy) *Handler {
	return &Handler{
		deploy: deploy,
	}
}

package api

import (
	"net/http"

	"kubegems.io/library/rest/response"
)

type Plugin interface {
	Install(m *API) error
	OnRoute(route *Route) error
}

type NoopPlugin struct{}

func (n NoopPlugin) Install(m *API) error {
	return nil
}

func (n NoopPlugin) OnRoute(route *Route) error {
	return nil
}

type VersionPlugin struct {
	NoopPlugin
	Version any
}

func (v VersionPlugin) Install(m *API) error {
	m.Route(GET("/version").Doc("version").To(func(resp http.ResponseWriter, req *http.Request) {
		response.Raw(resp, http.StatusOK, v.Version, nil)
	}))
	return nil
}

type HealthCheckPlugin struct {
	NoopPlugin
	CheckFun func() error
}

func (h HealthCheckPlugin) Install(m *API) error {
	m.Route(GET("/healthz").Doc("health check").To(func(resp http.ResponseWriter, req *http.Request) {
		if h.CheckFun != nil {
			if err := h.CheckFun(); err != nil {
				response.ServerError(resp, err)
				return
			}
		}
		response.Raw(resp, http.StatusOK, "ok", nil)
	}))
	return nil
}

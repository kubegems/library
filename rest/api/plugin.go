package api

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"kubegems.io/library/rest/request"
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

type OpenTelemetryPlugin struct {
	TraceProvider trace.TracerProvider
}

func (o OpenTelemetryPlugin) Install(m *API) error {
	return nil
}

func (o OpenTelemetryPlugin) OnRoute(route *Route) error {
	route.Handler = otelhttp.WithRouteTag(route.Path, route.Handler)
	// inject filter
	midware := otelhttp.NewMiddleware(route.Path, otelhttp.WithTracerProvider(o.TraceProvider))
	filter := FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		nn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			vars := request.PathVars(r)
			vals := make([]string, 0, len(vars))
			for _, v := range vars {
				vals = append(vals, v.Key+"="+v.Value)
			}
			sp := trace.SpanFromContext(r.Context())
			sp.SetAttributes(
				attribute.StringSlice("pathvars", vals),
			)
		})
		midware(nn).ServeHTTP(w, r)
	})
	// prepend
	route.Filters = append([]Filter{filter}, route.Filters...)
	return nil
}

// Copyright 2023 The Kubegems Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"net/http"

	"github.com/go-openapi/spec"
	"kubegems.io/library/rest/listen"
	"kubegems.io/library/rest/matcher"
	"kubegems.io/library/rest/mux"
	"kubegems.io/library/rest/openapi"
	"kubegems.io/library/rest/response"
)

type API struct {
	tls     tlsfiles
	filters PatternFilters
	swagger *spec.Swagger
	builder *openapi.Builder
	mux     *mux.MethodServeMux
}

type tlsfiles struct {
	crt string
	key string
}

func NewAPI() *API {
	swagger := &spec.Swagger{SwaggerProps: spec.SwaggerProps{
		Swagger:     "2.0",
		Definitions: map[string]spec.Schema{},
	}}
	return &API{
		swagger: swagger,
		mux:     mux.NewMethodServeMux(),
		filters: PatternFilters{},
		builder: openapi.NewBuilder(openapi.InterfaceBuildOptionDefault, swagger.Definitions),
	}
}

func (m *API) Filter(pattern string, filters ...Filter) *API {
	sec, err := matcher.Compile(pattern)
	if err != nil {
		panic(err)
	}
	m.filters = append(m.filters, PatternFilter{Pattern: sec, Filters: filters})
	return m
}

func (m *API) Route(route *Route) *API {
	m.mux.HandleFunc(route.Method, route.Path, route.Func)
	AddSwaggerOperation(m.swagger, route, m.builder)
	return m
}

func (m *API) Register(prefix string, modules ...Module) *API {
	rg := NewGroup(prefix)
	for _, module := range modules {
		module.RegisterRoute(rg)
	}
	for _, methods := range rg.BuildRoutes() {
		for _, route := range methods {
			m.Route(route)
		}
	}
	return m
}

func (m *API) BuildHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		m.filters.Process(resp, req, m.mux)
	})
}

func (m *API) TLS(cert, key string) *API {
	m.tls = tlsfiles{crt: cert, key: key}
	return m
}

func (m *API) Serve(ctx context.Context, listenaddr string) error {
	return listen.ServeContext(ctx, listenaddr, m.BuildHandler(), m.tls.crt, m.tls.key)
}

func (m *API) HealthCheck(checkfun func() error) *API {
	return m.Route(GET("/healthz").Doc("health check").To(func(resp http.ResponseWriter, req *http.Request) {
		if checkfun != nil {
			if err := checkfun(); err != nil {
				response.ServerError(resp, err)
				return
			}
		}
		response.Raw(resp, http.StatusOK, "ok", nil)
	}))
}

func (m *API) APIDoc(completer func(swagger *spec.Swagger)) *API {
	if completer != nil {
		completer(m.swagger)
	}
	// api doc
	specPath := "/docs/api.json"
	m.Route(GET(specPath).Doc("swagger api doc").To(func(resp http.ResponseWriter, req *http.Request) {
		response.Raw(resp, http.StatusOK, m.swagger, nil)
	}))
	// UI
	redocui, swaggerui := NewRedocUI(specPath), NewSwaggerUI(specPath)
	m.Route(GET("/docs").
		Doc("swagger api html").
		Parameters(QueryParameter("provider", "UI provider").In("swagger", "redoc")).
		To(func(resp http.ResponseWriter, req *http.Request) {
			switch req.URL.Query().Get("provider") {
			case "swagger", "":
				renderHTML(resp, swaggerui)
			case "redoc":
				renderHTML(resp, redocui)
			}
		}),
	)
	return m
}

func (m *API) Version(data any) *API {
	return m.Route(GET("/version").Doc("version").To(func(resp http.ResponseWriter, req *http.Request) {
		response.Raw(resp, http.StatusOK, data, nil)
	}))
}

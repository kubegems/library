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
	"net/http"
	"path"

	"github.com/go-openapi/spec"
	"kubegems.io/library/log"
	"kubegems.io/library/rest/filters"
	"kubegems.io/library/rest/mux"
	"kubegems.io/library/rest/openapi"
	libreflector "kubegems.io/library/rest/reflector"
	"kubegems.io/library/rest/response"
)

type API struct {
	options  *APIOptions
	swagger  *spec.Swagger
	mactcher *mux.MethodServeMux
}

type APIOptions struct {
	Prefix           string
	HealthCheck      func() error
	SwaggerCompleter func(swagger *spec.Swagger)
	Filters          filters.Filters
}

func NewAPI(options APIOptions) *API {
	swagger := &spec.Swagger{SwaggerProps: spec.SwaggerProps{Swagger: "2.0"}}
	if options.SwaggerCompleter == nil {
		options.SwaggerCompleter = KubegemsSwaggerCompleter
	}
	options.SwaggerCompleter(swagger)
	mux := mux.NewMethodServeMux()
	mux.HandleFunc("GET", path.Join(options.Prefix, "/docs.json"), Swagger(swagger))
	mux.HandleFunc("GET", "/healthz", Healthz(options.HealthCheck))
	return &API{swagger: swagger, options: &options, mactcher: mux}
}

func (m *API) RegisterController(parents []string, controller any) error {
	return libreflector.Register(m.mactcher, m.swagger, m.options.Prefix, parents, controller)
}

func (m *API) RegisterModules(modules ...RestModule) *API {
	rg := NewGroup(m.options.Prefix)
	for _, module := range modules {
		module.RegisterRoute(rg)
	}
	tree := Tree{
		Group:           rg,
		RouteUpdateFunc: ListWrrapperFunc,
	}
	tree.AddToMux(m.mactcher)
	tree.AddToSwagger(m.swagger, openapi.NewBuilder(openapi.InterfaceBuildOptionDefault))
	return m
}

func (m *API) Handle(method string, pattern string, handler http.Handler) {
	m.mactcher.Handle(method, pattern, handler)
}

func (m *API) BuildHandler() http.Handler {
	handler := http.Handler(m.mactcher)
	handler = LogFilter(log.Logger, handler) // log
	return handler
}

func Healthz(checkfun func() error) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		if checkfun != nil {
			if err := checkfun(); err != nil {
				response.ServerError(resp, err)
				return
			}
		}
		response.OK(resp, "ok")
	}
}

func Swagger(swagger *spec.Swagger) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		response.Raw(resp, http.StatusOK, swagger, nil)
	}
}

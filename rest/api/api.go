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

	"kubegems.io/library/rest/listen"
)

type API struct {
	tls     tlsfiles
	filter  FilterHolder
	plugins []Plugin
	mux     Router
}

type tlsfiles struct {
	crt string
	key string
}

func NewAPI() *API {
	return &API{
		mux:    NewMux(),
		filter: NewFilters(),
	}
}

type Predicate func(r *http.Request) bool

func (m *API) Filter(pattern string, filters ...Filter) *API {
	if err := m.filter.Register(pattern, filters...); err != nil {
		panic(err)
	}
	return m
}

func (m *API) Route(route Route) *API {
	if err := m.mux.HandleRoute(&route); err != nil {
		panic(err)
	}
	for _, plugin := range m.plugins {
		if err := plugin.OnRoute(&route); err != nil {
			panic(err)
		}
	}
	return m
}

func (m *API) NotFound(handler http.Handler) *API {
	m.mux.SetNotFound(handler)
	return m
}

func (m *API) Register(prefix string, modules ...Module) *API {
	rg := NewGroup(prefix)
	for _, module := range modules {
		rg = rg.SubGroup(module.Routes()...)
	}
	for _, methods := range rg.Build() {
		for _, route := range methods {
			m.Route(route)
		}
	}
	return m
}

func (m *API) Build() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		m.filter.Process(resp, req, m.mux)
	})
}

func (m *API) TLS(cert, key string) *API {
	m.tls = tlsfiles{crt: cert, key: key}
	return m
}

func (m *API) Serve(ctx context.Context, listenaddr string) error {
	return listen.ServeContext(ctx, listenaddr, m.Build(), m.tls.crt, m.tls.key)
}

func (m *API) Plugin(plugin ...Plugin) *API {
	for _, p := range plugin {
		if err := p.Install(m); err != nil {
			panic(err)
		}
		m.plugins = append(m.plugins, p)
	}
	return m
}

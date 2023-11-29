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
	plugins []Plugin
	mux     Router
}

type tlsfiles struct {
	crt string
	key string
}

func NewAPI() *API {
	return &API{
		mux: NewMux(),
	}
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

func (m *API) Group(groups ...Group) *API {
	for _, group := range groups {
		for _, routes := range group.Build() {
			for _, route := range routes {
				m.Route(route)
			}
		}
	}
	return m
}

func (m *API) PrefixGroup(prefix string, groups ...Group) *API {
	for _, path := range NewGroup(prefix).SubGroup(groups...).Build() {
		for _, route := range path {
			m.Route(route)
		}
	}
	return m
}

func (m *API) Build() http.Handler {
	return m.mux
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

// Copyright 2022 The kubegems.io Authors
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

package mux

import (
	"context"
	"fmt"
	"net/http"

	"kubegems.io/library/rest/matcher"
)

var (
	NotFoundHandler         = http.NotFound
	MethodNotAllowedHandler = MethodNotAllowed
)

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
}

// using get pathvars from context.Context returns map[string]string{}
var contextKeyPathVars = &struct{ name string }{name: "path variables"}

func PathVar(r *http.Request, key string) string {
	if vars := PathVars(r); vars != nil {
		return vars[key]
	}
	return ""
}

func PathVars(r *http.Request) map[string]string {
	if vars, ok := r.Context().Value(contextKeyPathVars).(map[string]string); ok {
		return vars
	}
	return nil
}

// ServeMux is a http.ServeMux like library,but support path variable
// for method match, see MethodServerMux
type ServeMux struct {
	matcher matcher.PatternMatcher[http.Handler]
}

func NewServeMux() *ServeMux {
	return &ServeMux{matcher: matcher.NewMatcher[http.Handler]()}
}

func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if matched, val, vars := mux.matcher.Match(r.URL.Path); matched {
		r = r.WithContext(context.WithValue(r.Context(), contextKeyPathVars, vars))
		val.ServeHTTP(w, r)
		return
	}
	NotFoundHandler(w, r)
}

func (mux *ServeMux) Handle(pattern string, handler http.Handler) {
	_, _ = mux.matcher.Register(pattern, handler)
}

func (mux *ServeMux) HandlerFunc(pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	mux.Handle(pattern, http.HandlerFunc(handler))
}

type MethodHandler map[string]http.Handler

func (m MethodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if allmatchhandler, ok := m[""]; ok {
		allmatchhandler.ServeHTTP(w, r)
		return
	}
	if h, ok := m[r.Method]; ok {
		h.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

type MethodServeMux struct {
	pathmap  map[string]*MethodHandler
	mactcher matcher.PatternMatcher[*MethodHandler]
}

func NewMethodServeMux() *MethodServeMux {
	return &MethodServeMux{
		pathmap:  make(map[string]*MethodHandler),
		mactcher: matcher.NewMatcher[*MethodHandler](),
	}
}

func (mux *MethodServeMux) Handle(method, pattern string, handler http.Handler) {
	mh, ok := mux.pathmap[pattern]
	if ok {
		if _, ok := (*mh)[method]; ok {
			panic(fmt.Sprintf("mux: multiple registrations for %s %s", method, pattern))
		} else {
			(*mh)[method] = handler
			return
		}
	}
	mh = &MethodHandler{method: handler}
	_, _ = mux.mactcher.Register(pattern, mh)
	mux.pathmap[pattern] = mh
}

func (mux *MethodServeMux) HandleFunc(method, pattern string, handler func(w http.ResponseWriter, r *http.Request)) {
	mux.Handle(method, pattern, http.HandlerFunc(handler))
}

func (mux *MethodServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if matched, val, vars := mux.mactcher.Match(r.URL.Path); matched {
		val.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKeyPathVars, vars)))
		return
	}
	NotFoundHandler(w, r)
}

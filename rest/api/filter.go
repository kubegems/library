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
	"path"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-logr/logr"
	"kubegems.io/library/rest/matcher"
	"kubegems.io/library/rest/response"
)

type FilterHolder interface {
	Filter
	Register(pattern string, filters ...Filter) error
}

type Filter interface {
	Process(w http.ResponseWriter, r *http.Request, next http.Handler)
}

type FilterFunc func(w http.ResponseWriter, r *http.Request, next http.Handler)

func (f FilterFunc) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	f(w, r, next)
}

type PredicatedFilter struct {
	Predicate func(r *http.Request) bool
	Filter    Filter
}

func (f PredicatedFilter) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if f.Predicate != nil || !f.Predicate(r) {
		next.ServeHTTP(w, r)
	} else {
		f.Filter.Process(w, r, next)
	}
}

type SimpleFilters struct {
	Node matcher.Node[Filters]
}

var _ FilterHolder = (*SimpleFilters)(nil)

func NewFilters() *SimpleFilters {
	return &SimpleFilters{}
}

func (t *SimpleFilters) Register(pattern string, filters ...Filter) error {
	_, node, err := t.Node.Get(pattern)
	if err != nil {
		return err
	}
	node.Value = append(node.Value, filters...)
	return nil
}

func (t *SimpleFilters) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	node, _ := t.Node.Match(r.URL.Path, nil)
	if node == nil {
		next.ServeHTTP(w, r)
		return
	}
	node.Value.Process(w, r, next)
}

type Filters []Filter

func (fs Filters) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if len(fs) == 0 {
		next.ServeHTTP(w, r)
		return
	}
	fs[0].Process(w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs[1:].Process(w, r, next)
	}))
}

func CORSFilter() Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		orgin := r.Header.Get("Origin")
		if orgin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", orgin)
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		next.ServeHTTP(w, r)
	})
}

func LoggingFilter(log logr.Logger) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		start := time.Now()
		ww := &StatusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(ww, r)
		duration := time.Since(start)
		log.Info(r.RequestURI,
			"method", r.Method,
			"remote", r.RemoteAddr,
			"code", ww.StatusCode,
			"duration", duration.String())
	})
}

type StatusResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func (w *StatusResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

type OIDCClientOptions struct {
	Issuer         string
	Audience       string
	NoAuthPatterns []string // white list pathes, match by path.Match
}

func OIDCFilter(ctx context.Context, opts *OIDCClientOptions) Filter {
	// no oidc
	if opts.Issuer == "" {
		return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
			next.ServeHTTP(w, r)
		})
	}
	ctx = oidc.InsecureIssuerURLContext(ctx, opts.Issuer)
	provider, err := oidc.NewProvider(ctx, opts.Issuer)
	if err != nil {
		panic(err)
	}
	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: opts.Audience == "",
		SkipIssuerCheck:   true,
	})
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		for _, pattern := range opts.NoAuthPatterns {
			if match, _ := path.Match(pattern, r.URL.Path); match {
				next.ServeHTTP(w, r)
				return
			}
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			queries := r.URL.Query()
			for _, k := range []string{"token", "access_token"} {
				if token = queries.Get(k); token != "" {
					break
				}
			}
		}
		if len(token) == 0 {
			response.Unauthorized(w, "missing access token")
			return
		}
		idtoken, err := verifier.Verify(r.Context(), token)
		if err != nil {
			// ResponseError(w, apierr.NewUnauthorizedError("invalid access token"))
			response.Unauthorized(w, "invalid access token")
			return
		}
		r = r.WithContext(NewOIDCContext(r.Context(), OIDCInfo{
			Username: idtoken.Subject,
			Token:    token,
		}))
		next.ServeHTTP(w, r)
	})
}

type contextOIDCKey struct{}

type OIDCInfo struct {
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
}

func NewOIDCContext(ctx context.Context, val OIDCInfo) context.Context {
	return context.WithValue(ctx, contextOIDCKey{}, val)
}

func OIDCFromContext(ctx context.Context) OIDCInfo {
	if val, ok := ctx.Value(contextOIDCKey{}).(OIDCInfo); ok {
		return val
	}
	return OIDCInfo{}
}

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

type Filter func(w http.ResponseWriter, r *http.Request, next http.Handler)

type Filters []Filter

func (fs Filters) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if len(fs) == 0 {
		next.ServeHTTP(w, r)
		return
	}
	fs[0](w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs[1:].Process(w, r, next)
	}))
}

type PatternFilter struct {
	Pattern matcher.Section
	Filters Filters
}

type PatternFilters []PatternFilter

func (p PatternFilters) Process(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if len(p) == 0 {
		next.ServeHTTP(w, r)
		return
	}
	if match, _, _ := p[0].Pattern.Match(matcher.PathTokens(r.URL.Path)); !match {
		p[1:].Process(w, r, next)
		return
	}
	p[0].Filters.Process(w, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p[1:].Process(w, r, next)
	}))
}

func CORSFilter() Filter {
	return func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		orgin := r.Header.Get("Origin")
		if orgin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", orgin)
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		next.ServeHTTP(w, r)
	}
}

func LoggingFilter(log logr.Logger) Filter {
	return func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		start := time.Now()
		ww := &StatusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(ww, r)
		duration := time.Since(start)
		log.Info(r.RequestURI,
			"method", r.Method,
			"remote", r.RemoteAddr,
			"code", ww.StatusCode,
			"duration", duration.String())
	}
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

func OIDCAuthFilter(ctx context.Context, opts *OIDCClientOptions) Filter {
	// no oidc
	if opts.Issuer == "" {
		return func(w http.ResponseWriter, r *http.Request, next http.Handler) {
			next.ServeHTTP(w, r)
		}
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
	return func(w http.ResponseWriter, r *http.Request, next http.Handler) {
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
		r = r.WithContext(NewUsernameContext(r.Context(), idtoken.Subject))
		next.ServeHTTP(w, r)
	}
}

type contextUsernameKey struct{}

func UsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(contextUsernameKey{}).(string); ok {
		return username
	}
	return ""
}

func NewUsernameContext(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, contextUsernameKey{}, username)
}

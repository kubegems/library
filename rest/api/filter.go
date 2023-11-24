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
	"compress/flate"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"kubegems.io/library/rest/matcher"
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
	node, vars := t.Node.Match(r.URL.Path, nil)
	if len(vars) > 0 {
		varsmap := make(map[string]string, len(vars))
		for _, v := range vars {
			varsmap[v.Name] = v.Value
		}
		r = r.WithContext(context.WithValue(r.Context(), httpVarsContextKey{}, varsmap))
	}
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

func NoopFilter() Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		next.ServeHTTP(w, r)
	})
}

func LoggingFilter(log logr.Logger) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Info(r.RequestURI, "method", r.Method, "remote", r.RemoteAddr, "duration", time.Since(start).String())
	})
}

// NewCompressionFilter returns a filter that compresses the response body
func NewCompressionFilter() Filter {
	gzipPool := &sync.Pool{
		New: func() interface{} {
			gw, err := gzip.NewWriterLevel(nil, gzip.BestSpeed)
			if err != nil {
				panic(err)
			}
			return gw
		},
	}
	flatePool := &sync.Pool{
		New: func() interface{} {
			fw, err := flate.NewWriter(nil, flate.BestSpeed)
			if err != nil {
				panic(err)
			}
			return fw
		},
	}
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		var wrappedWriter io.Writer
		encoding := r.Header.Get("Accept-Encoding")
		accept := ""
		for len(encoding) > 0 {
			var token string
			if next := strings.Index(encoding, ","); next != -1 {
				token = encoding[:next]
				encoding = encoding[next+1:]
			} else {
				token = encoding
				encoding = ""
			}
			if strings.TrimSpace(token) != "" {
				accept = token
				break
			}
		}
		switch accept {
		case "gzip":
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Add("Vary", "Accept-Encoding")

			gw := gzipPool.Get().(*gzip.Writer)
			gw.Reset(w)
			defer gzipPool.Put(gw)

			wrappedWriter = gw
		case "deflate":
			w.Header().Set("Content-Encoding", "deflate")
			w.Header().Add("Vary", "Accept-Encoding")
			fw := flatePool.Get().(*flate.Writer)
			fw.Reset(w)
			defer flatePool.Put(fw)

			wrappedWriter = fw
		}
		if wrappedWriter != nil {
			w = &CompresseWriter{ResponseWriter: w, w: wrappedWriter}
		}
		next.ServeHTTP(w, r)
	})
}

type CompresseWriter struct {
	http.ResponseWriter
	w io.Writer
}

func (cw *CompresseWriter) Flush() {
	if flusher, ok := cw.w.(http.Flusher); ok {
		flusher.Flush()
	}
	if flusher, ok := cw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func NewConditionFilter(cond func(r *http.Request) bool, filter Filter) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		if cond(r) {
			filter.Process(w, r, next)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

package api

import (
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

func LogFilter(log logr.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &StatusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(ww, r)
		duration := time.Since(start)
		log.Info(r.RequestURI, "method", r.Method, "code", ww.StatusCode, "remote", r.RemoteAddr, "duration", duration.String())
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

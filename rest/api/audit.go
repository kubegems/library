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
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

type Auditor interface {
	OnRequest(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, *AuditLog)
	OnResponse(w http.ResponseWriter, r *http.Request, auditlog *AuditLog)
}

type AuditSink interface {
	Save(log *AuditLog) error
}

func NewAuditFilter(auditor Auditor, sink AuditSink) Filter {
	return FilterFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		ww, auditlog := auditor.OnRequest(w, r) // audit request
		if auditlog == nil {                    // no audit log, skip
			next.ServeHTTP(ww, r)
			return
		}
		if ww == nil {
			ww = w
		}
		next.ServeHTTP(ww, r)               // process request and response
		auditor.OnResponse(ww, r, auditlog) // audit response
		_ = sink.Save(auditlog)             // save audit log
	})
}

type LoggerAuditSink struct {
	Sink   AuditSink
	Logger logr.Logger
}

func (l *LoggerAuditSink) Save(log *AuditLog) error {
	// trim query params
	reqpath := log.Request.URL
	if idx := strings.Index(reqpath, "?"); idx > 0 {
		reqpath = reqpath[:idx]
	}
	l.Logger.Info(
		reqpath,
		"method", log.Request.Method,
		"remote", log.Request.ClientIP,
		"code", log.Response.StatusCode,
		"duration", log.EndTime.Sub(log.StartTime).String(),
		"resource", log.Resource,
		"name", log.ResourceName,
	)
	if l.Sink != nil {
		return l.Sink.Save(log)
	}
	return nil
}

const DefaultAuditLogCacheSize = 256

func NewCachedAuditSink(ctx context.Context, sink AuditSink, maxCacheSize int) AuditSink {
	if maxCacheSize <= 0 {
		maxCacheSize = DefaultAuditLogCacheSize
	}
	logger := logr.FromContextOrDiscard(ctx).WithName("cached-audit-sink")
	cachesink := &CachedAuditSink{
		sink:   sink,
		cache:  make(chan *AuditLog, maxCacheSize),
		logger: logger,
	}
	go func() {
		for {
			select {
			case auditlog := <-cachesink.cache:
				if err := sink.Save(auditlog); err != nil {
					logger.Error(err, "save audit log")
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return cachesink
}

type CachedAuditSink struct {
	sink   AuditSink
	logger logr.Logger
	cache  chan *AuditLog
}

func (c *CachedAuditSink) Save(log *AuditLog) error {
	select {
	case c.cache <- log:
	default:
		c.logger.Error(fmt.Errorf("cache channel full,drop audit log"), "save audit log")
		return fmt.Errorf("cache is full")
	}
	return nil
}

const MB = 1 << 20

type SimpleAuditor struct {
	RecordReadBody                bool     // Record read actions
	RecordRequestBodyContentTypes []string // Record only for these content types
	MaxBodySize                   int      // Max body size to record,0 means disable
	WhiteList                     []string // White list
}

func NewSimpleAuditor() *SimpleAuditor {
	return &SimpleAuditor{
		RecordReadBody: false,
		RecordRequestBodyContentTypes: []string{
			"application/json",
			"application/xml",
			"application/x-www-form-urlencoded",
		},
		MaxBodySize: 1 * MB,
	}
}

type AuditSSH struct {
	User          string   `json:"user,omitempty"`
	RemoteAddr    string   `json:"remoteAddr,omitempty"`
	LocalAddr     string   `json:"localAddr,omitempty"`
	SessionID     string   `json:"sessionID,omitempty"`
	ClientVersion string   `json:"clientVersion,omitempty"`
	ServerVersion string   `json:"serverVersion,omitempty"`
	PublicKey     string   `json:"publicKey,omitempty"`
	Command       string   `json:"command,omitempty"`
	Env           []string `json:"env,omitempty"`
}

type AuditRequest struct {
	HttpVersion string            `json:"httpVersion,omitempty"` // http version
	Method      string            `json:"method,omitempty"`      // method
	URL         string            `json:"url,omitempty"`         // full url
	Header      map[string]string `json:"header,omitempty"`      // header
	Body        []byte            `json:"body,omitempty"`        // ignore body if size > 1MB or stream.
	ClientIP    string            `json:"clientIP,omitempty"`    // client ip
	RemoteAddr  string            `json:"remoteAddr,omitempty"`
	LocalAddr   string            `json:"localAddr,omitempty"`
}

type AuditResponse struct {
	StatusCode   int               `json:"statusCode,omitempty"`   // status code
	Header       map[string]string `json:"header,omitempty"`       // header
	ResponseBody []byte            `json:"responseBody,omitempty"` // ignore body if size > 1MB or stream.
}

type AuditExtraMetadata map[string]string

type AuditLog struct {
	SSH *AuditSSH `json:"ssh,omitempty"` // for ssh only
	// request
	Request  AuditRequest  `json:"request,omitempty"`
	Response AuditResponse `json:"response,omitempty"`
	// authz
	Subject string `json:"subject,omitempty"` // username
	// Resource is the resource type, e.g. "pods", "namespaces/default/pods/nginx-xxx"
	// we can detect the resource type and name from the request path.
	// GET  /zoos/{zoo_id}/animals/{animal_id} 	-> get zoos,zoo_id,animals,animal_id
	// GET  /zoos/{zoo_id}/animals 				-> list zoos,zoo_id,animals,animal_id
	// POST /zoos/{zoo_id}/animals:set-free 	-> set-free zoos,zoo_id,animals
	Action       string             `json:"action,omitempty"`       // create, update, delete, get, list, set-free, etc.
	Domain       string             `json:"domain,omitempty"`       // for multi-tenant
	Parents      []AttrbuteResource `json:"parents,omitempty"`      // parent resources, e.g. "zoos/{zoo_id}",
	Resource     string             `json:"resource,omitempty"`     // resource type, e.g. "animals"
	ResourceName string             `json:"resourceName,omitempty"` //  "{animal_id}", or "" if list
	// metadata
	StartTime time.Time          `json:"startTime,omitempty"` // request start time
	EndTime   time.Time          `json:"endTime,omitempty"`   // request end time
	Metadata  AuditExtraMetadata `json:"metadata,omitempty"`  // extra metadata
}

type auditmetadaContext struct{}

var auditmetadaContextKey = auditmetadaContext{}

func WithAuditLog(ctx context.Context, log *AuditLog) context.Context {
	return context.WithValue(ctx, auditmetadaContextKey, log)
}

func AuditLogFromContext(ctx context.Context) *AuditLog {
	log, ok := ctx.Value(auditmetadaContextKey).(*AuditLog)
	if !ok {
		return nil
	}
	return log
}

func SetAuditExtra(req *http.Request, k, v string) {
	log := AuditLogFromContext(req.Context())
	if log == nil {
		return
	}
	if log.Metadata == nil {
		log.Metadata = make(AuditExtraMetadata)
	}
	log.Metadata[k] = v
}

func (a *SimpleAuditor) OnRequest(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, *AuditLog) {
	auditlog := &AuditLog{
		Request: AuditRequest{
			HttpVersion: r.Proto,
			Method:      r.Method,
			URL:         r.URL.String(),
			Header:      HttpHeaderToMap(r.Header),
			ClientIP:    ExtractClientIP(r),
		},
		StartTime: time.Now(),
	}
	respcachesize := 0
	if a.RecordReadBody || r.Method != http.MethodGet {
		auditlog.Request.Body = ReadBodySafely(r, a.RecordRequestBodyContentTypes, a.MaxBodySize)
		respcachesize = a.MaxBodySize
	}
	return &StatusResponseWriter{Inner: w, MaxCacheSize: respcachesize}, auditlog
}

func (a *SimpleAuditor) OnResponse(w http.ResponseWriter, r *http.Request, auditlog *AuditLog) {
	if auditlog == nil {
		return
	}
	if attr := AttributesFromContext(r.Context()); attr != nil {
		auditlog.Action = attr.Action
		if size := len(attr.Resources); size > 0 {
			parents, last := attr.Resources[:size-1], attr.Resources[size-1]
			auditlog.Parents, auditlog.Resource, auditlog.ResourceName = parents, last.Resource, last.Name
		}
	}
	auditlog.Subject = AuthenticateFromContext(r.Context()).User.Name
	auditlog.EndTime = time.Now()
	auditlog.Response = AuditResponse{
		Header: HttpHeaderToMap(w.Header()),
	}
	if statusWriter, ok := w.(*StatusResponseWriter); ok {
		auditlog.Response.StatusCode = statusWriter.Code
		auditlog.Response.ResponseBody = statusWriter.Cache
	}
}

func ExtractClientIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-Ip")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	return clientIP
}

func HttpHeaderToMap(header http.Header) map[string]string {
	m := make(map[string]string)
	for k, v := range header {
		m[k] = strings.Join(v, ",")
	}
	return m
}

func ReadBodySafely(req *http.Request, allowsContentType []string, maxReadSize int) []byte {
	contenttype, contentlen := req.Header.Get("Content-Type"), req.ContentLength
	if contenttype == "" || contentlen == 0 {
		return nil
	}
	allowed := slices.ContainsFunc(allowsContentType, func(s string) bool {
		return strings.HasPrefix(contenttype, s)
	})
	if !allowed {
		return nil
	}
	cachesize := maxReadSize
	if contentlen < int64(maxReadSize) {
		cachesize = int(contentlen)
	}
	if cachesize <= 0 {
		return nil
	}
	cachedbody := make([]byte, cachesize)
	n, err := io.ReadFull(req.Body, cachedbody)
	// io.ReadFull returns io.ErrUnexpectedEOF if EOF is encountered before filling the buffer.
	if err != nil && err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	req.Body = NewCachedBody(req.Body, cachedbody[:n], err)
	return cachedbody[:n]
}

var _ io.ReadCloser = &CachedBody{}

type CachedBody struct {
	cached []byte
	err    error // early read error
	readn  int
	body   io.ReadCloser
}

// NewCachedBody returns a new CachedBody.
// a CachedBody is a io.ReadCloser that read from cached first, then read from body.
func NewCachedBody(body io.ReadCloser, cached []byte, earlyerr error) *CachedBody {
	return &CachedBody{body: body, cached: cached, err: earlyerr}
}

func (w *CachedBody) Read(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.readn < len(w.cached) {
		n += copy(p, w.cached[w.readn:])
		w.readn += n
		if n == len(p) {
			return n, nil
		}
		p = p[n:] // continue read from body
	}
	bn, err := w.body.Read(p)
	n += bn
	return n, err
}

func (w *CachedBody) Close() error {
	return w.body.Close()
}

var _ http.ResponseWriter = &StatusResponseWriter{}

type StatusResponseWriter struct {
	Inner        http.ResponseWriter
	Code         int
	Cache        []byte
	MaxCacheSize int
}

func (w *StatusResponseWriter) Header() http.Header {
	return w.Inner.Header()
}

func (w *StatusResponseWriter) Write(p []byte) (n int, err error) {
	if w.Code == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if leftcachesize := w.MaxCacheSize - len(w.Cache); leftcachesize > 0 {
		if len(p) > leftcachesize {
			w.Cache = append(w.Cache, p[:leftcachesize]...)
		} else {
			w.Cache = append(w.Cache, p...)
		}
	}
	return w.Inner.Write(p)
}

func (w *StatusResponseWriter) WriteHeader(statusCode int) {
	w.Code = statusCode
	w.Inner.WriteHeader(statusCode)
}

package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/maps"
	"kubegems.io/library/rest/matcher"
	"kubegems.io/library/rest/request"
)

type Router interface {
	HandleRoute(route *Route) error
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	SetNotFound(handler http.Handler)
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
}

func UnsupportedMediaType(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "415 unsupported media type", http.StatusUnsupportedMediaType)
}

// The HyperText Transfer Protocol (HTTP) 406 Not Acceptable client error response code indicates
// that the server cannot produce a response matching the list of acceptable values
// defined in the request's proactive content negotiation headers,
// and that the server is unwilling to supply a default representation.
func NotAcceptable(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "406 not acceptable", http.StatusNotAcceptable)
}

func MediaTypeCheckFunc(accepts, produces []string, handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(accepts) > 0 && !MatchMIME(r.Header.Get("Content-Type"), accepts) {
			UnsupportedMediaType(w, r)
			return
		}
		if len(produces) > 0 && !MatchMIME(r.Header.Get("Accept"), produces) {
			NotAcceptable(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	}
}

func MatchMIME(accept string, supported []string) bool {
	base, _, _ := strings.Cut(accept, ";")
	accept = strings.TrimSpace(strings.ToLower(base))
	if accept == "" || accept == "*/*" || len(supported) == 0 {
		return true
	}
	for _, s := range supported {
		base, _, _ := strings.Cut(s, ";")
		s = strings.TrimSpace(strings.ToLower(base))
		if s == "*/*" || accept == s {
			return true
		}
	}
	return false
}

type MethodsHandler map[string]http.Handler

func (h MethodsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := h.handler(r)
	if handler == nil {
		w.Header().Add("Allow", strings.Join(maps.Keys(h), ","))
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		} else {
			MethodNotAllowed(w, r)
		}
		return
	}
	handler.ServeHTTP(w, r)
}

func (h MethodsHandler) handler(r *http.Request) http.Handler {
	if h == nil || len(h) == 0 {
		return nil
	}
	handler := h[r.Method]
	if handler == nil {
		handler = h[""]
	}
	return handler
}

type Mux struct {
	NotFound http.Handler
	Handlers matcher.Node[MethodsHandler]
}

func NewMux() *Mux {
	return &Mux{}
}

func (m *Mux) Handle(method, pattern string, handler http.Handler) error {
	_, node, err := m.Handlers.Get(pattern)
	if err != nil {
		return err
	}
	if node.Value == nil {
		node.Value = MethodsHandler{}
	}
	if _, ok := node.Value[method]; ok {
		return fmt.Errorf("already registered: %s %s", method, pattern)
	}
	node.Value[method] = handler
	return nil
}

func (m *Mux) SetNotFound(handler http.Handler) {
	m.NotFound = handler
}

func (m *Mux) HandleRoute(route *Route) error {
	method, pattern := route.Method, route.Path
	sections, node, err := m.Handlers.Get(pattern)
	if err != nil {
		return err
	}
	if node.Value == nil {
		node.Value = MethodsHandler{}
	}
	if _, ok := node.Value[method]; ok {
		return fmt.Errorf("already registered: %s %s", method, pattern)
	}
	node.Value[method] = route
	// complete pathvars
	vars := []Param{}
	for _, section := range sections {
		for _, elem := range section {
			if elem.VarName != "" {
				// check already exists
				exists := slices.ContainsFunc(route.Params, func(i Param) bool {
					return i.Name == elem.VarName
				})
				if exists {
					continue
				}
				param := Param{
					Name: elem.VarName,
					Kind: ParamKindPath,
					Type: "string",
				}
				if elem.Validate != nil {
					param.Pattern = elem.Validate.String()
				}
				vars = append(vars, param)
			}
		}
	}
	route.Params = append(vars, route.Params...)
	route.Path = matcher.NoRegexpString(sections)
	return nil
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	node, vars := m.Handlers.Match(r.URL.Path, nil)
	if node == nil || node.Value == nil {
		if m.NotFound == nil {
			http.NotFound(w, r)
		} else {
			m.NotFound.ServeHTTP(w, r)
		}
		return
	}
	varsmap := make(map[string]string, len(vars))
	for _, v := range vars {
		varsmap[v.Name] = v.Value
	}
	r = r.WithContext(context.WithValue(r.Context(), httpVarsContextKey{}, varsmap))
	node.Value.ServeHTTP(w, r)
}

type httpVarsContextKey struct{}

func PathVars(r *http.Request) map[string]string {
	if vars, ok := r.Context().Value(httpVarsContextKey{}).(map[string]string); ok {
		return vars
	}
	return nil
}

func NewDefauBodyltValidation() func(r *http.Request, data any) error {
	v := validator.New()
	v.RegisterValidation("regxp", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.String {
			return true
		}
		regxp := fl.Param()
		if regxp == "" {
			return true
		}
		if matched, err := regexp.MatchString(regxp, fl.Field().String()); err != nil {
			return false
		} else if matched {
			return true
		}
		return false
	})
	return func(r *http.Request, data any) error {
		return v.StructCtx(r.Context(), data)
	}
}

func init() {
	request.PathVarsFunc = PathVars
	request.ValidateBody = NewDefauBodyltValidation()
}

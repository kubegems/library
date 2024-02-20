package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"kubegems.io/library/rest/matcher"
	"kubegems.io/library/rest/request"
)

type Router interface {
	HandleRoute(route *Route) error
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	SetNotFound(handler http.Handler)
	SetMethodNotAllowed(handler http.Handler)
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

func (h MethodsHandler) NotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Allow", strings.Join(maps.Keys(h), ","))
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
	} else {
		MethodNotAllowed(w, r)
	}
}

func (h MethodsHandler) selectHandler(r *http.Request) http.Handler {
	if h == nil || len(h) == 0 {
		return nil
	}
	for _, candidate := range []string{r.Method, ""} {
		if handler, ok := h[candidate]; ok {
			return handler
		}
	}
	return nil
}

type Mux struct {
	NotFound         http.Handler
	MethodNotAllowed http.Handler
	Tree             matcher.Node[MethodsHandler]
}

func NewMux() *Mux {
	return &Mux{}
}

func (m *Mux) Handle(method, pattern string, handler http.Handler) error {
	_, node, err := m.Tree.Get(pattern)
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

func (m *Mux) SetMethodNotAllowed(handler http.Handler) {
	m.MethodNotAllowed = handler
}

func (m *Mux) HandleRoute(route *Route) error {
	method, pattern := route.Method, route.Path
	sections, node, err := m.Tree.Get(pattern)
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
	// complete pathparam from sections if not exists
	completePathParam(route, sections)
	return nil
}

func completePathParam(route *Route, sections []matcher.Section) {
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
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	node, vars := m.Tree.Match(r.URL.Path, nil)
	if node == nil || node.Value == nil {
		if m.NotFound == nil {
			http.NotFound(w, r)
		} else {
			m.NotFound.ServeHTTP(w, r)
		}
		return
	}
	reqvars := make([]request.PathVar, len(vars))
	for i, v := range vars {
		reqvars[i] = request.PathVar{Key: v.Name, Value: v.Value}
	}
	r = r.WithContext(context.WithValue(r.Context(), httpVarsContextKey{}, reqvars))

	if handler := node.Value.selectHandler(r); handler != nil {
		handler.ServeHTTP(w, r)
		return
	}
	if m.MethodNotAllowed != nil {
		m.MethodNotAllowed.ServeHTTP(w, r)
		return
	}
	node.Value.NotAllowed(w, r)
}

type httpVarsContextKey struct{}

func PathVars(r *http.Request) request.PathVarList {
	if vars, ok := r.Context().Value(httpVarsContextKey{}).([]request.PathVar); ok {
		return vars
	}
	return nil
}

var (
	NameRegexp          = regexp.MustCompile(`^[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*$`)
	NameWithSlashRegexp = regexp.MustCompile(`^[a-zA-Z0-9]+(?:[._/-][a-zA-Z0-9]+)*$`)
)

func NewDefauBodyltValidation() func(r *http.Request, data any) error {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		// skip if tag key says it should be ignored
		if name == "-" {
			return ""
		}
		return name
	})
	v.RegisterValidation("regexp", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.String {
			return true
		}
		regxp := fl.Param()
		if regxp == "" {
			return true
		}
		if matched, _ := regexp.MatchString(regxp, fl.Field().String()); matched {
			return true
		}
		return false
	})
	v.RegisterValidation("name", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.String {
			return true
		}
		return NameRegexp.MatchString(fl.Field().String())
	})
	v.RegisterValidation("names", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.String {
			return true
		}
		return NameWithSlashRegexp.MatchString(fl.Field().String())
	})
	return func(r *http.Request, data any) error {
		return v.StructCtx(r.Context(), data)
	}
}

func init() {
	request.PathVarsFunc = PathVars
	request.ValidateBody = NewDefauBodyltValidation()
}

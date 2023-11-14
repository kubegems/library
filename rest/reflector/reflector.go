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

package reflector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	libreflect "kubegems.io/library/reflect"
	"kubegems.io/library/rest/api"
	"kubegems.io/library/rest/request"
	"kubegems.io/library/rest/response"
	libstrings "kubegems.io/library/strings"
)

func RegisterController(prefix string, parents []string, controller any) ([]ConvertedHandler, error) {
	v := reflect.ValueOf(controller)
	t := v.Type()
	handlers := make([]ConvertedHandler, 0, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		if m := t.Method(i); m.IsExported() {
			handlers = append(handlers, parseMethod(prefix, parents, v, m))
		}
	}
	return handlers, nil
}

type ConvertedHandler struct {
	Method   string
	Path     string
	Desc     string // openapi description
	Resource string // openapi resource name
	ReqArgs  []Argv
	RespArgs []Argv
	Handler  http.Handler
}

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
)

type argloc int

const (
	arglocContext argloc = 1 << iota
	arglocBody
	arglocPath
	arglocQuery
	arglocHeader
	arglocForm
	arglocFile
	arglocError
)

type Argv struct {
	Loc  argloc
	Typ  reflect.Type
	Name string // name in path, query, header, form
}

// parseMethod parse method to http.Handler
// GetJob				GET jobs/{job}
// GetJobStatus			GET jobs/{job}/status/{status}
// ListJobStatus   		GET jobs/{job}/status
// CreateJobStatus		POST jobs/{job}/status
// StartJob		   		POST jobs/{job}:start
func parseMethod(prefix string, pathvarnames []string, arg0 reflect.Value, reflectMethod reflect.Method) ConvertedHandler {
	handler := &ConvertedHandler{}

	pathvarnames = applyMethodPath(prefix, pathvarnames, reflectMethod.Name, handler)

	reqargs, respargs := parseArgs(handler.Method, reflectMethod, pathvarnames)
	handler.ReqArgs, handler.RespArgs = reqargs, respargs

	handler.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		callargs, err := prepareCallArgs(r, arg0, reqargs)
		if err != nil {
			response.Error(w, err)
			return
		}
		// call method
		results := reflectMethod.Func.Call(callargs)
		if len(results) == 0 {
			return
		}
		for i := len(respargs) - 1; i >= 0; i-- {
			switch respargs[i].Loc {
			case arglocBody:
				response.OK(w, results[i].Interface())
				return
			case arglocError:
				// check is nil error
				if results[i].IsNil() {
					continue
				}
				response.Error(w, results[i].Interface().(error))
				return
			case arglocHeader:
				w.Header().Set(respargs[i].Name, fmt.Sprintf("%v", results[i].Interface()))
			}
		}
		// default response
		response.OK(w, "OK")
	})
	return *handler
}

func applyMethodPath(prefix string, pathvarnames []string, methodName string, ch *ConvertedHandler) []string {
	words := libstrings.SplitWords(methodName)
	for i := range words {
		words[i] = strings.ToLower(strings.TrimSpace(words[i]))
	}
	httpMethod, isPlural, customAction := "", false, ""
	action := words[0]
	switch action {
	case "create":
		httpMethod, isPlural = http.MethodPost, true
	case "update":
		httpMethod, isPlural = http.MethodPut, false
	case "delete", "remove":
		httpMethod, isPlural = http.MethodDelete, false
	case "get":
		httpMethod, isPlural = http.MethodGet, false
	case "list":
		httpMethod, isPlural = http.MethodGet, true
	default:
		httpMethod, isPlural, customAction = http.MethodPost, false, action
	}
	path := prefix

	pathvarnames = append(pathvarnames, words[1:]...)

	for _, name := range pathvarnames {
		path += fmt.Sprintf("/%s/{%s}", libstrings.ToPlural(name), libstrings.ToSingular(name))
	}
	if isPlural {
		// remove last path var
		path = path[:strings.LastIndex(path, "/")]
	}
	if customAction != "" {
		path += ":" + customAction
	}
	ch.Method = httpMethod
	ch.Path = path
	ch.Desc = strings.Title(action) + " " + strings.Title(strings.Join(pathvarnames, " "))
	if len(pathvarnames) > 0 {
		ch.Resource = strings.Title(libstrings.ToPlural(pathvarnames[len(pathvarnames)-1]))
	}
	return pathvarnames
}

func parseArgs(method string, reflectMethod reflect.Method, pathvarnames []string) ([]Argv, []Argv) {
	t := reflectMethod.Type
	reqargs := make([]Argv, 0, t.NumIn()-1)
	pathvarindex := 0
	hasBody := method != http.MethodGet
	for i := 1; i < t.NumIn(); i++ {
		inType := t.In(i)
		if inType.Implements(contextType) {
			reqargs = append(reqargs, Argv{Loc: arglocContext})
			continue
		}
		switch inType.Kind() {
		// pathvar
		case reflect.String, reflect.Bool,
			reflect.Int, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64,
			reflect.Uint, reflect.Uint32, reflect.Uint64,
			reflect.Complex64, reflect.Complex128:
			argv := Argv{Loc: arglocPath, Typ: inType}
			if pathvarindex < len(pathvarnames) {
				argv.Name = libstrings.ToSingular(pathvarnames[pathvarindex])
				pathvarindex++
			}
			reqargs = append(reqargs, argv)
		// body or query
		case reflect.Struct, reflect.Map, reflect.Ptr, reflect.Slice, reflect.Interface:
			if hasBody {
				reqargs = append(reqargs, Argv{Loc: arglocBody, Typ: inType})
				hasBody = false
			} else {
				reqargs = append(reqargs, Argv{Loc: arglocQuery, Typ: inType})
			}
		}
	}
	respargs := make([]Argv, 0, t.NumOut())
	hasRespbody := false
	for i := 0; i < t.NumOut(); i++ {
		outType := t.Out(i)
		if outType.Implements(errorType) {
			respargs = append(respargs, Argv{Loc: arglocError, Typ: outType})
			continue
		}
		if !hasRespbody {
			respargs = append(respargs, Argv{Loc: arglocBody, Typ: outType})
			hasRespbody = true
		} else {
			respargs = append(respargs, Argv{Loc: arglocHeader, Typ: outType})
		}
	}
	return reqargs, respargs
}

func prepareCallArgs(r *http.Request, arg0 reflect.Value, args []Argv) ([]reflect.Value, error) {
	pathvars, queries := api.PathVars(r), r.URL.Query()
	callargs := []reflect.Value{arg0}
	for _, arg := range args {
		switch arg.Loc {
		case arglocContext:
			callargs = append(callargs, reflect.ValueOf(r.Context()))
		case arglocPath:
			if arg.Name == "" {
				callargs = append(callargs, reflect.Zero(arg.Typ))
				continue
			}
			switch arg.Typ.Kind() {
			case reflect.String:
				callargs = append(callargs, reflect.ValueOf(pathvars[arg.Name]))
			case reflect.Bool:
				callargs = append(callargs, reflect.ValueOf(pathvars[arg.Name] == "true"))
			case reflect.Int, reflect.Int32, reflect.Int64:
				v, _ := strconv.ParseInt(pathvars[arg.Name], 10, 64)
				callargs = append(callargs, reflect.ValueOf(v))
			case reflect.Float32, reflect.Float64:
				v, _ := strconv.ParseFloat(pathvars[arg.Name], 64)
				callargs = append(callargs, reflect.ValueOf(v))
			case reflect.Uint, reflect.Uint32, reflect.Uint64:
				v, _ := strconv.ParseUint(pathvars[arg.Name], 10, 64)
				callargs = append(callargs, reflect.ValueOf(v))
			case reflect.Complex64, reflect.Complex128:
				v, _ := strconv.ParseComplex(pathvars[arg.Name], 128)
				callargs = append(callargs, reflect.ValueOf(v))
			}
		case arglocBody:
			body := reflect.New(arg.Typ).Elem()
			if err := decodeBody(r, body); err != nil {
				return nil, err
			}
			callargs = append(callargs, body.Elem())
		case arglocQuery:
			query := reflect.New(arg.Typ)
			for k := range queries {
				_ = libreflect.SetFiledValue(query.Interface(), k, queries.Get(k))
			}
			callargs = append(callargs, query.Elem())
		}
	}
	return callargs, nil
}

func decodeBody(r *http.Request, v reflect.Value) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	switch v.Interface().(type) {
	case io.Reader:
		v.Set(reflect.ValueOf(r.Body))
		return nil
	case []byte:
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return err
		}
		v.SetBytes(b)
		return nil
	default:
		return request.Body(r, v.Addr().Interface())
	}
}

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
	"net/http"
	"reflect"

	"github.com/go-openapi/spec"
	libreflect "kubegems.io/library/reflect"
	"kubegems.io/library/rest/openapi"
)

func BuildOpenAPIRoute(list []ConvertedHandler) map[string]spec.PathItem {
	paths := map[string]spec.PathItem{}
	for _, handler := range list {
		pathItem := paths[handler.Path]
		operation := buildRouteOperation(handler)
		switch handler.Method {
		case "GET":
			pathItem.Get = operation
		case "POST":
			pathItem.Post = operation
		case "PUT":
			pathItem.Put = operation
		case "DELETE":
			pathItem.Delete = operation
		case "PATCH":
			pathItem.Patch = operation
		case "HEAD":
			pathItem.Head = operation
		case "OPTIONS":
			pathItem.Options = operation
		}
		paths[handler.Path] = pathItem
	}
	return paths
}

func buildRouteOperation(handler ConvertedHandler) *spec.Operation {
	return &spec.Operation{
		OperationProps: spec.OperationProps{
			Summary:     handler.Resource,
			Description: handler.Desc,
			Tags:        []string{handler.Resource},
			Parameters: func() []spec.Parameter {
				params := []spec.Parameter{}
				for _, reqarg := range handler.ReqArgs {
					switch reqarg.Loc {
					case arglocQuery:
						params = append(params, buildQueryParams(reqarg)...)
					case arglocPath:
						if reqarg.Name == "" {
							continue
						}
						params = append(params, spec.Parameter{
							ParamProps:   spec.ParamProps{Name: reqarg.Name, In: "path", Required: true},
							SimpleSchema: spec.SimpleSchema{Type: simpleType(reqarg.Typ)},
						})
					case arglocHeader:
						params = append(params, spec.Parameter{
							ParamProps:   spec.ParamProps{Name: reqarg.Name, In: "header"},
							SimpleSchema: spec.SimpleSchema{Type: simpleType(reqarg.Typ)},
						})
					case arglocBody:
						params = append(params, spec.Parameter{
							ParamProps: spec.ParamProps{
								Name:   "body",
								In:     "body",
								Schema: SchemaOfType(reqarg.Typ),
							},
						})
					}
				}
				return params
			}(),
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: map[int]spec.Response{
						http.StatusOK: {
							ResponseProps: spec.ResponseProps{
								Description: "OK",
								Headers: func() map[string]spec.Header {
									headers := map[string]spec.Header{}
									for _, resparg := range handler.RespArgs {
										if resparg.Loc == arglocHeader {
											headers[resparg.Name] = spec.Header{
												HeaderProps: spec.HeaderProps{},
											}
										}
									}
									return headers
								}(),
								Schema: bodySchema(handler.RespArgs),
							},
						},
					},
				},
			},
		},
	}
}

func simpleType(v reflect.Type) string {
	switch v.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Struct:
		return "object"
	case reflect.Slice:
		return "array"
	default:
		return "string"
	}
}

func buildQueryParams(arg Argv) []spec.Parameter {
	params := []spec.Parameter{}
	switch arg.Typ.Kind() {
	case reflect.Struct:
		newv := reflect.New(arg.Typ).Interface()
		libreflect.EachFiledValue(newv, func(pathes []string, val reflect.Value) error {
			if len(pathes) == 1 {
				params = append(params, spec.Parameter{
					ParamProps:   spec.ParamProps{Name: pathes[0], In: "query"},
					SimpleSchema: spec.SimpleSchema{Type: "string"},
				})
			}
			return nil
		})
	}
	return params
}

func bodySchema(args []Argv) *spec.Schema {
	for _, arg := range args {
		if arg.Loc == arglocBody {
			return SchemaOfType(arg.Typ)
		}
	}
	return nil
}

func SchemaOfType(t reflect.Type) *spec.Schema {
	return openapi.DefaultBuilder.Build(reflect.New(t).Interface())
}

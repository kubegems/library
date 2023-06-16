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

package api

import (
	"net/http"
	"strings"

	"github.com/go-openapi/spec"
	"golang.org/x/exp/maps"
	"kubegems.io/library/rest/mux"
	"kubegems.io/library/rest/openapi"
)

type RestModule interface {
	RegisterRoute(r *Group)
}

type Function = http.HandlerFunc

type Tree struct {
	Group           *Group
	RouteUpdateFunc func(r *Route) // can update route setting before build
	built           map[string]map[string]Route
}

func (t *Tree) Build() map[string]map[string]Route {
	if t.built == nil {
		items := map[string]map[string]Route{}
		t.buildItems(items, t.Group, nil, nil, "")
		t.built = items
	}
	return t.built
}

func (t *Tree) buildItems(items map[string]map[string]Route, group *Group, baseparams []Param, basetags []string, basepath string) {
	basepath = strings.TrimRight(basepath, "/") + "/" + strings.TrimLeft(group.path, "/")
	baseparams = append(baseparams, group.params...)
	if group.tag != "" {
		basetags = append(basetags, group.tag)
	}
	for i := range group.routes {
		route := group.routes[i] // todo: deep copy route
		route.Tags = append(basetags, route.Tags...)
		route.Params = append(baseparams, route.Params...)
		route.Path = basepath + route.Path
		methods, ok := items[route.Path]
		if !ok {
			methods = map[string]Route{}
			items[route.Path] = methods
		}
		if t.RouteUpdateFunc != nil {
			t.RouteUpdateFunc(route)
		}
		methods[route.Method] = *route
	}
	for _, group := range group.subGroups {
		t.buildItems(items, group, baseparams, basetags, basepath)
	}
}

type Group struct {
	tag       string
	path      string
	params    []Param // common params apply to all routes in the group
	routes    []*Route
	subGroups []*Group // sub groups
}

func NewGroup(path string) *Group {
	return &Group{path: path}
}

func (g *Group) Tag(name string) *Group {
	g.tag = name
	return g
}

func (g *Group) AddRoutes(rs ...*Route) *Group {
	g.routes = append(g.routes, rs...)
	return g
}

func (g *Group) AddSubGroup(groups ...*Group) *Group {
	g.subGroups = append(g.subGroups, groups...)
	return g
}

func (g *Group) Parameters(params ...Param) *Group {
	g.params = append(g.params, params...)
	return g
}

type Route struct {
	Summary    string
	Path       string
	Method     string
	Deprecated bool
	Func       Function
	Tags       []string
	Consumes   []string
	Produces   []string
	Params     []Param
	Responses  []ResponseMeta
	Properties map[string]interface{}
}

type ResponseMeta struct {
	Code        int
	Headers     map[string]string
	Body        interface{}
	Description string
}

func Do(method string, path string) *Route {
	return &Route{
		Method: method,
		Path:   path,
	}
}

func OPTIONS(path string) *Route {
	return Do(http.MethodOptions, path)
}

func HEAD(path string) *Route {
	return Do(http.MethodHead, path)
}

func GET(path string) *Route {
	return Do(http.MethodGet, path)
}

func POST(path string) *Route {
	return Do(http.MethodPost, path)
}

func PUT(path string) *Route {
	return Do(http.MethodPut, path)
}

func PATCH(path string) *Route {
	return Do(http.MethodPatch, path)
}

func DELETE(path string) *Route {
	return Do(http.MethodDelete, path)
}

func (n *Route) To(fun Function) *Route {
	n.Func = fun
	return n
}

func (n *Route) ShortDesc(summary string) *Route {
	n.Summary = summary
	return n
}

func (n *Route) Doc(summary string) *Route {
	n.Summary = summary
	return n
}

func (n *Route) Paged() *Route {
	n.Params = append(n.Params, QueryParameter("page", "page number").Optional())
	n.Params = append(n.Params, QueryParameter("size", "page size").Optional())
	return n
}

func (n *Route) Parameters(params ...Param) *Route {
	n.Params = append(n.Params, params...)
	return n
}

// Accept types of all the responses
func (n *Route) Accept(mime ...string) *Route {
	n.Consumes = append(n.Consumes, mime...)
	return n
}

// ContentType of all available responses type
func (n *Route) ContentType(mime ...string) *Route {
	n.Produces = append(n.Produces, mime...)
	return n
}

func (n *Route) Response(body interface{}, descs ...string) *Route {
	n.Responses = append(n.Responses, ResponseMeta{Code: http.StatusOK, Body: body, Description: strings.Join(descs, "")})
	return n
}

func (n *Route) SetProperty(k string, v interface{}) *Route {
	if n.Properties == nil {
		n.Properties = make(map[string]interface{})
	}
	n.Properties[k] = v
	return n
}

func (n *Route) Tag(tags ...string) *Route {
	n.Tags = append(n.Tags, tags...)
	return n
}

type ParamKind string

const (
	ParamKindPath   ParamKind = "path"
	ParamKindQuery  ParamKind = "query"
	ParamKindHeader ParamKind = "header"
	ParamKindForm   ParamKind = "formData"
	ParamKindBody   ParamKind = "body"
)

type Param struct {
	Name        string
	Kind        ParamKind
	Type        string
	Enum        []any
	Default     any
	IsOptional  bool
	Description string
	Example     any
}

func BodyParameter(name string, value any) Param {
	return Param{Kind: ParamKindBody, Name: name, Example: value}
}

func FormParameter(name string, description string) Param {
	return Param{Kind: ParamKindForm, Name: name, Description: description}
}

func PathParameter(name string, description string) Param {
	return Param{Kind: ParamKindPath, Name: name, Description: description}
}

func QueryParameter(name string, description string) Param {
	return Param{Kind: ParamKindQuery, Name: name, Description: description}
}

func (p Param) Optional() Param {
	p.IsOptional = true
	return p
}

func (p Param) Desc(desc string) Param {
	p.Description = desc
	return p
}

func (p Param) DataType(t string) Param {
	p.Type = t
	return p
}

func (p Param) In(t ...any) Param {
	p.Enum = append(p.Enum, t...)
	return p
}

func (t *Tree) AddToMux(mux *mux.MethodServeMux) {
	for path, methods := range t.Build() {
		for method, route := range methods {
			mux.HandleFunc(method, path, route.Func)
		}
	}
}

func (tree Tree) AddToSwagger(swagger *spec.Swagger, builder *openapi.Builder) {
	if swagger.Paths == nil {
		swagger.Paths = &spec.Paths{}
	}
	if swagger.Paths.Paths == nil {
		swagger.Paths.Paths = map[string]spec.PathItem{}
	}
	for path, methods := range tree.Build() {
		pathItem := spec.PathItem{}
		for method, route := range methods {
			operation := buildRouteOperation(route, builder)
			switch method {
			case http.MethodGet, "":
				pathItem.Get = operation
			case http.MethodPost:
				pathItem.Post = operation
			case http.MethodPut:
				pathItem.Put = operation
			case http.MethodDelete:
				pathItem.Delete = operation
			case http.MethodPatch:
				pathItem.Patch = operation
			case http.MethodHead:
				pathItem.Head = operation
			case http.MethodOptions:
				pathItem.Options = operation
			}
		}
		swagger.Paths.Paths[path] = pathItem
	}
	if swagger.Definitions == nil {
		swagger.Definitions = map[string]spec.Schema{}
	}
	maps.Copy(swagger.Definitions, builder.Definitions)
}

func buildRouteOperation(route Route, builder *openapi.Builder) *spec.Operation {
	return &spec.Operation{
		OperationProps: spec.OperationProps{
			ID: "",
			Tags: func() []string {
				if len(route.Tags) > 0 {
					// only use the last tag
					return route.Tags[len(route.Tags)-1:]
				}
				return route.Tags
			}(),
			Summary:     route.Summary,
			Description: route.Summary,
			Consumes:    route.Consumes,
			Produces:    route.Produces,
			Deprecated:  route.Deprecated,
			Parameters: func() []spec.Parameter {
				var parameters []spec.Parameter
				for _, param := range route.Params {
					parameters = append(parameters, spec.Parameter{
						ParamProps: spec.ParamProps{
							Name:        param.Name,
							Description: param.Description,
							In:          string(param.Kind),
							Schema:      builder.Build(param.Example),
							Required:    param.Kind == ParamKindPath || param.Kind == ParamKindBody || !param.IsOptional,
						},
						CommonValidations: spec.CommonValidations{
							Enum: param.Enum,
						},
						SimpleSchema: spec.SimpleSchema{
							Type:    param.Type,
							Default: param.Default,
						},
					})
				}
				return parameters
			}(),
			Responses: &spec.Responses{
				ResponsesProps: spec.ResponsesProps{
					StatusCodeResponses: func() map[int]spec.Response {
						responses := map[int]spec.Response{}
						for _, resp := range route.Responses {
							responses[resp.Code] = spec.Response{
								ResponseProps: spec.ResponseProps{
									Description: resp.Description,
									Schema:      builder.Build(resp.Body),
									Headers: func() map[string]spec.Header {
										headers := map[string]spec.Header{}
										for k, h := range resp.Headers {
											headers[k] = spec.Header{HeaderProps: spec.HeaderProps{Description: h}}
										}
										return headers
									}(),
								},
							}
						}
						if len(responses) == 0 {
							responses[200] = spec.Response{ResponseProps: spec.ResponseProps{Description: "OK"}}
						}
						return responses
					}(),
				},
			},
		},
	}
}

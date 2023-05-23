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

package openapi

// type SwaggerBuilder struct {
// 	openapi *Builder
// }

// func NewSwaggerBuilder() *SwaggerBuilder {
// 	return &SwaggerBuilder{
// 		openapi: NewBuilder(InterfaceBuildOptionOverride),
// 	}
// }

// func (b *SwaggerBuilder) BuildRouteTree(tree Tree) *spec.Swagger {
// 	return &spec.Swagger{
// 		SwaggerProps: spec.SwaggerProps{
// 			Swagger: "2.0",
// 			Schemes: []string{"http", "https"},
// 			Paths: &spec.Paths{
// 				Paths: func() map[string]spec.PathItem {
// 					paths := map[string]spec.PathItem{}
// 					for path, methods := range tree.Build() {
// 						pathItem := spec.PathItem{}
// 						for method, route := range methods {
// 							operation := b.buildRouteOperation(route)
// 							switch method {
// 							case http.MethodGet, "":
// 								pathItem.Get = operation
// 							case http.MethodPost:
// 								pathItem.Post = operation
// 							case http.MethodPut:
// 								pathItem.Put = operation
// 							case http.MethodDelete:
// 								pathItem.Delete = operation
// 							case http.MethodPatch:
// 								pathItem.Patch = operation
// 							case http.MethodHead:
// 								pathItem.Head = operation
// 							case http.MethodOptions:
// 								pathItem.Options = operation
// 							}
// 						}
// 						paths[path] = pathItem
// 					}
// 					return paths
// 				}(),
// 			},
// 		},
// 	}
// }

// func (b *SwaggerBuilder) buildRouteOperation(route Route) *spec.Operation {
// 	return &spec.Operation{
// 		OperationProps: spec.OperationProps{
// 			ID:          "",
// 			Tags:        route.Tags,
// 			Summary:     route.Summary,
// 			Description: route.Summary,
// 			Consumes:    route.Consumes,
// 			Produces:    route.Produces,
// 			Deprecated:  route.Deprecated,
// 			Parameters: func() []spec.Parameter {
// 				var parameters []spec.Parameter
// 				for _, param := range route.Params {
// 					parameters = append(parameters, spec.Parameter{
// 						ParamProps: spec.ParamProps{
// 							Name:        param.Name,
// 							Description: param.Description,
// 							In:          string(param.Kind),
// 							Schema:      b.openapi.Build(param.Example),
// 						},
// 						CommonValidations: spec.CommonValidations{
// 							Enum:    param.Enum,
// 							Pattern: "", // TODO: support path pattern
// 						},
// 						SimpleSchema: spec.SimpleSchema{
// 							Type:    param.Type,
// 							Example: param.Example,
// 							Default: param.Default,
// 						},
// 					})
// 				}
// 				return parameters
// 			}(),
// 			Responses: &spec.Responses{
// 				ResponsesProps: spec.ResponsesProps{
// 					StatusCodeResponses: func() map[int]spec.Response {
// 						responses := map[int]spec.Response{}
// 						for _, resp := range route.Responses {
// 							responses[resp.Code] = spec.Response{
// 								ResponseProps: spec.ResponseProps{
// 									Description: resp.Description,
// 									Schema:      b.openapi.Build(resp.Body),
// 									Headers: func() map[string]spec.Header {
// 										headers := map[string]spec.Header{}
// 										for k, h := range resp.Headers {
// 											headers[k] = spec.Header{HeaderProps: spec.HeaderProps{Description: h}}
// 										}
// 										return headers
// 									}(),
// 								},
// 							}
// 						}
// 						if len(responses) == 0 {
// 							responses[200] = spec.Response{ResponseProps: spec.ResponseProps{Description: "OK"}}
// 						}
// 						return responses
// 					}(),
// 				},
// 			},
// 		},
// 	}
// }

// // sanitizePath removes regex expressions from named path params,
// // since openapi only supports setting the pattern as a property named "pattern".
// // Expressions like "/api/v1/{name:[a-z]}/" are converted to "/api/v1/{name}/".
// // The second return value is a map which contains the mapping from the path parameter
// // name to the extracted pattern
// func sanitizePath(restfulPath string) (string, map[string]string) {
// 	openapiPath := ""
// 	patterns := map[string]string{}
// 	for _, fragment := range strings.Split(restfulPath, "/") {
// 		if fragment == "" {
// 			continue
// 		}
// 		if strings.HasPrefix(fragment, "{") && strings.Contains(fragment, ":") {
// 			split := strings.Split(fragment, ":")
// 			fragment = split[0][1:]
// 			pattern := split[1][:len(split[1])-1]
// 			patterns[fragment] = pattern
// 			fragment = "{" + fragment + "}"
// 		}
// 		openapiPath += "/" + fragment
// 	}
// 	return openapiPath, patterns
// }

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
	"github.com/go-openapi/spec"
	"kubegems.io/library/rest/response"
)

func KubegemsSwaggerCompleter(swagger *spec.Swagger) {
	swagger.Schemes = []string{"http", "https"}
	swagger.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "KubeGems",
			Description: "kubegems api",
			Contact: &spec.ContactInfo{
				ContactInfoProps: spec.ContactInfoProps{
					Name:  "kubegems",
					URL:   "http://kubegems.io",
					Email: "support@kubegems.io",
				},
			},
			Version: "v1",
		},
	}
	swagger.Schemes = []string{"http", "https"}
	swagger.SecurityDefinitions = map[string]*spec.SecurityScheme{
		"jwt": spec.APIKeyAuth("Authorization", "header"),
	}
	swagger.Security = []map[string][]string{{"jwt": {}}}
}

func ListWrrapperFunc(r *Route) {
	paged := false
	for _, item := range r.Params {
		if item.Kind == ParamKindQuery && item.Name == "page" {
			paged = true
			break
		}
	}
	for i, v := range r.Responses {
		//  if query parameters exist, response as a paged response
		if paged {
			r.Responses[i].Body = response.Response{Data: response.Page[any]{List: []any{v.Body}}}
		} else {
			r.Responses[i].Body = response.Response{Data: v.Body}
		}
	}
}

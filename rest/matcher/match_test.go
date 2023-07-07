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

package matcher

import (
	"reflect"
	"regexp"
	"testing"
)

func Test_matcher_Register(t *testing.T) {
	matcher := NewMatcher[string]()

	type args struct {
		pattern string
		val     string
	}
	tests := []struct {
		args     args
		wanrVars []string
		wantErr  bool
	}{
		{
			args:     args{pattern: "/api/{path}*", val: "-"},
			wanrVars: []string{"path"},
		},
		{
			args:     args{pattern: "/api/{name}/{path}", val: "-"},
			wanrVars: []string{"name", "path"},
		},
		{
			args:    args{pattern: "/api/{name/{path}", val: "-"},
			wantErr: true,
		},
		{
			args:     args{pattern: "/api/{name}/{path}:action", val: "-"},
			wanrVars: []string{"name", "path"},
		},
		{
			args:     args{pattern: "/api/{name}/{path}:action", val: "-"},
			wanrVars: []string{"name", "path"},
			wantErr:  true, // repeat register
		},
		{
			args:     args{pattern: `/api/{name}/{path:\\[}:action`, val: "-"},
			wanrVars: []string{"name", "path"},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.args.pattern, func(t *testing.T) {
			pathvars, err := matcher.Register(tt.args.pattern, tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("matcher.Register() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(pathvars, tt.wanrVars) {
				t.Errorf("matcher.Register() pathvars = %v, wanrVars %v", pathvars, tt.wanrVars)
			}
		})
	}
}

func Test_matcher_Match(t *testing.T) {
	tests := []struct {
		registered []string
		req        string
		matched    bool
		wantMatch  string
		vars       map[string]string
	}{
		{
			registered: []string{
				"/api/{a}",
				"/api/v{a}*",
				"/api/v1",
				"/apis",
				"/api/{a}/{b}/{c}",
				"/api/{path}*",
			},
			req:       "/api/v1/g/v/k",
			matched:   true,
			wantMatch: "/api/v{a}*",
			vars: map[string]string{
				"a": "1/g/v/k",
			},
		},
		{
			registered: []string{
				"/v1/service-proxy/{realpath}*",
				"/v1/{group}/{version}/{resource}",
			},
			req:       "/v1/service-proxy/js/t2.js",
			matched:   true,
			wantMatch: "/v1/service-proxy/{realpath}*",
			vars: map[string]string{
				"realpath": "js/t2.js",
			},
		},
		{
			registered: []string{
				"/v1/{group}/{version}/{resource}/{name}",
				"/v1/{group}/{version}/configmap/{name}",
			},
			req:       "/v1/core/v1/configmap/abc",
			matched:   true,
			wantMatch: "/v1/{group}/{version}/configmap/{name}",
			vars: map[string]string{
				"group":   "core",
				"version": "v1",
				"name":    "abc",
			},
		},
		{
			registered: []string{
				"/api/v{a}*",
				"/api/{a}/{b}/{c}",
				"/api/{path}*",
			},
			req:       "/api/v2/v/k",
			matched:   true,
			wantMatch: "/api/{a}/{b}/{c}",
			vars: map[string]string{
				"a": "v2",
				"b": "v",
				"c": "k",
			},
		},
		{
			registered: []string{
				"/api/s",
			},
			req:     "/api",
			matched: false,
			vars:    map[string]string{},
		},
		{
			registered: []string{
				"/api/dog:wang",
			},
			req:     "/api/dog",
			matched: false,
			vars:    map[string]string{},
		},
		{
			registered: []string{
				"/api/{dog:[a-z]+}",
			},
			req:     "/api/HI",
			matched: false,
			vars:    map[string]string{},
		},
		{
			registered: []string{"/api"},
			req:        "",
			matched:    false,
			vars:       map[string]string{},
		},
		{
			registered: []string{
				"/api/{name}/{path}*:action",
				"/api/{name}/{path}*",
			},
			req:       "/api/dog/wang/1:action",
			matched:   true,
			wantMatch: "/api/{name}/{path}*:action",
			vars: map[string]string{
				"name": "dog",
				"path": "wang/1",
			},
		},
		{
			registered: []string{
				"/api/{repository:(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*/?)+}*/manifests/{reference}",
				"/api/{repository}*/blobs/{digest:[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}}",
			},
			req:       "/api/lib/a/b/c/manifests/v1",
			matched:   true,
			wantMatch: "/api/{repository:(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*/?)+}*/manifests/{reference}",
			vars: map[string]string{
				"repository": "lib/a/b/c",
				"reference":  "v1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.req, func(t *testing.T) {
			m := NewMatcher[string]()
			for _, v := range tt.registered {
				if _, err := m.Register(v, v); err != nil {
					t.Error(err)
				}
			}
			matched, val, vars := m.Match(tt.req)
			if matched != tt.matched {
				t.Errorf("matcher.Match() matched = %v, want %v", matched, tt.matched)
			}

			if tt.wantMatch != "" && !reflect.DeepEqual(val, tt.wantMatch) {
				t.Errorf("matcher.Match() val = %v, want %v", val, tt.wantMatch)
			}

			if !reflect.DeepEqual(vars, tt.vars) {
				t.Errorf("matcher.Match() vars = %v, want %v", vars, tt.vars)
			}
		})
	}
}

func TestParsePathTokens(t *testing.T) {
	tests := []struct {
		path string

		want []string
	}{
		{
			path: "/apis/v1/abc",
			want: []string{"/apis", "/v1", "/abc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := PathTokens(tt.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePathTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompileSection(t *testing.T) {
	tests := []struct {
		name    string
		want    []Section
		wantErr bool
	}{
		{
			name: "/zoo/tom",
			want: []Section{
				{{Pattern: "/zoo"}},
				{{Pattern: "/tom"}},
			},
		},
		{
			name: "/v1/proxy*",
			want: []Section{
				{{Pattern: "/v1"}},
				{{Pattern: "/proxy", Greedy: true}},
			},
		},
		{
			name: "/api/v{version}/{name}*",
			want: []Section{
				{{Pattern: "/api"}},
				{{Pattern: "/v"}, {Pattern: "{version}", VarName: "version"}},
				{{Pattern: "/"}, {Pattern: "{name}", VarName: "name", Greedy: true}},
			},
		},
		{
			name: "/{repository:(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*/?)+}*/manifests/{reference}",
			want: []Section{
				{
					{Pattern: "/"},
					{
						Pattern: "{repository:(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*/?)+}", VarName: "repository", Greedy: true,
						Validate: regexp.MustCompile(`^(?:[a-zA-Z0-9]+(?:[._-][a-zA-Z0-9]+)*/?)+$`),
					},
					{Pattern: "/manifests"},
					{Pattern: "/"},
					{Pattern: "{reference}", VarName: "reference"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileSection(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Compile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSection_Match(t *testing.T) {
	type want struct {
		matched bool
		left    string
		vars    map[string]string
	}
	tests := []struct {
		pattern string
		tomatch string
		want    want
	}{
		{
			pattern: "pre{name}suf",
			tomatch: "prehellosuf",
			want:    want{matched: true, vars: map[string]string{"name": "hello"}},
		},
		{
			pattern: "pre{name}suf",
			tomatch: "presuf", // 假设 name 为空时，不被匹配
		},
		{
			pattern: "pre{name}",
			tomatch: "presuf/data",
			want:    want{matched: true, left: "/data", vars: map[string]string{"name": "suf"}},
		},
		{
			pattern: "pre*",
			tomatch: "prehellosuf/anything",
			want:    want{matched: true, vars: map[string]string{}},
		},
		{
			pattern: "pre{name}*",
			tomatch: "prehellosuf/anything",
			want:    want{matched: true, vars: map[string]string{"name": "hellosuf/anything"}},
		},
		{
			pattern: "pre",
			tomatch: "prehellosuf",
		},
		{
			pattern: "empty",
			tomatch: "",
		},
		{
			pattern: "apis",
			tomatch: "ap2is",
		},
		{
			pattern: "apis",
			tomatch: "api",
		},
		{
			pattern: "{a}{b}",
			tomatch: "tom",
			want:    want{matched: true, vars: map[string]string{"b": "tom"}},
		},
		{
			pattern: "{a}:{b}",
			tomatch: "tom:cat",
			want:    want{matched: true, vars: map[string]string{"a": "tom", "b": "cat"}},
		},
		{
			pattern: "{a}*:cat",
			tomatch: "tom:cat",
			want:    want{matched: true, vars: map[string]string{"a": "tom"}},
		},
		{
			pattern: "{repository}*/index/{name}",
			tomatch: "tom/z/index/cat",
			want:    want{matched: true, vars: map[string]string{"repository": "tom/z", "name": "cat"}},
		},
		{
			pattern: "{repository:([a-z]{3:64}/?)+}*/index/{name}",
			tomatch: "tom/Z/index/cat",
			want:    want{matched: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			compiled, err := Compile(tt.pattern)
			if err != nil {
				t.Error(err)
				return
			}
			matched, left, vars := compiled.Match(PathTokens(tt.tomatch))
			if matched != tt.want.matched {
				t.Errorf("MatchSection() matched = %v, want %v", matched, tt.want.matched)
			}
			if ((len(left) == 0) != (len(tt.want.left) == 0)) && !reflect.DeepEqual(left, tt.want.left) {
				t.Errorf("MatchSection() left = %v, want %v", left, tt.want.left)
			}
			if !reflect.DeepEqual(vars, tt.want.vars) {
				t.Errorf("MatchSection() vars = %v, want %v", vars, tt.want.vars)
			}
		})
	}
}

func TestSection_score(t *testing.T) {
	tests := []struct {
		a  string
		b  string
		eq int
	}{
		{a: "/a", b: "/{a}", eq: 1},
		{a: "api", b: "{a}", eq: 1},
		{a: "v{a}*", b: "{a}", eq: -1},
		{a: "{a}*", b: "{a}*:action", eq: -1},
	}
	for _, tt := range tests {
		t.Run(tt.a, func(t *testing.T) {
			seca, _ := Compile(tt.a)
			secb, _ := Compile(tt.b)

			scorea, scoreb := seca.score(), secb.score()
			if (scorea == scoreb && tt.eq != 0) ||
				(scorea > scoreb && tt.eq != 1) ||
				(scorea < scoreb && tt.eq != -1) {
				t.Errorf("Section.score() a = %v, b= %v, want %v", scorea, scoreb, tt.eq)
			}
		})
	}
}

func TestCompileError_Error(t *testing.T) {
	tests := []struct {
		name   string
		fields CompileError
		want   string
	}{
		{
			fields: CompileError{
				Pattern:  "pre{name}suf",
				Position: 1,
				Str:      "pre",
				Message:  "invalid character",
			},
			want: "invalid [pre] in [pre{name}suf] at position 1: invalid character",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fields
			if got := e.Error(); got != tt.want {
				t.Errorf("CompileError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

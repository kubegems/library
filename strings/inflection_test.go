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

package strings

import (
	"reflect"
	"testing"
)

func TestFirstWord(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "FirstWord", want: "First"},
		{name: "firstWord", want: "first"},
		{name: "first word", want: "first"},
		{name: "First Word", want: "First"},
		{name: "hi", want: "hi"},
		{name: "HELLO_WORLD", want: "HELLO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FirstWord(tt.name); got != tt.want {
				t.Errorf("FirstWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{name: "hello world", want: []string{"hello", "world"}},
		{name: "helloWorld", want: []string{"hello", "World"}},
		{name: "HELLO_WORLD", want: []string{"HELLO", "WORLD"}},
		{name: "HelloWorld", want: []string{"Hello", "World"}},
		{name: "hello-world", want: []string{"hello", "world"}},
		{name: "hello_world", want: []string{"hello", "world"}},
		{name: "hello.world", want: []string{"hello", "world"}},
		{name: "___hello______World", want: []string{"hello", "World"}},
		{name: " hello WORLD", want: []string{"hello", "WORLD"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitWords(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitWords() = %v, want %v", got, tt.want)
			}
		})
	}
}

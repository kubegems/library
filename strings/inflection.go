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
	"unicode"

	"github.com/jinzhu/inflection"
)

func ToPlural(name string) string {
	return inflection.Plural(name)
}

func ToSingular(name string) string {
	return inflection.Singular(name)
}

// SplitWords splits a string into words
// Words are separated by spaces, underscores, dashes, or dots
// e.g. "hello world" -> ["hello", "world"]
// e.g. "helloWorld" -> ["hello", "World"]
// e.g. "HELLO_WORLD" -> ["HELLO", "WORLD"]
func SplitWords(name string) []string {
	var words []string
	findByWords(name, func(s string) bool {
		words = append(words, s)
		return true
	})
	return words
}

func FirstWord(name string) string {
	findByWords(name, func(s string) bool {
		name = s
		return false
	})
	return name
}

func findByWords(name string, mapfunc func(string) bool) {
	pre, start := ' ', 0
	for i, r := range name {
		if unicode.IsSpace(r) || r == '_' || r == '-' || r == '.' {
			if i != start {
				if ok := mapfunc(name[start:i]); !ok {
					return
				}
			}
			start = i + 1
			continue
		}
		if unicode.IsLower(pre) && unicode.IsUpper(r) {
			if i != start {
				if ok := mapfunc(name[start:i]); !ok {
					return
				}
			}
			start = i
		}
		pre = r
	}
	if start != len(name) {
		if ok := mapfunc(name[start:]); !ok {
			return
		}
	}
}

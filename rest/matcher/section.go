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

package matcher

import (
	"fmt"
	"strings"
)

type ElementKind string

const (
	ElementKindNone     ElementKind = ""
	ElementKindConst    ElementKind = "const"
	ElementKindVariable ElementKind = "{}"
	ElementKindStar     ElementKind = "*"
)

type Element struct {
	Kind  ElementKind
	Param string
}

type CompileError struct {
	Pattern  string
	Position int
	Rune     rune
	Message  string
}

func (e CompileError) Error() string {
	return fmt.Sprintf("invalid char [%c] in [%s] at position %d: %s", e.Rune, e.Pattern, e.Position, e.Message)
}

func MustCompileSection(pattern string) Section {
	ret, err := CompileSection(pattern)
	if err != nil {
		panic(err)
	}
	return ret
}

type Section []Element

func CompileSection(pattern string) (Section, error) {
	elems := []Element{}
	pos := 0
	currentKind := ElementKindNone
	for i, rune := range pattern {
		switch {
		case rune == '{' && currentKind != ElementKindVariable:
			// end a const definition
			if currentKind == ElementKindConst {
				elems = append(elems, Element{Kind: ElementKindConst, Param: pattern[pos:i]})
			}
			// start a variable defination
			currentKind = ElementKindVariable
			pos = i + 1

		case rune == '}' && currentKind == ElementKindVariable:
			// end a variable defination
			elems = append(elems, Element{Kind: ElementKindVariable, Param: pattern[pos:i]})
			currentKind = ElementKindNone
			pos = i + 1
		case rune == '*':
			switch currentKind {
			case ElementKindVariable:
				continue
			case ElementKindConst:
				elems = append(elems, Element{Kind: ElementKindConst, Param: pattern[pos:i]})
				currentKind = ElementKindNone
			}
			// if previous is a star, ignore this star
			if len(elems) > 0 && elems[len(elems)-1].Kind != ElementKindStar {
				elems = append(elems, Element{Kind: ElementKindStar})
			}
			pos = i + 1
		default:
			// if in a variable difinarion or a const define
			if currentKind == ElementKindVariable || currentKind == ElementKindConst {
				continue
			}
			// start a const defination if not
			if currentKind != ElementKindConst {
				currentKind = ElementKindConst
				pos = i
			}
		}
	}
	switch currentKind {
	case ElementKindConst:
		// end a const definition
		params := pattern[pos:]
		if len(params) != 0 {
			elems = append(elems, Element{Kind: ElementKindConst, Param: pattern[pos:]})
		}
	case ElementKindVariable:
		return nil, CompileError{Position: len(pattern), Pattern: pattern, Rune: rune(pattern[len(pattern)-1]), Message: "variable defination not closed"}
	}
	return elems, nil
}

func (s Section) Match(tokens []string) (bool, bool, map[string]string) {
	vars := map[string]string{}
	if len(tokens) == 0 {
		return false, false, nil
	}
	matchedAll := false
	token := tokens[0]
	proc := Element{Kind: ElementKindNone}
	for _, elem := range s {
		switch elem.Kind {
		case ElementKindConst:
			// lastIndex or Index?
			index := strings.Index(token, elem.Param)
			if index == -1 {
				return false, false, nil
			}
			switch proc.Kind {
			case ElementKindVariable:
				// index == 0 means const is at the start of section
				// but prev is a variable, so it's not match prev
				if index == 0 {
					return false, false, nil
				}
				vars[proc.Param] = token[:index]
			}
			token = token[index+len(elem.Param):]
			proc = elem
		case ElementKindVariable:
			proc = elem
		case ElementKindStar:
			token = strings.Join(append([]string{token}, tokens[1:]...), "")
			tokens = nil
			matchedAll = true
			if proc.Kind != ElementKindVariable {
				proc = elem
			}
		}
	}
	// unclosed
	switch proc.Kind {
	case ElementKindVariable:
		vars[proc.Param] = token
		token = ""
	case ElementKindStar:
		token = ""
	}
	// section left some chars
	if token != "" {
		return false, false, nil
	}
	return true, matchedAll, vars
}

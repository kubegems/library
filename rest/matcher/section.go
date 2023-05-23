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

type elementKind string

const (
	elementKindNone     elementKind = ""
	elementKindConst    elementKind = "const"
	elementKindVariable elementKind = "{}"
	elementKindStar     elementKind = "*"
	elementKindSplit    elementKind = "/"
)

type element struct {
	kind  elementKind
	param string
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

func mustCompileSection(pattern string) []element {
	ret, err := compileSection(pattern)
	if err != nil {
		panic(err)
	}
	return ret
}

func compileSection(pattern string) ([]element, error) {
	elems := []element{}

	patternlen := len(pattern)
	hasStarSuffix := false
	if pattern[patternlen-1] == '*' {
		pattern = pattern[:patternlen-1]
		hasStarSuffix = true
	}

	pos := 0
	currentKind := elementKindNone
	for i, rune := range pattern {
		switch {
		case rune == '{' && currentKind != elementKindVariable:
			// end a const definition
			if currentKind == elementKindConst {
				elems = append(elems, element{kind: elementKindConst, param: pattern[pos:i]})
			}
			// start a variable defination
			currentKind = elementKindVariable
			pos = i + 1

		case rune == '}' && currentKind == elementKindVariable:
			// end a variable defination
			elems = append(elems, element{kind: elementKindVariable, param: pattern[pos:i]})
			currentKind = elementKindNone
			pos = i + 1
		default:
			// if in a variable difinarion or a const define
			if currentKind == elementKindVariable || currentKind == elementKindConst {
				continue
			}
			// start a const defination if not
			if currentKind != elementKindConst {
				currentKind = elementKindConst
				pos = i
			}
		}
	}
	// last
	switch currentKind {
	case elementKindConst:
		// end a const definition
		elems = append(elems, element{kind: elementKindConst, param: pattern[pos:]})
	case elementKindVariable:
		return nil, CompileError{Position: len(pattern), Pattern: pattern, Rune: rune(pattern[len(pattern)-1]), Message: "variable defination not closed"}
	}

	if hasStarSuffix {
		elems = append(elems, element{kind: elementKindStar})
	}
	return elems, nil
}

func matchSection(compiled []element, sections []string) (bool, bool, map[string]string) {
	vars := map[string]string{}
	if len(sections) == 0 {
		return false, false, nil
	}

	section := sections[0]

	pos := 0
	for i, elem := range compiled {
		switch elem.kind {
		case elementKindConst:
			conslen := len(elem.param)
			if len(section) < pos+conslen {
				return false, false, nil
			}
			str := section[pos : pos+conslen]
			if str != elem.param {
				return false, false, nil
			}
			// next section mactch
			pos += conslen
		case elementKindVariable:
			// no next
			if i == len(compiled)-1 {
				vars[elem.param] = section[pos:]
				return true, false, vars
			}
			// if next is const
			nextsec := compiled[i+1]
			switch nextsec.kind {
			case elementKindConst:
				index := strings.Index(section[pos:], nextsec.param)
				if index == -1 || index == 0 {
					// not match next const
					return false, false, nil
				}
				// var is bettwen pos and next sec start
				vars[elem.param] = section[pos : pos+index]
				pos += index
			case elementKindVariable:
				continue
			case elementKindStar:
				// var is bettwen pos to sections end
				vars[elem.param] = strings.Join(append([]string{section[pos:]}, sections[1:]...), "")
				return true, true, vars
			}
		case elementKindStar:
			return true, true, vars
		case elementKindSplit:
			if section == "/" {
				return true, false, vars
			}
			return false, false, nil
		}
	}
	// section left some chars
	// eg.  {kind:const,param:api} and apis; 's' remain
	if section[pos:] != "" {
		return false, false, nil
	}
	return true, false, vars
}

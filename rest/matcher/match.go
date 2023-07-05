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
	"fmt"
	"sort"
	"strings"
)

type PatternMatcher[T any] interface {
	// Register registers a pattern with a value and returns the variables in the pattern.
	Register(pattern string, val T) ([]string, error)
	// Match returns true if path matches a pattern, and the value registered with the pattern and the variables in the path.
	Match(path string) (bool, T, map[string]string)
}

var _ PatternMatcher[string] = &Node[string]{}

func NewMatcher[T any]() *Node[T] {
	return &Node[T]{}
}

type Node[T any] struct {
	key      Section
	val      *matchitem[T]
	children []*Node[T]
}

func (n *Node[T]) Register(pattern string, val T) ([]string, error) {
	sections, err := compilePathPattern(pattern)
	if err != nil {
		return nil, err
	}
	item := &matchitem[T]{pattern: pattern, val: val}

	cur := n
	for i, section := range sections {
		child := &Node[T]{key: section}
		if index := cur.indexChild(child); index == -1 {
			if i == len(sections)-1 {
				child.val = item
			}
			cur.children = append(cur.children, child)
			sortSectionMatches(cur.children)
		} else {
			child = cur.children[index]
			if i == len(sections)-1 {
				if child.val != nil {
					return nil, fmt.Errorf("pattern %s conflicts with exists %s", pattern, child.val.pattern)
				}
				child.val = item
			}
		}
		cur = child
	}
	return variables(sections), nil
}

func (n *Node[T]) Match(path string) (bool, T, map[string]string) {
	vars := map[string]string{}
	if match := matchchildren(n, parsePathTokens(path), vars); match == nil {
		return false, *new(T), vars
	} else {
		return true, match.val, vars
	}
}

func (n *Node[T]) indexChild(s *Node[T]) int {
	for index, child := range n.children {
		if isSamePattern(child.key, s.key) {
			return index
		}
	}
	return -1
}

func isSamePattern(a, b []Element) bool {
	tostr := func(elems []Element) string {
		str := ""
		for _, e := range elems {
			switch e.Kind {
			case ElementKindConst:
				str += e.Param
			case ElementKindVariable:
				str += "{}"
			case ElementKindStar:
				str += "*"
			}
		}
		return str
	}
	return tostr(a) == tostr(b)
}

func sortSectionMatches[T any](sections []*Node[T]) {
	sort.Slice(sections, func(i, j int) bool {
		secsi, secsj := sections[i].key, sections[j].key
		switch lasti, lastj := (secsi)[len(secsi)-1].Kind, (secsj)[len(secsj)-1].Kind; {
		case lasti == ElementKindStar && lastj != ElementKindStar:
			return false
		case lasti != ElementKindStar && lastj == ElementKindStar:
			return true
		}
		cnti, cntj := 0, 0
		for _, v := range secsi {
			switch v.Kind {
			case ElementKindConst:
				cnti += 99
			case ElementKindVariable:
				cnti -= 1
			}
		}
		for _, v := range secsj {
			switch v.Kind {
			case ElementKindConst:
				cntj += 99
			case ElementKindVariable:
				cntj -= 1
			}
		}
		return cnti > cntj
	})
}

type matchitem[T any] struct {
	pattern string
	val     T
}

func matchchildren[T any](cur *Node[T], tokens []string, vars map[string]string) *matchitem[T] {
	if len(tokens) == 0 {
		return nil
	}
	for _, child := range cur.children {
		if matched, matchlefttokens, secvars := child.key.Match(tokens); matched {
			if child.val != nil && len(tokens) == 1 || matchlefttokens {
				mergeMap(secvars, vars)
				return child.val
			}
			if result := matchchildren(child, tokens[1:], secvars); result != nil {
				mergeMap(secvars, vars)
				return result
			}
		}
	}
	return nil
}

func mergeMap(src, dst map[string]string) map[string]string {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func parsePathTokens(path string) []string {
	tokens := []string{}
	pos := 0
	for i, char := range path {
		if char == '/' {
			if pos != i {
				tokens = append(tokens, path[pos:i])
			}
			tokens = append(tokens, "/")
			pos = i + 1
		}
	}
	if pos != len(path) {
		tokens = append(tokens, path[pos:])
	}
	return tokens
}

func variables(sections []Section) []string {
	vars := []string{}
	for _, pattern := range sections {
		for _, elem := range pattern {
			if elem.Kind == ElementKindVariable {
				vars = append(vars, elem.Param)
			}
		}
	}
	return vars
}

func compilePathPattern(pattern string) ([]Section, error) {
	sections := []Section{}
	pathtokens := parsePathTokens(pattern)
	for i, token := range pathtokens {
		if token == "/" {
			sections = append(sections, []Element{{Kind: ElementKindConst, Param: "/"}})
			continue
		}
		if strings.HasSuffix(token, "*") {
			token = strings.Join(pathtokens[i:], "")
			compiled, err := CompileSection(token)
			if err != nil {
				return nil, err
			}
			sections = append(sections, compiled)
			break
		}
		compiled, err := CompileSection(token)
		if err != nil {
			return nil, err
		}
		sections = append(sections, compiled)
	}
	return sections, nil
}

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
	"regexp"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type PatternMatcher[T any] interface {
	// Register registers a pattern with a value and returns the variables in the pattern.
	Register(pattern string, val T) ([]string, error)
	// Match returns true if path matches a pattern, and the value registered with the pattern and the variables in the path.
	Match(path string) (bool, T, map[string]string)
}

var _ PatternMatcher[string] = &Tree[string]{}

func NewMatcher[T any]() *Tree[T] {
	return &Tree[T]{}
}

type Tree[T any] struct {
	key      Section
	val      *matchitem[T]
	children []*Tree[T]
}

type matchitem[T any] struct {
	pattern string
	val     T
}

func (n *Tree[T]) Match(path string) (bool, T, map[string]string) {
	vars := map[string]string{}
	if match := matchchildren(n, PathTokens(path), vars); match == nil {
		return false, *new(T), vars
	} else {
		return true, match.val, vars
	}
}

func matchchildren[T any](cur *Tree[T], tokens []string, vars map[string]string) *matchitem[T] {
	if len(tokens) == 0 {
		return nil
	}
	for _, child := range cur.children {
		if matched, lefttokens, secvars := child.key.Match(tokens); matched {
			maps.Copy(vars, secvars)
			// matched the last token and the child has a value
			if len(lefttokens) == 0 {
				return child.val
			}
			// continue matching
			if result := matchchildren(child, lefttokens, secvars); result != nil {
				maps.Copy(vars, secvars)
				return result
			}
		}
	}
	return nil
}

func PathTokens(path string) []string {
	tokens := []string{}
	pos := 0
	for i, char := range path {
		if char == '/' {
			if pos != i {
				tokens = append(tokens, path[pos:i])
			}
			pos = i
		}
	}
	if pos != len(path) {
		tokens = append(tokens, path[pos:])
	}
	return tokens
}

func (n *Tree[T]) Register(pattern string, val T) ([]string, error) {
	sections, err := CompileSection(pattern)
	if err != nil {
		return nil, err
	}
	cur := n
	for i, section := range sections {
		child := &Tree[T]{key: section}
		if index := cur.indexChild(child); index == -1 {
			cur.children = append(cur.children, child)
			// sort children by score, so that we can match the most likely child first
			slices.SortFunc(cur.children, func(a, b *Tree[T]) bool { return a.key.score() > b.key.score() })
		} else {
			child = cur.children[index]
		}
		if i == len(sections)-1 {
			if child.val != nil {
				return nil, fmt.Errorf("pattern %s conflicts with exists %s", pattern, child.val.pattern)
			}
			child.val = &matchitem[T]{pattern: pattern, val: val}
		}
		cur = child
	}
	return variables(sections), nil
}

func (n *Tree[T]) indexChild(child *Tree[T]) int {
	for index, exists := range n.children {
		if exists.key.String() == child.key.String() {
			return index
		}
	}
	return -1
}

func variables(sections []Section) []string {
	vars := []string{}
	for _, pattern := range sections {
		for _, elem := range pattern {
			if elem.VarName != "" {
				vars = append(vars, elem.VarName)
			}
		}
	}
	return vars
}

type Element struct {
	Pattern  string
	VarName  string
	Greedy   bool
	Validate *regexp.Regexp
}

type CompileError struct {
	Pattern  string
	Position int
	Str      string
	Message  string
}

func (e CompileError) Error() string {
	return fmt.Sprintf("invalid [%s] in [%s] at position %d: %s", e.Str, e.Pattern, e.Position, e.Message)
}

func CompileSection(patten string) ([]Section, error) {
	elems, err := Compile(patten)
	if err != nil {
		return nil, err
	}
	sections := []Section{}
	pre := 0
	for i, elem := range elems {
		if elem.VarName != "" && elem.Greedy {
			return append(sections, elems[pre:]), nil
		}
		if elem.VarName == "" && strings.HasPrefix(elem.Pattern, "/") {
			if i != pre {
				sections = append(sections, elems[pre:i])
			}
			pre = i
		}
	}
	if pre != len(elems) {
		sections = append(sections, elems[pre:])
	}
	return sections, nil
}

// Compile reads a variable name and a regular expression from a string.
func Compile(pattern string) (Section, error) {
	elems := []Element{}
	pre, curly := -1, 0
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++ // skip the next char
		case '{':
			// open a variable
			if curly == 0 {
				// close pre section
				if pre != -1 {
					elems = append(elems, Element{Pattern: pattern[pre:i]})
				}
				pre = i
			}
			curly++
		case '}':
			if curly == 1 {
				varname := pattern[pre+1 : i]
				// close a variable
				elem := Element{
					Pattern: pattern[pre : i+1],
					VarName: varname,
				}
				if idx := strings.IndexRune(elem.VarName, ':'); idx != -1 {
					name, regstr := elem.VarName[:idx], elem.VarName[idx+1:]
					elem.VarName = name
					if regstr != "" {
						regexp, err := regexp.Compile("^" + regstr + "$")
						if err != nil {
							return nil, CompileError{Pattern: pattern, Position: pre + 1 + idx + 1, Str: regstr, Message: err.Error()}
						}
						elem.Validate = regexp
					}
				}
				// check greedy
				if i < len(pattern)-1 && pattern[i+1] == '*' {
					elem.Greedy = true
					i++
				}
				elems = append(elems, elem)
				pre = -1
			}
			curly--
		case '/':
			if curly != 0 {
				continue
			}
			if pre != -1 {
				elems = append(elems, Element{Pattern: pattern[pre:i]})
			}
			pre = i
		default:
			// start const section
			if curly == 0 && pre == -1 {
				pre = i
			}
		}
	}
	// close the last const section
	if curly != 0 {
		return nil, CompileError{Pattern: pattern, Position: len(pattern) - 1, Message: "unclosed variable"}
	}
	if pre != -1 {
		elems = append(elems, Element{Pattern: pattern[pre:]})
	}
	// check const greedy, e.g. /abc*def
	for i := range elems {
		if elems[i].VarName == "" && elems[i].Pattern != "" {
			patten := elems[i].Pattern
			if patten[len(patten)-1] == '*' {
				elems[i].Greedy = true
				elems[i].Pattern = patten[:len(patten)-1]
			}
		}
	}
	return elems, nil
}

type Section []Element

func (s Section) String() string {
	patten := ""
	for _, elem := range s {
		patten += elem.Pattern
	}
	return patten
}

func (s Section) score() int {
	score := 0
	for _, v := range s {
		if v.Greedy {
			score += 1
		}
		if v.VarName != "" {
			score += 10
		} else {
			score += 100
		}
	}
	return score
}

func (s Section) Match(tokens []string) (bool, []string, map[string]string) {
	vars := map[string]string{}
	pre := Element{}
	if len(tokens) == 0 {
		return false, tokens, nil
	}
	token, lefttokens := tokens[0], tokens[1:]
	for _, elem := range s {
		if elem.Greedy {
			token, lefttokens = strings.Join(append([]string{token}, lefttokens...), ""), []string{}
		}
		if elem.VarName == "" {
			// lastIndex or Index?
			index := strings.Index(token, elem.Pattern)
			if index == -1 {
				return false, nil, nil
			}
			// end pre processing
			if pre.VarName != "" {
				varmatch := token[:index]
				if (varmatch == "" && pre.VarName != "") || (pre.Validate != nil && !pre.Validate.MatchString(varmatch)) {
					return false, nil, nil
				}
				vars[pre.VarName] = varmatch
			}
			token = token[index+len(elem.Pattern):]
		}
		pre = elem
	}
	// unclosed const greedy
	if pre.VarName == "" && pre.Greedy {
		token = ""
	}
	// unclosed variable
	if pre.VarName != "" {
		// regexp check
		if pre.Validate != nil && !pre.Validate.MatchString(token) {
			return false, nil, nil
		}
		vars[pre.VarName] = token
		token = ""
	}
	// section left some chars
	if token != "" {
		return false, nil, nil
	}
	return true, lefttokens, vars
}

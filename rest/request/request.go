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

package request

import (
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type PathVar struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type PathVarList []PathVar

func (p PathVarList) Get(key string) string {
	for _, v := range p {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

func (p PathVarList) Map() map[string]string {
	m := make(map[string]string, len(p))
	for _, v := range p {
		m[v.Key] = v.Value
	}
	return m
}

var PathVarsFunc = func(r *http.Request) PathVarList {
	return nil
}

type ListOptions struct {
	Page   int    `json:"page,omitempty"`
	Size   int    `json:"size,omitempty"`
	Search string `json:"search,omitempty"`
	Sort   string `json:"sort,omitempty"`
}

// nolint: gomnd
func GetListOptions(r *http.Request) ListOptions {
	return ListOptions{
		Page:   Query(r, "page", 1),
		Size:   Query(r, "size", 10),
		Search: Query(r, "search", ""),
		Sort:   Query(r, "sort", ""),
	}
}

func HeaderOrQuery[T any](r *http.Request, key string, defaultValue T) T {
	if val := r.Header.Get(key); val == "" {
		return ValueOrDefault(r.URL.Query().Get(key), defaultValue)
	} else {
		return ValueOrDefault(val, defaultValue)
	}
}

func PathVars(r *http.Request) PathVarList {
	return PathVarsFunc(r)
}

func Path[T any](r *http.Request, key string, defaultValue T) T {
	return ValueOrDefault(PathVars(r).Get(key), defaultValue)
}

func Header[T any](r *http.Request, key string, defaultValue T) T {
	val := r.Header.Get(key)
	return ValueOrDefault(val, defaultValue)
}

func Query[T any](r *http.Request, key string, defaultValue T) T {
	val := r.URL.Query().Get(key)
	return ValueOrDefault(val, defaultValue)
}

// nolint: forcetypeassert,gomnd,ifshort
// ValueOrDefault return default value if empty string
func ValueOrDefault[T any](val string, defaultValue T) T {
	if val == "" {
		return defaultValue
	}
	switch any(defaultValue).(type) {
	case string:
		return any(val).(T)
	case []string:
		if val == "" {
			return defaultValue
		}
		return any(strings.Split(val, ",")).(T)
	case int:
		intval, _ := strconv.Atoi(val)
		return any(intval).(T)
	case bool:
		b, _ := strconv.ParseBool(val)
		return any(b).(T)
	case int64:
		intval, _ := strconv.ParseInt(val, 10, 64)
		return any(intval).(T)
	case time.Time:
		t, _ := time.Parse(time.RFC3339, val)
		return any(t).(T)
	default:
		return defaultValue
	}
}

func Body(r *http.Request, into any) error {
	body := r.Body
	// check if the request body needs decompression
	switch contentEncoding := r.Header.Get("Content-Encoding"); contentEncoding {
	case "gzip":
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			return err
		}
		body = reader
	case "deflate":
		zlibReader, err := zlib.NewReader(r.Body)
		if err != nil {
			return err
		}
		body = zlibReader
	}

	mediatype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mediatype {
	case "application/json", "":
		if err := json.NewDecoder(body).Decode(into); err != nil {
			return err
		}
	case "application/xml":
		if err := xml.NewDecoder(body).Decode(into); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported media type: %s", mediatype)
	}
	return ValidateBody(r, into)
}

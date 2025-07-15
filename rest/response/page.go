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

package response

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubegems.io/library/rest/request"
)

const DefaultPageSize = 10

// ItemAccessor 封装了从泛型项目中提取名称、时间和pvc使用率的函数, 后续更多自定义字段拓展可以基于此结构体进行扩展
type ItemAccessor[T any] struct {
	GetName     func(item T) string
	GetTime     func(item T) time.Time
	GetPVCRatio func(item T) float64
}

// DefaultObjectAccessor 提供 Kubernetes 对象的默认访问器实现
var DefaultObjectAccessor = ItemAccessor[any]{
	GetName: func(t any) string {
		if item, ok := any(t).(interface{ GetName() string }); ok {
			return item.GetName()
		}
		if item, ok := any(&t).(interface{ GetName() string }); ok {
			return item.GetName()
		}
		return ""
	},
	GetTime: func(t any) time.Time {
		if item, ok := any(t).(interface{ GetCreationTimestamp() metav1.Time }); ok {
			return item.GetCreationTimestamp().Time
		}
		if item, ok := any(&t).(interface{ GetCreationTimestamp() metav1.Time }); ok {
			return item.GetCreationTimestamp().Time
		}
		return time.Time{}
	},
	GetPVCRatio: func(t any) float64 {
		var anno map[string]string
		if item, ok := any(t).(interface{ GetAnnotations() map[string]string }); ok {
			anno = item.GetAnnotations()
		}
		if item, ok := any(&t).(interface{ GetAnnotations() map[string]string }); ok {
			anno = item.GetAnnotations()
		}
		// storage.kubegems.io/pvc-ratio 是 Kubegems 特有的注解，用于表示 PVC 的使用率
		if ratio, ok := anno["storage.kubegems.io/pvc-ratio"]; ok {
			if f, err := strconv.ParseFloat(ratio, 64); err == nil {
				return f
			}
		}
		return 0
	},
}

// GetObjectAccessor 返回适用于指定类型的对象访问器
func GetObjectAccessor[T any]() ItemAccessor[T] {
	return ItemAccessor[T]{
		GetName: func(t T) string {
			return DefaultObjectAccessor.GetName(any(t))
		},
		GetTime: func(t T) time.Time {
			return DefaultObjectAccessor.GetTime(any(t))
		},
		GetPVCRatio: func(t T) float64 {
			return DefaultObjectAccessor.GetPVCRatio(any(t))
		},
	}
}

type Page[T any] struct {
	Total int64 `json:"total"`
	List  []T   `json:"list"`
	Page  int64 `json:"page"`
	Size  int64 `json:"size"`
}

func PageObjectFromRequest[T any](req *http.Request, list []T) Page[T] {
	return PageObjectFromListOptions(list, request.GetListOptions(req))
}

// PageObjectFromListOptions used for client.Object pagination T in list
// use any of T to suit for both eg. Pod(not implement metav1.Object) and *Pod(metav1.Object)
func PageObjectFromListOptions[T any](list []T, opts request.ListOptions) Page[T] {
	accessor := GetObjectAccessor[T]()
	return PageFromListOptionsWithAccessor(list, opts, accessor)
}

// PageFromRequest auto pagination from user request on item name or time in list
func PageFromRequest[T any](req *http.Request, list []T, namefunc func(item T) string, timefunc func(item T) time.Time) Page[T] {
	return PageFromListOptions(list, request.GetListOptions(req), namefunc, timefunc)
}

func PageFromListOptions[T any](list []T, opts request.ListOptions, namefunc func(item T) string, timefunc func(item T) time.Time) Page[T] {
	return PageFrom(list, opts.Page, opts.Size, SearchNameFunc(opts.Search, namefunc), SortByFunc(opts.Sort, namefunc, timefunc))
}

// PageFromRequestWithAccessor auto pagination from user request using ItemAccessor
func PageFromRequestWithAccessor[T any](req *http.Request, list []T, accessor ItemAccessor[T]) Page[T] {
	return PageFromListOptionsWithAccessor(list, request.GetListOptions(req), accessor)
}

// PageFromListOptionsWithAccessor 使用 ItemAccessor 进行分页处理
func PageFromListOptionsWithAccessor[T any](list []T, opts request.ListOptions, accessor ItemAccessor[T]) Page[T] {
	return PageFrom(list, opts.Page, opts.Size, SearchNameFuncWithAccessor(opts.Search, accessor), SortByFuncWithAccessor(opts.Sort, accessor))
}

func PageFrom[T any](list []T, page, size int, pickfun func(item T) bool, sortfun func(a, b T) int) Page[T] {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = DefaultPageSize
	}

	// filter
	if pickfun != nil {
		datas := []T{}
		for _, item := range list {
			if pickfun(item) {
				datas = append(datas, item)
			}
		}
		list = datas
	}

	// sort
	if sortfun != nil {
		slices.SortFunc(list, sortfun)
	}

	// page
	total := len(list)
	startIdx := (page - 1) * size
	endIdx := startIdx + size
	if startIdx > total {
		startIdx = 0
		endIdx = 0
	}
	if endIdx > total {
		endIdx = total
	}
	list = list[startIdx:endIdx]
	return Page[T]{
		Total: int64(total),
		List:  list,
		Page:  int64(page),
		Size:  int64(size),
	}
}

func SearchNameFunc[T any](search string, getname func(T) string) func(T) bool {
	if getname == nil || search == "" {
		return nil
	}
	return func(item T) bool {
		return strings.Contains(getname(item), search)
	}
}

func SortByFunc[T any](by string, getname func(T) string, gettime func(T) time.Time) func(a, b T) int {
	switch by {
	case "createTime", "createTimeAsc", "time":
		if gettime == nil {
			return nil
		}
		return func(a, b T) int {
			if timcmp := gettime(a).Compare(gettime(b)); timcmp == 0 && getname != nil {
				return strings.Compare(getname(a), getname(b))
			} else {
				return timcmp
			}
		}
	case "createTimeDesc", "time-", "": // default sort by time desc
		if gettime == nil {
			return nil
		}
		return func(a, b T) int {
			if timcmp := gettime(b).Compare(gettime(a)); timcmp == 0 && getname != nil {
				return strings.Compare(getname(a), getname(b))
			} else {
				return timcmp
			}
		}
	case "name":
		if getname == nil {
			return nil
		}
		return func(a, b T) int {
			return strings.Compare(getname(a), getname(b))
		}
	case "nameDesc", "name-":
		if getname == nil {
			return nil
		}
		return func(a, b T) int {
			return strings.Compare(getname(b), getname(a))
		}
	default:
		return nil
	}
}

// SearchNameFuncWithAccessor 使用 ItemAccessor 进行搜索过滤
func SearchNameFuncWithAccessor[T any](search string, accessor ItemAccessor[T]) func(T) bool {
	if accessor.GetName == nil || search == "" {
		return nil
	}
	return func(item T) bool {
		return strings.Contains(accessor.GetName(item), search)
	}
}

// SortByFuncWithAccessor 使用 ItemAccessor 进行排序
func SortByFuncWithAccessor[T any](by string, accessor ItemAccessor[T]) func(a, b T) int {
	switch by {
	case "createTime", "createTimeAsc", "time":
		if accessor.GetTime == nil {
			return nil
		}
		return func(a, b T) int {
			if timcmp := accessor.GetTime(a).Compare(accessor.GetTime(b)); timcmp == 0 && accessor.GetName != nil {
				return strings.Compare(accessor.GetName(a), accessor.GetName(b))
			} else {
				return timcmp
			}
		}
	case "createTimeDesc", "time-", "": // default sort by time desc
		if accessor.GetTime == nil {
			return nil
		}
		return func(a, b T) int {
			if timcmp := accessor.GetTime(b).Compare(accessor.GetTime(a)); timcmp == 0 && accessor.GetName != nil {
				return strings.Compare(accessor.GetName(b), accessor.GetName(a))
			} else {
				return timcmp
			}
		}
	case "name":
		if accessor.GetName == nil {
			return nil
		}
		return func(a, b T) int {
			return strings.Compare(accessor.GetName(a), accessor.GetName(b))
		}
	case "nameDesc", "name-":
		if accessor.GetName == nil {
			return nil
		}
		return func(a, b T) int {
			return strings.Compare(accessor.GetName(b), accessor.GetName(a))
		}
	case "ratioDesc", "ratio-":
		if accessor.GetPVCRatio == nil {
			return nil
		}
		return func(a, b T) int {
			if ratioA, ratioB := accessor.GetPVCRatio(a), accessor.GetPVCRatio(b); ratioA == ratioB && accessor.GetName != nil {
				return strings.Compare(accessor.GetName(a), accessor.GetName(b))
			} else if ratioA < ratioB {
				return 1
			} else if ratioA > ratioB {
				return -1
			}
			return 0
		}
	case "ratio", "ratioAsc":
		if accessor.GetPVCRatio == nil {
			return nil
		}
		return func(a, b T) int {
			if ratioA, ratioB := accessor.GetPVCRatio(a), accessor.GetPVCRatio(b); ratioA == ratioB && accessor.GetName != nil {
				return strings.Compare(accessor.GetName(a), accessor.GetName(b))
			} else if ratioA < ratioB {
				return -1
			} else if ratioA > ratioB {
				return 1
			}
			return 0
		}

	default:
		return nil
	}
}

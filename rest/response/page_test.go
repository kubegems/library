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
	"net/url"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestObj struct {
	Name        string
	CreateAt    metav1.Time
	Annotations map[string]string
}

func (t TestObj) GetName() string                   { return t.Name }
func (t TestObj) GetCreationTimestamp() metav1.Time { return t.CreateAt }
func (t TestObj) GetAnnotations() map[string]string { return t.Annotations }

// 辅助函数：创建测试对象列表
func createTestObjects(baseTime metav1.Time) []TestObj {
	return []TestObj{
		{
			Name:        "pod1",
			CreateAt:    metav1.Time{Time: baseTime.Time.Add(10 * time.Second)},
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.75"},
		},
		{
			Name:        "pod2",
			CreateAt:    metav1.Time{Time: baseTime.Time.Add(9 * time.Second)},
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.76"},
		},
		{
			Name:        "pod3",
			CreateAt:    metav1.Time{Time: baseTime.Time.Add(8 * time.Second)},
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.77"},
		},
		{
			Name:        "pod4",
			CreateAt:    metav1.Time{Time: baseTime.Time.Add(7 * time.Second)},
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.78"},
		},
	}
}

// 辅助函数：创建测试对象指针列表
func createTestObjectPointers(baseTime metav1.Time) []*TestObj {
	objects := createTestObjects(baseTime)
	pointers := make([]*TestObj, len(objects))
	for i := range objects {
		pointers[i] = &objects[i]
	}
	return pointers
}

// 辅助函数：从对象列表中提取名称
func extractNames[T any](objects []T, getName func(T) string) []string {
	names := make([]string, len(objects))
	for i, obj := range objects {
		names[i] = getName(obj)
	}
	return names
}

// 辅助函数：验证分页结果
func validatePageResult[T any](t *testing.T, testName string, got Page[T], expectedTotal int64, expectedPage int64, expectedSize int64, expectedNames []string, getName func(T) string) {
	t.Helper()

	gotNames := extractNames(got.List, getName)

	if got.Total != expectedTotal {
		t.Errorf("%s: Total = %d, want %d", testName, got.Total, expectedTotal)
	}
	if got.Page != expectedPage {
		t.Errorf("%s: Page = %d, want %d", testName, got.Page, expectedPage)
	}
	if got.Size != expectedSize {
		t.Errorf("%s: Size = %d, want %d", testName, got.Size, expectedSize)
	}
	if len(gotNames) != len(expectedNames) {
		t.Errorf("%s: got %d items, want %d items", testName, len(gotNames), len(expectedNames))
		return
	}
	for i, expected := range expectedNames {
		if i >= len(gotNames) || gotNames[i] != expected {
			t.Errorf("%s: item[%d] = %q, want %q (full list: got=%v, want=%v)",
				testName, i, gotNames[i], expected, gotNames, expectedNames)
			break
		}
	}
}

func TestPageObjectFromRequest(t *testing.T) {
	baseTime := metav1.Now()

	t.Run("empty list", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=10&sort=nameDesc"}}
		got := PageObjectFromRequest(req, []TestObj{})
		validatePageResult(t, "empty list", got, 0, 1, 10, []string{}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("basic pagination", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=2"}}
		got := PageObjectFromRequest(req, list)
		// 默认按时间降序排序，所以第一页应该是 pod1, pod2 (时间最新的)
		validatePageResult(t, "basic pagination", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("ratio desc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=2&sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		// 按 ratio 降序排序，第一页应该是 pod4(0.78), pod3(0.77)
		validatePageResult(t, "ratio desc sorting", got, 4, 1, 2, []string{"pod4", "pod3"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("search filter", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "search=pod3"}}
		got := PageObjectFromRequest(req, list)
		validatePageResult(t, "search filter", got, 1, 1, 10, []string{"pod3"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("pointer objects", func(t *testing.T) {
		list := createTestObjectPointers(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "search=pod2"}}
		got := PageObjectFromRequest(req, list)
		validatePageResult(t, "pointer objects", got, 1, 1, 10, []string{"pod2"}, func(obj *TestObj) string { return obj.Name })
	})

	t.Run("page exceeds total pages", func(t *testing.T) {
		list := createTestObjects(baseTime)
		// 总共4个对象，每页2个，总共2页，请求第3页
		req := &http.Request{URL: &url.URL{RawQuery: "page=3&size=2"}}
		got := PageObjectFromRequest(req, list)
		// 超出总页数时，应该返回空列表，但保持总数和分页信息
		validatePageResult(t, "page exceeds total pages", got, 4, 3, 2, []string{}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("page exceeds with size 1", func(t *testing.T) {
		list := createTestObjects(baseTime)
		// 总共4个对象，每页1个，总共4页，请求第5页
		req := &http.Request{URL: &url.URL{RawQuery: "page=5&size=1"}}
		got := PageObjectFromRequest(req, list)
		// 超出总页数时，应该返回空列表
		validatePageResult(t, "page exceeds with size 1", got, 4, 5, 1, []string{}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("debug search with accessor", func(t *testing.T) {
		list := createTestObjects(baseTime)
		accessor := GetObjectAccessor[TestObj]()

		// 验证 accessor.GetName 可以正确获取名称
		if name := accessor.GetName(list[0]); name == "" {
			t.Errorf("accessor.GetName returned empty string, expected %q", list[0].Name)
		} else if name != list[0].Name {
			t.Errorf("accessor.GetName = %q, want %q", name, list[0].Name)
		}

		// 验证搜索功能
		searchFunc := SearchNameFuncWithAccessor("pod1", accessor)
		if searchFunc == nil {
			t.Error("SearchNameFuncWithAccessor returned nil")
		} else {
			if !searchFunc(list[0]) {
				t.Error("searchFunc should return true for pod1")
			}
			if searchFunc(list[1]) {
				t.Error("searchFunc should return false for pod2")
			}
		}
	})
}

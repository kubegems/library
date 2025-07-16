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
	"fmt"
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

type TestObjPointer struct {
	Name        string
	CreateAt    metav1.Time
	Annotations map[string]string
}

func (t *TestObjPointer) GetName() string                   { return t.Name }
func (t *TestObjPointer) GetCreationTimestamp() metav1.Time { return t.CreateAt }
func (t *TestObjPointer) GetAnnotations() map[string]string { return t.Annotations }

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

// 辅助函数：创建测试对象指针列表（使用 TestObjPointer）
func createTestObjPointers(baseTime metav1.Time) []TestObjPointer {
	return []TestObjPointer{
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

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "empty list",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=10&sort=nameDesc"}}

				t.Run("value types", func(t *testing.T) {
					got := PageObjectFromRequest(req, []TestObj{})
					validatePageResult(t, "empty list - value types", got, 0, 1, 10, []string{}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					got := PageObjectFromRequest(req, []*TestObjPointer{})
					validatePageResult(t, "empty list - pointer types", got, 0, 1, 10, []string{}, func(obj *TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "basic pagination",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "basic pagination - value types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "basic pagination - pointer types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "ratio desc sorting",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=2&sort=ratioDesc"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "ratio desc sorting - value types", got, 4, 1, 2, []string{"pod4", "pod3"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "ratio desc sorting - pointer types", got, 4, 1, 2, []string{"pod4", "pod3"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "search filter",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search=pod3"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "search filter - value types", got, 1, 1, 10, []string{"pod3"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "search filter - pointer types", got, 1, 1, 10, []string{"pod3"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "page exceeds total pages",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=3&size=2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "page exceeds total pages - value types", got, 4, 3, 2, []string{}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "page exceeds total pages - pointer types", got, 4, 3, 2, []string{}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "page exceeds with size 1",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=5&size=1"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "page exceeds with size 1 - value types", got, 4, 5, 1, []string{}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "page exceeds with size 1 - pointer types", got, 4, 5, 1, []string{}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "debug search with accessor",
			run: func(t *testing.T) {
				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					accessor := GetObjectAccessor[TestObj]()

					if name := accessor.GetName(list[0]); name == "" {
						t.Errorf("accessor.GetName returned empty string, expected %q", list[0].Name)
					} else if name != list[0].Name {
						t.Errorf("accessor.GetName = %q, want %q", name, list[0].Name)
					}

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

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					accessor := GetObjectAccessor[TestObjPointer]()

					if name := accessor.GetName(list[0]); name == "" {
						t.Errorf("accessor.GetName returned empty string, expected %q", list[0].Name)
					} else if name != list[0].Name {
						t.Errorf("accessor.GetName = %q, want %q", name, list[0].Name)
					}

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
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}

// 测试更多排序场景
func TestSortingScenarios(t *testing.T) {
	baseTime := metav1.Now()

	t.Run("name asc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "sort=name"}}
		got := PageObjectFromRequest(req, list)
		// 按名称升序排序：pod1, pod2, pod3, pod4
		validatePageResult(t, "name asc sorting", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("name desc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "sort=nameDesc"}}
		got := PageObjectFromRequest(req, list)
		// 按名称降序排序：pod4, pod3, pod2, pod1
		validatePageResult(t, "name desc sorting", got, 4, 1, 10, []string{"pod4", "pod3", "pod2", "pod1"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("time asc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "sort=createTimeAsc"}}
		got := PageObjectFromRequest(req, list)
		// 按时间升序排序：pod4, pod3, pod2, pod1 (时间最早的先)
		validatePageResult(t, "time asc sorting", got, 4, 1, 10, []string{"pod4", "pod3", "pod2", "pod1"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("ratio asc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "sort=ratio"}}
		got := PageObjectFromRequest(req, list)
		// 按 ratio 升序排序：pod1(0.75), pod2(0.76), pod3(0.77), pod4(0.78)
		validatePageResult(t, "ratio asc sorting", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
	})
	t.Run("ratio desc sorting", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{RawQuery: "sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		// 按 ratio 降序排序：pod4(0.78), pod3(0.77), pod2(0.76), pod1(0.75)
		validatePageResult(t, "ratio desc sorting", got, 4, 1, 10, []string{"pod4", "pod3", "pod2", "pod1"}, func(obj TestObj) string { return obj.Name })
	})
}

// 测试分页边界情况
func TestPaginationBoundaries(t *testing.T) {
	baseTime := metav1.Now()

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "zero page",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=0&size=2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "zero page - value types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "zero page - pointer types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "negative page",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=-1&size=2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "negative page - value types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "negative page - pointer types", got, 4, 1, 2, []string{"pod1", "pod2"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "zero size",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=1&size=0"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "zero size - value types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "zero size - pointer types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "second page",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "page=2&size=2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "second page - value types", got, 4, 2, 2, []string{"pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "second page - pointer types", got, 4, 2, 2, []string{"pod3", "pod4"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}

// 测试搜索功能
func TestSearchFunctionality(t *testing.T) {
	baseTime := metav1.Now()

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "empty search",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search="}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "empty search - value types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "empty search - pointer types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "partial name search",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search=pod"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "partial name search - value types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "partial name search - pointer types", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "specific name search",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search=pod2"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "specific name search - value types", got, 1, 1, 10, []string{"pod2"}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "specific name search - pointer types", got, 1, 1, 10, []string{"pod2"}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "no match search",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search=notfound"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "no match search - value types", got, 0, 1, 10, []string{}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "no match search - pointer types", got, 0, 1, 10, []string{}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
		{
			name: "case sensitive search",
			run: func(t *testing.T) {
				req := &http.Request{URL: &url.URL{RawQuery: "search=POD1"}}

				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "case sensitive search - value types", got, 0, 1, 10, []string{}, func(obj TestObj) string { return obj.Name })
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					got := PageObjectFromRequest(req, list)
					validatePageResult(t, "case sensitive search - pointer types", got, 0, 1, 10, []string{}, func(obj TestObjPointer) string { return obj.Name })
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}

// 测试 accessor 功能
func TestAccessorFunctionality(t *testing.T) {
	baseTime := metav1.Now()

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "accessor functionality",
			run: func(t *testing.T) {
				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					accessor := GetObjectAccessor[TestObj]()

					if name := accessor.GetName(list[0]); name != "pod1" {
						t.Errorf("GetName = %q, want %q", name, "pod1")
					}

					if gotTime := accessor.GetTime(list[0]); !gotTime.Equal(list[0].CreateAt.Time) {
						t.Errorf("GetTime = %v, want %v", gotTime, list[0].CreateAt.Time)
					}

					if ratio := accessor.GetPVCRatio(list[0]); ratio != 0.75 {
						t.Errorf("GetPVCRatio = %f, want %f", ratio, 0.75)
					}
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					accessor := GetObjectAccessor[TestObjPointer]()

					if name := accessor.GetName(list[0]); name != "pod1" {
						t.Errorf("GetName = %q, want %q", name, "pod1")
					}

					if gotTime := accessor.GetTime(list[0]); !gotTime.Equal(list[0].CreateAt.Time) {
						t.Errorf("GetTime = %v, want %v", gotTime, list[0].CreateAt.Time)
					}

					if ratio := accessor.GetPVCRatio(list[0]); ratio != 0.75 {
						t.Errorf("GetPVCRatio = %f, want %f", ratio, 0.75)
					}
				})
			},
		},
		{
			name: "search function with accessor",
			run: func(t *testing.T) {
				t.Run("value types", func(t *testing.T) {
					list := createTestObjects(baseTime)
					accessor := GetObjectAccessor[TestObj]()

					searchFunc := SearchNameFuncWithAccessor("", accessor)
					if searchFunc != nil {
						t.Error("SearchNameFuncWithAccessor should return nil for empty search")
					}

					searchFunc = SearchNameFuncWithAccessor("pod1", accessor)
					if searchFunc == nil {
						t.Error("SearchNameFuncWithAccessor should not return nil for valid search")
					} else {
						if !searchFunc(list[0]) {
							t.Error("searchFunc should return true for matching item")
						}
						if searchFunc(list[1]) {
							t.Error("searchFunc should return false for non-matching item")
						}
					}
				})

				t.Run("pointer types", func(t *testing.T) {
					list := createTestObjPointers(baseTime)
					accessor := GetObjectAccessor[TestObjPointer]()

					searchFunc := SearchNameFuncWithAccessor("", accessor)
					if searchFunc != nil {
						t.Error("SearchNameFuncWithAccessor should return nil for empty search")
					}

					searchFunc = SearchNameFuncWithAccessor("pod1", accessor)
					if searchFunc == nil {
						t.Error("SearchNameFuncWithAccessor should not return nil for valid search")
					} else {
						if !searchFunc(list[0]) {
							t.Error("searchFunc should return true for matching item")
						}
						if searchFunc(list[1]) {
							t.Error("searchFunc should return false for non-matching item")
						}
					}
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.run)
	}
}

func TestInvalidDataCases(t *testing.T) {
	baseTime := metav1.Now()

	t.Run("empty annotations", func(t *testing.T) {
		list := []TestObj{
			{
				Name:        "pod1",
				CreateAt:    baseTime,
				Annotations: nil,
			},
			{
				Name:        "pod2",
				CreateAt:    baseTime,
				Annotations: map[string]string{},
			},
		}
		req := &http.Request{URL: &url.URL{RawQuery: "sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		validatePageResult(t, "empty annotations", got, 2, 1, 10, []string{"pod1", "pod2"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("invalid ratio annotation", func(t *testing.T) {
		list := []TestObj{
			{
				Name:        "pod1",
				CreateAt:    baseTime,
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "invalid"},
			},
			{
				Name:        "pod2",
				CreateAt:    baseTime,
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.5"},
			},
		}
		req := &http.Request{URL: &url.URL{RawQuery: "sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		validatePageResult(t, "invalid ratio annotation", got, 2, 1, 10, []string{"pod2", "pod1"}, func(obj TestObj) string { return obj.Name })
	})
}

// 测试组合功能
func TestCombinedFunctionality(t *testing.T) {
	baseTime := metav1.Now()

	t.Run("search and sort", func(t *testing.T) {
		// 创建更多测试数据
		list := []TestObj{
			{
				Name:        "app1-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(10 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.80"},
			},
			{
				Name:        "app2-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(9 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.85"},
			},
			{
				Name:        "db-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(8 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.90"},
			},
			{
				Name:        "cache-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(7 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.95"},
			},
		}
		req := &http.Request{URL: &url.URL{RawQuery: "search=app&sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		// 搜索 "app" 并按 ratio 降序排序，应该是 app2-pod, app1-pod
		validatePageResult(t, "search and sort", got, 2, 1, 10, []string{"app2-pod", "app1-pod"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("search and sort combined", func(t *testing.T) {
		// 创建更多测试数据
		list := []TestObj{
			{
				Name:        "app1-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(10 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.8"},
			},
			{
				Name:        "app2-pod",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(5 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.9"},
			},
			{
				Name:        "app1-service",
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(15 * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.7"},
			},
		}
		req := &http.Request{URL: &url.URL{RawQuery: "search=app1&sort=ratioDesc"}}
		got := PageObjectFromRequest(req, list)
		// 搜索 "app1" 应该匹配 app1-pod 和 app1-service，按 ratio 降序排序
		validatePageResult(t, "search and sort combined", got, 2, 1, 10, []string{"app1-pod", "app1-service"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("search with pagination", func(t *testing.T) {
		// 创建6个测试项目
		list := make([]TestObj, 6)
		for i := 0; i < 6; i++ {
			list[i] = TestObj{
				Name:        fmt.Sprintf("test-pod-%d", i+1),
				CreateAt:    metav1.Time{Time: baseTime.Time.Add(time.Duration(i) * time.Second)},
				Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.5"},
			}
		}
		req := &http.Request{URL: &url.URL{RawQuery: "search=test&page=2&size=3"}}
		got := PageObjectFromRequest(req, list)
		// 搜索 "test" 匹配所有6个，默认按时间降序排序，第2页每页3个应该是中间3个
		// 时间降序：test-pod-6, test-pod-5, test-pod-4, test-pod-3, test-pod-2, test-pod-1
		// 第1页：test-pod-6, test-pod-5, test-pod-4
		// 第2页：test-pod-3, test-pod-2, test-pod-1
		validatePageResult(t, "search with pagination", got, 6, 2, 3, []string{"test-pod-3", "test-pod-2", "test-pod-1"}, func(obj TestObj) string { return obj.Name })
	})

	t.Run("no query parameters", func(t *testing.T) {
		list := createTestObjects(baseTime)
		req := &http.Request{URL: &url.URL{}}
		got := PageObjectFromRequest(req, list)
		validatePageResult(t, "no query parameters", got, 4, 1, 10, []string{"pod1", "pod2", "pod3", "pod4"}, func(obj TestObj) string { return obj.Name })
	})
}

// 调试搜索功能的测试
func TestDebugSearch(t *testing.T) {
	baseTime := metav1.Now()

	list := []TestObj{
		{
			Name:        "app1-pod",
			CreateAt:    baseTime,
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.8"},
		},
		{
			Name:        "app2-pod",
			CreateAt:    baseTime,
			Annotations: map[string]string{"storage.kubegems.io/pvc-ratio": "0.9"},
		},
	}

	// 测试搜索功能
	req := &http.Request{URL: &url.URL{RawQuery: "search=app1"}}
	got := PageObjectFromRequest(req, list)

	t.Logf("Debug: search=app1, got %d items: %v", len(got.List), extractNames(got.List, func(obj TestObj) string { return obj.Name }))
	t.Logf("Debug: Total=%d, expected 1", got.Total)

	if got.Total != 1 {
		t.Errorf("Expected 1 item matching 'app1', got %d", got.Total)
	}
}

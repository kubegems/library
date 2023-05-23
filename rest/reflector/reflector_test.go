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

package reflector

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
)

type SampleRequest struct {
	Name    string `json:"name"`
	Parent  string `json:"parent"`
	Config  any    `json:"config"`
	Options any    `json:"options"`
}

type ZooController struct{}

type ListOptions struct {
	Page   int    `json:"page"`
	Limit  int    `json:"limit"`
	Filter string `json:"filter"`
	Sort   string `json:"sort"`
}

func (c *ZooController) GetZooAnimal(ctx context.Context, zoo string, animal string) (string, error) {
	return fmt.Sprintf("hi %s", animal), nil
}

func (c *ZooController) ListZooAnimal(ctx context.Context, zoo string, queries ListOptions) (any, error) {
	return nil, nil
}

func (c *ZooController) DropAnimal(ctx context.Context, req any) (any, error) {
	return nil, nil
}

func (c *ZooController) CreateZoo(ctx context.Context, req any) (any, error) {
	return nil, nil
}

func TestRegisterController(t *testing.T) {
	controller := &ZooController{}
	got, err := RegisterController("v1", nil, controller)
	if err != nil {
		t.Errorf("RegisterController() error = %v", err)
		return
	}
	t.Logf("RegisterController() = %v", got)

	req := httptest.NewRequest("GET", "/v1/zoo/animal/tom", nil)
	resp := httptest.NewRecorder()

	got[0].Handler.ServeHTTP(resp, req)
}

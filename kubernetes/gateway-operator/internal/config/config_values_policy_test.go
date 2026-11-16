/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/valuespolicy"
)

func TestValuesPolicyDefaultsToEnabled(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Gateway.ValuesPolicy.Enabled {
		t.Fatal("values policy must be enabled by default")
	}
	if !reflect.DeepEqual(cfg.Gateway.ValuesPolicy.AllowedImageRegistries, valuespolicy.DefaultAllowedImageRegistries) {
		t.Fatalf("unexpected default registries %v", cfg.Gateway.ValuesPolicy.AllowedImageRegistries)
	}
}

func TestValuesPolicyFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "gateway:\n  values_policy:\n    enabled: false\n    allowed_image_registries:\n      - mirror.internal/platform\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.ValuesPolicy.Enabled {
		t.Fatal("expected the file to disable the policy")
	}
	if want := []string{"mirror.internal/platform"}; !reflect.DeepEqual(cfg.Gateway.ValuesPolicy.AllowedImageRegistries, want) {
		t.Fatalf("got %v, want %v", cfg.Gateway.ValuesPolicy.AllowedImageRegistries, want)
	}
}

func TestValuesPolicyFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_VALUES_POLICY_ENABLED", "false")
	t.Setenv("GATEWAY_VALUES_POLICY_ALLOWED_IMAGE_REGISTRIES", "a.example/x, b.example/y")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gateway.ValuesPolicy.Enabled {
		t.Fatal("expected the environment to disable the policy")
	}
	if want := []string{"a.example/x", "b.example/y"}; !reflect.DeepEqual(cfg.Gateway.ValuesPolicy.AllowedImageRegistries, want) {
		t.Fatalf("got %v, want %v", cfg.Gateway.ValuesPolicy.AllowedImageRegistries, want)
	}
}

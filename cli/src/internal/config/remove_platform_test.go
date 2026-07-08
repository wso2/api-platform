/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"strings"
	"testing"
)

func TestRemovePlatform_DeletesPlatform(t *testing.T) {
	cfg := &Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*Platform{
			"eu":      {},
			"default": {},
		},
	}

	if err := cfg.RemovePlatform("eu"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.Platforms["eu"]; ok {
		t.Fatalf("expected platform 'eu' to be removed")
	}
	// Removing the current platform resets the selection to the default.
	if got := cfg.GetCurrentPlatform(); got != DefaultPlatform {
		t.Fatalf("expected current platform to reset to %q, got %q", DefaultPlatform, got)
	}
}

func TestRemovePlatform_KeepsCurrentWhenOtherRemoved(t *testing.T) {
	cfg := &Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*Platform{
			"eu": {},
			"us": {},
		},
	}

	if err := cfg.RemovePlatform("us"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GetCurrentPlatform() != "eu" {
		t.Fatalf("expected current platform to stay 'eu', got %q", cfg.GetCurrentPlatform())
	}
}

func TestRemovePlatform_NotFound(t *testing.T) {
	cfg := &Config{Platforms: map[string]*Platform{"default": {}}}

	err := cfg.RemovePlatform("missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

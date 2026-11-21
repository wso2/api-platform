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
	"testing"
	"time"
)

func TestDeletionSettingsDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Gateway.DeletionGracePeriod != 0 {
		t.Errorf("DeletionGracePeriod = %v, want 0", cfg.Gateway.DeletionGracePeriod)
	}
	if cfg.Gateway.RetainVolumeOnDelete {
		t.Error("RetainVolumeOnDelete = true, want false")
	}
}

func TestDeletionSettingsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "gateway:\n  deletion_grace_period: \"30m\"\n  retain_volume_on_delete: true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Gateway.DeletionGracePeriod != 30*time.Minute {
		t.Errorf("DeletionGracePeriod = %v, want 30m", cfg.Gateway.DeletionGracePeriod)
	}
	if !cfg.Gateway.RetainVolumeOnDelete {
		t.Error("RetainVolumeOnDelete = false, want true")
	}
}

func TestDeletionSettingsFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_DELETION_GRACE_PERIOD", "2h")
	t.Setenv("GATEWAY_RETAIN_VOLUME_ON_DELETE", "true")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Gateway.DeletionGracePeriod != 2*time.Hour {
		t.Errorf("DeletionGracePeriod = %v, want 2h", cfg.Gateway.DeletionGracePeriod)
	}
	if !cfg.Gateway.RetainVolumeOnDelete {
		t.Error("RetainVolumeOnDelete = false, want true")
	}
}

func TestNegativeDeletionGracePeriodRejected(t *testing.T) {
	t.Setenv("GATEWAY_DELETION_GRACE_PERIOD", "-1m")
	if _, err := LoadConfig(""); err == nil {
		t.Fatal("LoadConfig accepted a negative deletion grace period")
	}
}

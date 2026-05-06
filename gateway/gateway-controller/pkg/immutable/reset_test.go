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

package immutable

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestResetSQLiteFiles_RemovesAllSuffixes(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "gateway.db")

	suffixes := []string{"", "-wal", "-shm", "-journal"}
	for _, s := range suffixes {
		if err := os.WriteFile(base+s, []byte("junk"), 0600); err != nil {
			t.Fatalf("setup: write %s: %v", base+s, err)
		}
	}

	if err := ResetSQLiteFiles(base, slog.Default()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, s := range suffixes {
		if _, err := os.Stat(base + s); !os.IsNotExist(err) {
			t.Errorf("expected %q to be removed, but it still exists", base+s)
		}
	}
}

func TestResetSQLiteFiles_MissingFilesSucceeds(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "nonexistent.db")

	if err := ResetSQLiteFiles(base, slog.Default()); err != nil {
		t.Fatalf("expected success for missing files, got: %v", err)
	}
}

func TestResetSQLiteFiles_Idempotent(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "gateway.db")

	if err := os.WriteFile(base, []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	log := slog.Default()
	if err := ResetSQLiteFiles(base, log); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := ResetSQLiteFiles(base, log); err != nil {
		t.Fatalf("second call (idempotent): %v", err)
	}
}

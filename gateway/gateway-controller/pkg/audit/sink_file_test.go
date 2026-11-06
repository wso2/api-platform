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

package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

func newTestFileSink(t *testing.T, path string, maxSizeMB int) (*fileSink, *logBuffer) {
	t.Helper()
	logger, logs := newTestLogger()
	s, err := newFileSink(config.AuditFileConfig{Path: path, MaxSizeMB: maxSizeMB}, logger)
	if err != nil {
		t.Fatalf("newFileSink error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s, logs
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func TestFileSinkCreatesAPrivateFileAndDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "audit", "audit.log")
	s, _ := newTestFileSink(t, path, 100)

	s.Write([]byte(`{"id":"1"}`))
	s.Write([]byte(`{"id":"2"}`))

	if got := readLines(t, path); strings.Join(got, ",") != `{"id":"1"},{"id":"2"}` {
		t.Fatalf("file lines = %q", got)
	}
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %#o, want %#o", p, got, want)
		}
	}
	if s.Name() != "file" {
		t.Fatalf("Name() = %q, want file", s.Name())
	}
}

func TestFileSinkAppendsToAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	if err := os.WriteFile(path, []byte("{\"id\":\"0\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := newTestFileSink(t, path, 100)

	s.Write([]byte(`{"id":"1"}`))

	if got := readLines(t, path); len(got) != 2 {
		t.Fatalf("file lines = %q, want the existing line and the new one", got)
	}
	if s.size != int64(len("{\"id\":\"0\"}\n{\"id\":\"1\"}\n")) {
		t.Fatalf("size = %d, want the whole file counted", s.size)
	}
}

func TestFileSinkRotatesAtMaxSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, _ := newTestFileSink(t, path, 1)
	s.maxBytes = 40 // three 11-byte lines fit, the fourth rotates

	for i := range 10 {
		s.Write(fmt.Appendf(nil, `{"id":"%d"}`, i))
	}

	// Rotated at records 3, 6 and 9; each rotation replaces the previous backup.
	if got := readLines(t, path+".1"); strings.Join(got, ",") != `{"id":"6"},{"id":"7"},{"id":"8"}` {
		t.Fatalf("backup lines = %q", got)
	}
	if got := readLines(t, path); strings.Join(got, ",") != `{"id":"9"}` {
		t.Fatalf("live lines = %q", got)
	}
	for _, p := range []string{path, path + ".1"} {
		if info, err := os.Stat(p); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v, mode %v; want 0600", p, err, info.Mode().Perm())
		}
	}
}

func TestFileSinkKeepsALineLargerThanMaxSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, _ := newTestFileSink(t, path, 1)
	s.maxBytes = 5

	s.Write([]byte(`{"id":"larger-than-the-limit"}`))

	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("an empty file was rotated for an oversized line: %v", err)
	}
	if got := readLines(t, path); len(got) != 1 {
		t.Fatalf("live lines = %q, want the oversized line", got)
	}
}

func TestFileSinkWithoutMaxSizeNeverRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, _ := newTestFileSink(t, path, 0)

	for i := range 100 {
		s.Write(fmt.Appendf(nil, `{"id":"%d"}`, i))
	}

	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("rotated although max_size_mb is 0: %v", err)
	}
	if got := readLines(t, path); len(got) != 100 {
		t.Fatalf("live lines = %d, want 100", len(got))
	}
}

func TestNewFileSinkRefusesAnUnsafePath(t *testing.T) {
	dir := t.TempDir()
	readable := filepath.Join(dir, "readable.log")
	if err := os.WriteFile(readable, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readable, 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.log")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.log")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	notAFile := filepath.Join(dir, "subdir")
	if err := os.Mkdir(notAFile, 0o700); err != nil {
		t.Fatal(err)
	}
	readOnly := filepath.Join(dir, "read-only.log")
	if err := os.WriteFile(readOnly, nil, 0o400); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		cfg     config.AuditFileConfig
		wantErr string
	}{
		{"readable by others", config.AuditFileConfig{Path: readable}, "allow group/other access"},
		{"symlink", config.AuditFileConfig{Path: link}, "is not a regular file"},
		{"directory", config.AuditFileConfig{Path: notAFile}, "is not a regular file"},
		{"parent is a file", config.AuditFileConfig{Path: filepath.Join(target, "audit.log")}, "not a directory"},
		{"not writable", config.AuditFileConfig{Path: readOnly}, "permission denied"},
		{"relative path", config.AuditFileConfig{Path: "audit.log"}, "path must be absolute"},
		{"negative max_size_mb", config.AuditFileConfig{Path: filepath.Join(dir, "a.log"), MaxSizeMB: -1}, "max_size_mb must be >= 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantErr == "permission denied" && os.Geteuid() == 0 {
				t.Skip("root ignores file permissions")
			}
			logger, _ := newTestLogger()
			s, err := newFileSink(tc.cfg, logger)
			if err == nil {
				_ = s.Close(context.Background())
				t.Fatal("newFileSink succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("newFileSink error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestNewFileSinkWarnsAboutAnOpenDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, logs := newTestFileSink(t, filepath.Join(dir, "audit.log"), 100)

	if !strings.Contains(logs.String(), "Audit log directory is group/other accessible") {
		t.Fatalf("no warning for a 0755 directory:\n%s", logs)
	}
}

func TestFileSinkWritesUnrotatedWhenTheRenameFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "logs")
	path := filepath.Join(dir, "audit.log")
	s, logs := newTestFileSink(t, path, 1)
	s.maxBytes = 15
	s.Write([]byte(`{"id":"1"}`))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	s.Write([]byte(`{"id":"2"}`))

	if !strings.Contains(logs.String(), "writing to it unrotated") {
		t.Fatalf("the failed rotation was not logged:\n%s", logs)
	}
	if got := readLines(t, path); len(got) != 2 {
		t.Fatalf("live lines = %q, want both lines", got)
	}
}

func TestFileSinkReopensAfterTheFileWasLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, logs := newTestFileSink(t, path, 100)
	s.Write([]byte(`{"id":"1"}`))

	// As after a rotation that could not open a new file.
	_ = s.f.Close()
	s.f = nil
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	s.Write([]byte(`{"id":"2"}`))
	if !strings.Contains(logs.String(), "Audit log file is not open; dropping the record") {
		t.Fatalf("the dropped record was not logged:\n%s", logs)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.Write([]byte(`{"id":"3"}`))
	if got := readLines(t, path); strings.Join(got, ",") != `{"id":"3"}` {
		t.Fatalf("live lines = %q, want writing to resume", got)
	}
}

func TestFileSinkLogsAFailedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, logs := newTestFileSink(t, path, 100)
	readOnly, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.f.Close()
	s.f = readOnly

	s.Write([]byte(`{"id":"1"}`))

	if !strings.Contains(logs.String(), "Failed to write audit record to the file; dropping it") {
		t.Fatalf("the failed write was not logged:\n%s", logs)
	}
}

func TestFileSinkClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	s, _ := newTestFileSink(t, path, 100)

	s.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Close(ctx); err == nil || !strings.Contains(err.Error(), "did not close") {
		t.Fatalf("Close with a stuck write = %v, want a timeout error", err)
	}
	s.mu.Unlock()

	waitFor(t, "the close to finish", func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.f == nil
	})
	s.Write([]byte(`{"id":"1"}`))
	if got := readLines(t, path); len(got) != 0 {
		t.Fatalf("written after Close: %q", got)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("second Close error = %v", err)
	}
}

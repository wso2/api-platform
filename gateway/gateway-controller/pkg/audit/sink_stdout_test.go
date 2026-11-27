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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replaceStdout points os.Stdout at f for the rest of the test.
func replaceStdout(t *testing.T, f *os.File) {
	t.Helper()
	orig := os.Stdout
	os.Stdout = f
	t.Cleanup(func() { os.Stdout = orig })
}

func TestStdoutSinkWritesOneLinePerRecord(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	replaceStdout(t, w)
	logger, _ := newTestLogger()
	s := newStdoutSink(logger)

	s.Write([]byte(`{"id":"1"}`))
	s.Write([]byte(`{"id":"2"}`))
	if err := s.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if want := "{\"id\":\"1\"}\n{\"id\":\"2\"}\n"; string(out) != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
	if s.Name() != "stdout" {
		t.Fatalf("Name() = %q, want stdout", s.Name())
	}
}

func TestStdoutSinkLogsAFailedWrite(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "closed"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	replaceStdout(t, f)
	logger, logs := newTestLogger()

	newStdoutSink(logger).Write([]byte(`{"id":"1"}`))

	if !strings.Contains(logs.String(), "Failed to write audit record; dropping it") {
		t.Fatalf("a failed write was not logged:\n%s", logs)
	}
}

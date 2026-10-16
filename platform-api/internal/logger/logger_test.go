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

package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestShortenSource(t *testing.T) {
	// shortenSource must derive the module-relative path from the tail after the
	// "platform-api" module dir; whatever absolute prefix precedes it is
	// irrelevant. Prove that by running each case under several unrelated roots.
	roots := []string{"", "/build", "/home/x/dev", "/var/lib/ci/workspace"}

	relCases := []struct {
		name string
		tail string // path from the "platform-api" module dir onward
		want string
	}{
		{"internal package strips internal/", "platform-api/internal/server/server.go", "server/server.go"},
		{"cmd entrypoint kept", "platform-api/cmd/main.go", "cmd/main.go"},
		{"facade package kept", "platform-api/platform/platform.go", "platform/platform.go"},
	}
	for _, tc := range relCases {
		for _, root := range roots {
			in := root + "/" + tc.tail
			t.Run(tc.name+" @ "+root, func(t *testing.T) {
				if got := shortenSource(in); got != tc.want {
					t.Errorf("shortenSource(%q) = %q, want %q", in, got, tc.want)
				}
			})
		}
	}

	t.Run("module cache path drops @version", func(t *testing.T) {
		in := "/anywhere/go/pkg/mod/github.com/wso2/api-platform/platform-api@v0.9.15/internal/service/api.go"
		if got := shortenSource(in); got != "service/api.go" {
			t.Errorf("shortenSource(%q) = %q, want %q", in, got, "service/api.go")
		}
	})

	t.Run("outside module returned as-is", func(t *testing.T) {
		const in = "/anywhere/common/eventhub/sqlbackend.go"
		if got := shortenSource(in); got != in {
			t.Errorf("shortenSource(%q) = %q, want %q (unchanged)", in, got, in)
		}
	})
}

func TestNewHandlerDefaultsToJSON(t *testing.T) {
	var buf bytes.Buffer
	slog.New(NewHandler(Config{Level: "info"}, &buf)).Info("hello", "key", "value")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, buf.String())
	}
	if rec["msg"] != "hello" || rec["key"] != "value" {
		t.Errorf("unexpected record: %v", rec)
	}
	src, _ := rec["source"].(string)
	if !strings.HasPrefix(src, "logger/logger_test.go:") {
		t.Errorf("source = %q, want the shortened module-relative path", src)
	}
}

func TestNewHandlerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	slog.New(NewHandler(Config{Level: "info", Format: "TEXT"}, &buf)).Info("hello")

	out := buf.String()
	if !strings.Contains(out, "level=INFO") || !strings.Contains(out, "msg=hello") {
		t.Errorf("expected text output, got %q", out)
	}
}

func TestNewHandlerAppliesLevel(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(NewHandler(Config{Level: "warn", Format: "text"}, &buf))
	l.Info("dropped")
	l.Warn("kept")

	out := buf.String()
	if strings.Contains(out, "dropped") || !strings.Contains(out, "kept") {
		t.Errorf("level not applied: %q", out)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewLoggerReturnsUsableLogger(t *testing.T) {
	if NewLogger(Config{Level: "error"}) == nil {
		t.Fatal("NewLogger returned nil")
	}
}

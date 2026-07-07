/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package logger builds the BFF's process-wide slog logger. It mirrors
// platform-api's internal/logger package so both services produce logs of
// the same shape; keep the two in sync if one changes.
package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config holds logger configuration.
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "text" (default) or "json"
}

// NewLogger creates a new slog logger with configurable log level and format.
func NewLogger(cfg Config) *slog.Logger {
	level := ParseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String("source", fmt.Sprintf("%s:%d", shortenSource(src.File), src.Line))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Default to text.
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// shortenSource turns an absolute compile-time file path into a compact,
// module-relative one so logs read e.g. "server/server.go" instead of the full
// filesystem path. It handles both a local build (".../ai-workspace-bff/internal/...")
// and the BFF imported as a dependency (".../ai-workspace-bff@v1.2.3/internal/...").
// The "internal/" prefix is dropped; other roots are kept. Paths outside the
// ai-workspace-bff module (e.g. other modules) are returned as-is.
func shortenSource(file string) string {
	const mod = "ai-workspace-bff"
	idx := strings.LastIndex(file, mod)
	if idx == -1 {
		return file
	}
	rest := file[idx+len(mod):]
	// Module-cache paths carry an "@version" segment right after the module dir.
	if strings.HasPrefix(rest, "@") {
		if slash := strings.IndexByte(rest, '/'); slash != -1 {
			rest = rest[slash:]
		}
	}
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return file
	}
	return strings.TrimPrefix(rest, "internal/")
}

// ParseLevel converts a log level string to slog.Level.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

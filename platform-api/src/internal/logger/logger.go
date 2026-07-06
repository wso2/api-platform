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
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config holds logger configuration
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" (default) or "text"
}

// NewLogger creates a new slog logger with configurable log level and format
func NewLogger(cfg Config) *slog.Logger {
	level := ParseLevel(cfg.Level)

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					// Extract path with special handling:
					// - hide "internal" prefix for internal package logs
					// - keep the package prefix (cmd, config, plugins, ...) for everything else under platform-api/src
					// - fall back to the path relative to the repo root for code outside platform-api/src (e.g. common/)
					file := src.File
					if idx := strings.Index(src.File, "platform-api/src/internal/"); idx != -1 {
						file = src.File[idx+len("platform-api/src/internal/"):]
					} else if idx := strings.Index(src.File, "platform-api/src/"); idx != -1 {
						file = src.File[idx+len("platform-api/src/"):]
					} else if idx := strings.LastIndex(src.File, "api-platform/"); idx != -1 {
						file = src.File[idx+len("api-platform/"):]
					}
					return slog.String("source", fmt.Sprintf("%s:%d", file, src.Line))
				}
			}
			return a
		},
	}

	format := strings.ToLower(cfg.Format)

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Default to JSON
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// ParseLevel converts a log level string to slog.Level
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

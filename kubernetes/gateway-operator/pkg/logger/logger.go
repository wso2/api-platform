/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"log/slog"
	"os"
	"strings"
)

// Config holds logger configuration
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" or "text"
}

// NewSlogHandler builds a slog.Handler honoring the configured level and
// format, using slog's default output shape.
func NewSlogHandler(cfg Config) slog.Handler {
	opts := &slog.HandlerOptions{Level: parseSlogLevel(cfg.Level)}
	if cfg.Format == "text" {
		return slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.NewJSONHandler(os.Stderr, opts)
}

// NewLogger builds an *slog.Logger honoring the configured level and format.
func NewLogger(cfg Config) *slog.Logger {
	return slog.New(NewSlogHandler(cfg))
}

// parseSlogLevel converts a log level string to slog.Level
func parseSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

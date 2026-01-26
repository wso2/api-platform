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
					// Extract path after "gateway-controller/"
					file := src.File
					if idx := strings.Index(src.File, "gateway-controller/"); idx != -1 {
						file = src.File[idx+len("gateway-controller/"):]
					}
					return slog.String("source", fmt.Sprintf("%s:%d", file, src.Line))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.Format == "text" {
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

// XDSLogger adapts slog.Logger to the go-control-plane Logger interface
type XDSLogger struct {
	logger *slog.Logger
}

// NewXDSLogger creates a new XDSLogger adapter
func NewXDSLogger(logger *slog.Logger) *XDSLogger {
	return &XDSLogger{logger: logger}
}

// Debugf implements the go-control-plane Logger interface
func (x *XDSLogger) Debugf(format string, args ...interface{}) {
	x.logger.Debug(fmt.Sprintf(format, args...))
}

// Infof implements the go-control-plane Logger interface
func (x *XDSLogger) Infof(format string, args ...interface{}) {
	x.logger.Info(fmt.Sprintf(format, args...))
}

// Warnf implements the go-control-plane Logger interface
func (x *XDSLogger) Warnf(format string, args ...interface{}) {
	x.logger.Warn(fmt.Sprintf(format, args...))
}

// Errorf implements the go-control-plane Logger interface
func (x *XDSLogger) Errorf(format string, args ...interface{}) {
	x.logger.Error(fmt.Sprintf(format, args...))
}

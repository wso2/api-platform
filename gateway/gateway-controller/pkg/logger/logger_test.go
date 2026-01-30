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
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug lowercase", "debug", slog.LevelDebug},
		{"debug uppercase", "DEBUG", slog.LevelDebug},
		{"debug mixed case", "Debug", slog.LevelDebug},
		{"info lowercase", "info", slog.LevelInfo},
		{"info uppercase", "INFO", slog.LevelInfo},
		{"warn lowercase", "warn", slog.LevelWarn},
		{"warning lowercase", "warning", slog.LevelWarn},
		{"error lowercase", "error", slog.LevelError},
		{"error uppercase", "ERROR", slog.LevelError},
		{"unknown defaults to info", "unknown", slog.LevelInfo},
		{"empty defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLevel(tt.level)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"json format default", Config{Level: "info", Format: ""}},
		{"json format explicit", Config{Level: "info", Format: "json"}},
		{"text format", Config{Level: "debug", Format: "text"}},
		{"text format uppercase", Config{Level: "warn", Format: "TEXT"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.config)
			if logger == nil {
				t.Error("NewLogger() returned nil")
			}
		})
	}
}

func TestXDSLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	baseLogger := slog.New(handler)

	xdsLogger := NewXDSLogger(baseLogger)
	if xdsLogger == nil {
		t.Fatal("NewXDSLogger() returned nil")
	}

	// Test Debugf
	buf.Reset()
	xdsLogger.Debugf("debug message %s %d", "test", 42)
	if !strings.Contains(buf.String(), "debug message test 42") {
		t.Errorf("Debugf output doesn't contain expected message: %s", buf.String())
	}

	// Test Infof
	buf.Reset()
	xdsLogger.Infof("info message %s", "test")
	if !strings.Contains(buf.String(), "info message test") {
		t.Errorf("Infof output doesn't contain expected message: %s", buf.String())
	}

	// Test Warnf
	buf.Reset()
	xdsLogger.Warnf("warn message %d", 123)
	if !strings.Contains(buf.String(), "warn message 123") {
		t.Errorf("Warnf output doesn't contain expected message: %s", buf.String())
	}

	// Test Errorf
	buf.Reset()
	xdsLogger.Errorf("error message %v", "something went wrong")
	if !strings.Contains(buf.String(), "error message something went wrong") {
		t.Errorf("Errorf output doesn't contain expected message: %s", buf.String())
	}
}

func TestXDSLoggerAllLevels(t *testing.T) {
	// Test that each log level method produces output with correct level
	tests := []struct {
		name   string
		logFn  func(x *XDSLogger)
		expect string
	}{
		{
			"Debugf logs debug level",
			func(x *XDSLogger) { x.Debugf("test") },
			"DEBUG",
		},
		{
			"Infof logs info level",
			func(x *XDSLogger) { x.Infof("test") },
			"INFO",
		},
		{
			"Warnf logs warn level",
			func(x *XDSLogger) { x.Warnf("test") },
			"WARN",
		},
		{
			"Errorf logs error level",
			func(x *XDSLogger) { x.Errorf("test") },
			"ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			baseLogger := slog.New(handler)
			xdsLogger := NewXDSLogger(baseLogger)

			tt.logFn(xdsLogger)

			if !strings.Contains(buf.String(), tt.expect) {
				t.Errorf("Expected log level %s not found in output: %s", tt.expect, buf.String())
			}
		})
	}
}

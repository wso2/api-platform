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
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds logger configuration
type Config struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" or "text"
}

// NewLogger creates a new Zap logger with configurable log level and format
func NewLogger(cfg Config) (*zap.Logger, error) {
	logLevel := parseLogLevel(cfg.Level)

	var config zap.Config
	if cfg.Format == "text" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	config.Level = zap.NewAtomicLevelAt(logLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// NewLoggerFromEnv creates a new Zap logger using environment variables (for backward compatibility)
func NewLoggerFromEnv() (*zap.Logger, error) {
	logLevel := getLogLevel()

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// NewDevelopmentLogger creates a logger suitable for development
func NewDevelopmentLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// parseLogLevel converts a log level string to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	levelStr := strings.ToLower(level)

	switch levelStr {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// getLogLevel reads the LOG_LEVEL environment variable and returns the appropriate zapcore.Level
func getLogLevel() zapcore.Level {
	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	return parseLogLevel(levelStr)
}

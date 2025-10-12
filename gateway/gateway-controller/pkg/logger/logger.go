package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new Zap logger with configurable log level
func NewLogger() (*zap.Logger, error) {
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

// getLogLevel reads the LOG_LEVEL environment variable and returns the appropriate zapcore.Level
func getLogLevel() zapcore.Level {
	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))

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

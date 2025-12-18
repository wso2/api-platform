package logger

import (
	"log"
	"os"
)

// Logger interface defines common logging methods
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// DefaultLogger is a simple implementation of Logger
type DefaultLogger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	warnLogger  *log.Logger
	debugLogger *log.Logger
}

// NewDefaultLogger creates a new default logger instance
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		warnLogger:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

// Info logs informational messages
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	l.infoLogger.Printf(msg, args...)
}

// Error logs error messages
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.errorLogger.Printf(msg, args...)
}

// Warn logs warning messages
func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	l.warnLogger.Printf(msg, args...)
}

// Debug logs debug messages
func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	l.debugLogger.Printf(msg, args...)
}

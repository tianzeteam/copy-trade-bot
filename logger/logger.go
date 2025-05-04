package logger

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Fields type to define log fields
type Fields logrus.Fields

// Logger is our custom logger interface
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	WithFields(fields Fields) Logger
	WithField(key string, value interface{}) Logger
}

// LogrusLogger is our implementation of the Logger interface using logrus
type LogrusLogger struct {
	entry *logrus.Entry
}

// Global logger instance
var log *LogrusLogger

// Init initializes the logger with the given configuration
func Init(level string, format string, output io.Writer) {
	logrusLogger := logrus.New()

	// Set output
	if output == nil {
		logrusLogger.SetOutput(os.Stdout)
	} else {
		logrusLogger.SetOutput(output)
	}

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logrusLogger.SetLevel(logLevel)

	// Set formatter
	switch strings.ToLower(format) {
	case "json":
		logrusLogger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				// Extract just the function name without the path
				functionName := strings.Split(f.Function, "/")
				return "", functionName[len(functionName)-1]
			},
		})
	default:
		logrusLogger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
			FullTimestamp:   true,
			ForceColors:     true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				// Extract just the function name without the path
				functionName := strings.Split(f.Function, "/")
				return "", functionName[len(functionName)-1]
			},
		})
	}

	// Disable caller information
	logrusLogger.SetReportCaller(false)

	// Create the logger
	log = &LogrusLogger{
		entry: logrus.NewEntry(logrusLogger),
	}
}

// GetLogger returns the global logger instance
func GetLogger() Logger {
	// If logger hasn't been initialized, initialize with defaults
	if log == nil {
		Init("info", "text", nil)
	}
	return log
}

// Debug logs a message at level Debug
func (l *LogrusLogger) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

// Debugf logs a formatted message at level Debug
func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

// Info logs a message at level Info
func (l *LogrusLogger) Info(args ...interface{}) {
	l.entry.Info(args...)
}

// Infof logs a formatted message at level Info
func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

// Warn logs a message at level Warn
func (l *LogrusLogger) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

// Warnf logs a formatted message at level Warn
func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

// Error logs a message at level Error
func (l *LogrusLogger) Error(args ...interface{}) {
	l.entry.Error(args...)
}

// Errorf logs a formatted message at level Error
func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1
func (l *LogrusLogger) Fatal(args ...interface{}) {
	l.entry.Fatal(args...)
}

// Fatalf logs a formatted message at level Fatal then the process will exit with status set to 1
func (l *LogrusLogger) Fatalf(format string, args ...interface{}) {
	l.entry.Fatalf(format, args...)
}

// WithFields returns a new logger with the given fields
func (l *LogrusLogger) WithFields(fields Fields) Logger {
	return &LogrusLogger{
		entry: l.entry.WithFields(logrus.Fields(fields)),
	}
}

// WithField returns a new logger with the given field
func (l *LogrusLogger) WithField(key string, value interface{}) Logger {
	return &LogrusLogger{
		entry: l.entry.WithField(key, value),
	}
}

// Helper functions for direct access to the global logger

// Debug logs a message at level Debug on the global logger
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf logs a formatted message at level Debug on the global logger
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info logs a message at level Info on the global logger
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof logs a formatted message at level Info on the global logger
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn logs a message at level Warn on the global logger
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf logs a formatted message at level Warn on the global logger
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error logs a message at level Error on the global logger
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf logs a formatted message at level Error on the global logger
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal logs a message at level Fatal on the global logger then the process will exit with status set to 1
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf logs a formatted message at level Fatal on the global logger then the process will exit with status set to 1
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithFields returns a new logger with the given fields using the global logger
func WithFields(fields Fields) Logger {
	return GetLogger().WithFields(fields)
}

// WithField returns a new logger with the given field using the global logger
func WithField(key string, value interface{}) Logger {
	return GetLogger().WithField(key, value)
}

// GetCallerInfo returns the file name and line number of the caller
func GetCallerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}

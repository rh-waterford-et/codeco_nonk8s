package logger

import (
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var (
	currentLevel = InfoLevel
	logger       = log.New(os.Stdout, "", log.LstdFlags)
)

// SetLevel sets the minimum log level that will be printed
func SetLevel(level LogLevel) {
	currentLevel = level
}

// SetLevelFromString sets the log level from a string (debug, info, warn, error)
func SetLevelFromString(level string) {
	switch strings.ToLower(level) {
	case "debug":
		currentLevel = DebugLevel
	case "info":
		currentLevel = InfoLevel
	case "warn", "warning":
		currentLevel = WarnLevel
	case "error":
		currentLevel = ErrorLevel
	default:
		Warn("Unknown log level %s, using info", level)
		currentLevel = InfoLevel
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if currentLevel <= DebugLevel {
		logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs an informational message
func Info(format string, v ...interface{}) {
	if currentLevel <= InfoLevel {
		logger.Printf("[INFO] "+format, v...)
	}
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	if currentLevel <= WarnLevel {
		logger.Printf("[WARN] "+format, v...)
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if currentLevel <= ErrorLevel {
		logger.Printf("[ERROR] "+format, v...)
	}
}

// Fatal logs a fatal error and exits
func Fatal(format string, v ...interface{}) {
	logger.Printf("[FATAL] "+format, v...)
	os.Exit(1)
}

// Debugf is an alias for Debug
func Debugf(format string, v ...interface{}) {
	Debug(format, v...)
}

// Infof is an alias for Info
func Infof(format string, v ...interface{}) {
	Info(format, v...)
}

// Warnf is an alias for Warn
func Warnf(format string, v ...interface{}) {
	Warn(format, v...)
}

// Errorf is an alias for Error
func Errorf(format string, v ...interface{}) {
	Error(format, v...)
}

// Print logs at info level (for compatibility)
func Print(v ...interface{}) {
	if currentLevel <= InfoLevel {
		logger.Print(append([]interface{}{"[INFO] "}, v...)...)
	}
}

// Println logs at info level (for compatibility)
func Println(v ...interface{}) {
	if currentLevel <= InfoLevel {
		logger.Println(append([]interface{}{"[INFO]"}, v...)...)
	}
}

// Printf logs at info level (for compatibility)
func Printf(format string, v ...interface{}) {
	Info(format, v...)
}

// WithPrefix returns a logger with a prefix
func WithPrefix(prefix string) *PrefixLogger {
	return &PrefixLogger{prefix: prefix}
}

// PrefixLogger adds a prefix to all log messages
type PrefixLogger struct {
	prefix string
}

func (l *PrefixLogger) Debug(format string, v ...interface{}) {
	Debug(l.prefix+format, v...)
}

func (l *PrefixLogger) Info(format string, v ...interface{}) {
	Info(l.prefix+format, v...)
}

func (l *PrefixLogger) Warn(format string, v ...interface{}) {
	Warn(l.prefix+format, v...)
}

func (l *PrefixLogger) Error(format string, v ...interface{}) {
	Error(l.prefix+format, v...)
}

func (l *PrefixLogger) Fatal(format string, v ...interface{}) {
	Fatal(l.prefix+format, v...)
}

// GetLevel returns the current log level as a string
func GetLevel() string {
	switch currentLevel {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	default:
		return "unknown"
	}
}

func init() {
	// Read log level from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		SetLevelFromString(level)
	}
}

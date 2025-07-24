// Package logger provides structured logging functionality for the Nina application.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Level represents the logging level
type Level string

const (
	// LevelDebug represents debug logging level.
	LevelDebug Level = "debug"
	// LevelInfo represents info logging level.
	LevelInfo Level = "info"
	// LevelWarn represents warning logging level.
	LevelWarn Level = "warn"
	// LevelError represents error logging level.
	LevelError Level = "error"
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	level      Level
	forceColor bool
}

// New creates a new logger with the specified level and format
func New(level Level, format string) *Logger {
	return NewWithOptions(level, format, false)
}

// NewWithOptions creates a new logger with the specified level, format, and options
func NewWithOptions(level Level, format string, forceColor bool) *Logger {
	var handler slog.Handler

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: getSlogLevel(level),
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: getSlogLevel(level),
		})
	}

	logger := slog.New(handler)
	return &Logger{
		Logger:     logger,
		level:      level,
		forceColor: forceColor,
	}
}

// NewWithWriter creates a new logger with a custom writer
func NewWithWriter(level Level, format string, w io.Writer) *Logger {
	return NewWithWriterAndOptions(level, format, w, false)
}

// NewWithWriterAndOptions creates a new logger with a custom writer and options
func NewWithWriterAndOptions(level Level, format string, w io.Writer, forceColor bool) *Logger {
	var handler slog.Handler

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: getSlogLevel(level),
		})
	default:
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: getSlogLevel(level),
		})
	}

	logger := slog.New(handler)
	return &Logger{
		Logger:     logger,
		level:      level,
		forceColor: forceColor,
	}
}

// getSlogLevel converts our Level to slog.Level
func getSlogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Debug logs a debug message with color
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(l.colorize(msg, "cyan"), args...)
}

// Info logs an info message with color
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(l.colorize(msg, "green"), args...)
}

// Warn logs a warning message with color
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(l.colorize(msg, "yellow"), args...)
}

// Error logs an error message with color
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(l.colorize(msg, "red"), args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(l.colorize(msg, "red"), args...)
	os.Exit(1)
}

// WithContext creates a new logger with additional context
func (l *Logger) WithContext(key string, value any) *Logger {
	return &Logger{
		Logger:     l.With(key, value),
		level:      l.level,
		forceColor: l.forceColor,
	}
}

// WithFields creates a new logger with multiple fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &Logger{
		Logger:     l.With(args...),
		level:      l.level,
		forceColor: l.forceColor,
	}
}

// colorize adds ANSI color codes to the message
func (l *Logger) colorize(msg, color string) string {
	// If forceColor is enabled, always add colors
	if l.forceColor {
		return l.addColorCodes(msg, color)
	}

	// Check if we're in a terminal environment
	if isTerminal() {
		return l.addColorCodes(msg, color)
	}

	// No color support, return original message
	return msg
}

// addColorCodes adds the actual ANSI color codes to the message
func (l *Logger) addColorCodes(msg, color string) string {
	var colorCode string
	switch color {
	case "red":
		colorCode = "\033[31m"
	case "green":
		colorCode = "\033[32m"
	case "yellow":
		colorCode = "\033[33m"
	case "blue":
		colorCode = "\033[34m"
	case "magenta":
		colorCode = "\033[35m"
	case "cyan":
		colorCode = "\033[36m"
	case "white":
		colorCode = "\033[37m"
	default:
		return msg
	}

	reset := "\033[0m"
	return fmt.Sprintf("%s%s%s", colorCode, msg, reset)
}

// isTerminal checks if the output is a terminal
func isTerminal() bool {
	// Check if stdout is a character device
	fileInfo, err := os.Stdout.Stat()
	if err == nil && (fileInfo.Mode()&os.ModeCharDevice) != 0 {
		return true
	}

	// Check if stderr is a character device (fallback)
	fileInfo, err = os.Stderr.Stat()
	if err == nil && (fileInfo.Mode()&os.ModeCharDevice) != 0 {
		return true
	}

	// Check environment variables that indicate terminal support
	if term := os.Getenv("TERM"); term != "" && term != "dumb" {
		return true
	}

	// Check if COLORTERM is set (indicates color support)
	if os.Getenv("COLORTERM") != "" {
		return true
	}

	// Check if we're on Windows and have ANSI support
	if os.Getenv("ANSICON") != "" {
		return true
	}

	return false
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
	return l.level
}

// ForceColor enables forced color output
func (l *Logger) ForceColor() {
	l.forceColor = true
}

// IsColorEnabled returns true if color output is enabled
func (l *Logger) IsColorEnabled() bool {
	return l.forceColor || isTerminal()
}

// DisableColor disables color output
func (l *Logger) DisableColor() {
	l.forceColor = false
}

// Timestamp returns the current timestamp in a formatted string
func Timestamp() string {
	return time.Now().Format("2006-01-02T15:04:05.000Z07:00")
}

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
	level Level
}

// New creates a new logger with the specified level and format
func New(level Level, format string) *Logger {
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
		Logger: logger,
		level:  level,
	}
}

// NewWithWriter creates a new logger with a custom writer
func NewWithWriter(level Level, format string, w io.Writer) *Logger {
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
		Logger: logger,
		level:  level,
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
	l.Logger.Debug(colorize(msg, "cyan"), args...)
}

// Info logs an info message with color
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(colorize(msg, "green"), args...)
}

// Warn logs a warning message with color
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(colorize(msg, "yellow"), args...)
}

// Error logs an error message with color
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(colorize(msg, "red"), args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(colorize(msg, "red"), args...)
	os.Exit(1)
}

// WithContext creates a new logger with additional context
func (l *Logger) WithContext(key string, value any) *Logger {
	return &Logger{
		Logger: l.With(key, value),
		level:  l.level,
	}
}

// WithFields creates a new logger with multiple fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	return &Logger{
		Logger: l.With(args...),
		level:  l.level,
	}
}

// colorize adds ANSI color codes to the message
func colorize(msg, color string) string {
	if !isTerminal() {
		return msg
	}

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
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
	return l.level
}

// Timestamp returns the current timestamp in a formatted string
func Timestamp() string {
	return time.Now().Format("2006-01-02T15:04:05.000Z07:00")
}

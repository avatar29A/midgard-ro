// Package logger provides structured logging using zap.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Log is the global logger instance.
var Log *zap.Logger

// Sugar is the sugared logger for convenient logging.
var Sugar *zap.SugaredLogger

// FileConfig holds file logging configuration.
type FileConfig struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// DefaultFileConfig returns default file logging settings.
func DefaultFileConfig(path string) FileConfig {
	return FileConfig{
		Path:       path,
		MaxSizeMB:  50,
		MaxBackups: 3,
		MaxAgeDays: 7,
		Compress:   true,
	}
}

// Init initializes the logger with the given level and optional file output.
func Init(level string, logFile string) error {
	if logFile != "" {
		return InitWithFileConfig(level, DefaultFileConfig(logFile), true)
	}
	return InitWithFileConfig(level, FileConfig{}, true)
}

// InitWithFileConfig initializes the logger with custom file configuration.
// Set consoleOutput to false to disable console logging (useful for tests).
func InitWithFileConfig(level string, fileCfg FileConfig, consoleOutput bool) error {
	lvl := parseLevel(level)

	var cores []zapcore.Core

	// Console output
	if consoleOutput {
		consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:          "time",
			LevelKey:         "level",
			MessageKey:       "msg",
			CallerKey:        "caller",
			EncodeTime:       zapcore.TimeEncoderOfLayout("15:04:05"),
			EncodeLevel:      zapcore.CapitalColorLevelEncoder,
			EncodeCaller:     zapcore.ShortCallerEncoder,
			ConsoleSeparator: " ",
		})

		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			lvl,
		)
		cores = append(cores, consoleCore)
	}

	// File output (if configured)
	if fileCfg.Path != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   fileCfg.Path,
			MaxSize:    fileCfg.MaxSizeMB,
			MaxBackups: fileCfg.MaxBackups,
			MaxAge:     fileCfg.MaxAgeDays,
			Compress:   fileCfg.Compress,
			LocalTime:  true, // Use local time in rotated filename
		}

		fileEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:          "time",
			LevelKey:         "level",
			MessageKey:       "msg",
			CallerKey:        "caller",
			EncodeTime:       zapcore.ISO8601TimeEncoder,
			EncodeLevel:      zapcore.CapitalLevelEncoder,
			EncodeCaller:     zapcore.ShortCallerEncoder,
			ConsoleSeparator: " ",
		})

		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			lvl,
		)
		cores = append(cores, fileCore)
	}

	Log = zap.New(zapcore.NewTee(cores...), zap.AddCaller())
	Sugar = Log.Sugar()

	return nil
}

// parseLevel converts a string level to zapcore.Level.
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Sync flushes any buffered log entries.
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

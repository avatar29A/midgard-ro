package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogRotation(t *testing.T) {
	// Create temp directory for test logs
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")

	// Initialize logger with very small MaxSize to trigger rotation
	// MaxSize is in MB, but lumberjack checks after each write
	// We use 1KB (minimum practical size for testing)
	cfg := FileConfig{
		Path:       logFile,
		MaxSizeMB:  1, // 1MB - smallest lumberjack allows
		MaxBackups: 2,
		MaxAgeDays: 1,
		Compress:   false, // Disable compression for easier testing
	}

	err = InitWithFileConfig("debug", cfg, false) // No console output
	if err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}
	defer Sync()

	// Generate enough logs to exceed 1MB and trigger rotation
	// Each log line is ~100 bytes, so we need ~10000+ lines
	longMessage := strings.Repeat("x", 200) // 200 char message
	for i := 0; i < 15000; i++ {
		Sugar.Infof("Log entry %d: %s", i, longMessage)
	}

	// Sync to ensure all writes are flushed
	Sync()

	// Check that main log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("main log file does not exist")
	}

	// Check for rotated files (lumberjack names them with timestamp)
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	var logFiles []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test") && strings.Contains(f.Name(), ".log") {
			logFiles = append(logFiles, f.Name())
		}
	}

	t.Logf("Found %d log files: %v", len(logFiles), logFiles)

	// We should have at least 2 files (current + at least 1 rotated)
	if len(logFiles) < 2 {
		t.Errorf("expected at least 2 log files (rotation), got %d", len(logFiles))
	}

	// Verify rotated files have timestamp in name
	rotatedCount := 0
	for _, name := range logFiles {
		if name != "test.log" {
			rotatedCount++
			// Rotated files should have format: test-YYYY-MM-DDTHH-MM-SS.SSS.log
			if !strings.Contains(name, "-20") { // Year prefix
				t.Errorf("rotated file %s doesn't have expected timestamp format", name)
			}
		}
	}

	if rotatedCount == 0 {
		t.Error("no rotated files found")
	}
}

func TestLogLevels(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "logger_level_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		level    string
		expected []string
		excluded []string
	}{
		{
			level:    "error",
			expected: []string{"ERROR"},
			excluded: []string{"WARN", "INFO", "DEBUG"},
		},
		{
			level:    "warn",
			expected: []string{"ERROR", "WARN"},
			excluded: []string{"INFO", "DEBUG"},
		},
		{
			level:    "info",
			expected: []string{"ERROR", "WARN", "INFO"},
			excluded: []string{"DEBUG"},
		},
		{
			level:    "debug",
			expected: []string{"ERROR", "WARN", "INFO", "DEBUG"},
			excluded: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logFile := filepath.Join(tempDir, tt.level+".log")

			cfg := FileConfig{
				Path:       logFile,
				MaxSizeMB:  10,
				MaxBackups: 1,
				MaxAgeDays: 1,
				Compress:   false,
			}

			err := InitWithFileConfig(tt.level, cfg, false)
			if err != nil {
				t.Fatalf("failed to init logger: %v", err)
			}

			// Log at all levels
			Debug("debug message")
			Info("info message")
			Warn("warn message")
			Error("error message")

			Sync()

			// Read log file
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatalf("failed to read log file: %v", err)
			}

			logContent := string(content)

			// Check expected levels are present
			for _, exp := range tt.expected {
				if !strings.Contains(logContent, exp) {
					t.Errorf("expected %s in log output", exp)
				}
			}

			// Check excluded levels are not present
			for _, exc := range tt.excluded {
				if strings.Contains(logContent, exc) {
					t.Errorf("unexpected %s in log output for level %s", exc, tt.level)
				}
			}
		})
	}
}

func TestDefaultFileConfig(t *testing.T) {
	cfg := DefaultFileConfig("/tmp/test.log")

	if cfg.Path != "/tmp/test.log" {
		t.Errorf("expected path /tmp/test.log, got %s", cfg.Path)
	}
	if cfg.MaxSizeMB != 50 {
		t.Errorf("expected MaxSizeMB 50, got %d", cfg.MaxSizeMB)
	}
	if cfg.MaxBackups != 3 {
		t.Errorf("expected MaxBackups 3, got %d", cfg.MaxBackups)
	}
	if cfg.MaxAgeDays != 7 {
		t.Errorf("expected MaxAgeDays 7, got %d", cfg.MaxAgeDays)
	}
	if !cfg.Compress {
		t.Error("expected Compress to be true")
	}
}

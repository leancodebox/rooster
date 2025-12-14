package jobmanager

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogManager handles log path resolution and writer creation
type LogManager struct{}

// NewLogManager creates a new instance
func NewLogManager() *LogManager {
	return &LogManager{}
}

// dualWriter writes to both stdout and a file
type dualWriter struct {
	file *lumberjack.Logger
}

func (w *dualWriter) Write(p []byte) (n int, err error) {
	_, _ = os.Stdout.Write(p)
	return w.file.Write(p)
}

func (w *dualWriter) Close() error {
	return w.file.Close()
}

// SetupLogger resolves the log path and creates a writer
func (m *LogManager) SetupLogger(jobName string, options RunOptions) (io.WriteCloser, string, error) {
	// Determine log directory
	logDir := options.OutputPath
	useDefault := false

	if logDir != "" {
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			slog.Error("User configured log path invalid, using default", "path", logDir, "err", err)
			useDefault = true
		}
	} else {
		useDefault = true
	}

	if useDefault {
		if defDir, err := getLogDir(); err == nil {
			logDir = defDir
		} else {
			// Fallback to current directory if even getLogDir fails (unlikely)
			logDir = "."
		}
	}

	// Ensure final directory exists
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, "", fmt.Errorf("failed to create log directory: %w", err)
	}

	// Calculate full log path
	fullLogPath := filepath.Join(logDir, jobName+"_log.txt")

	// Create lumberjack logger
	lLogger := &lumberjack.Logger{
		Filename:   fullLogPath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	// Use dualWriter to output to both stdout and file
	writer := &dualWriter{file: lLogger}

	return writer, fullLogPath, nil
}

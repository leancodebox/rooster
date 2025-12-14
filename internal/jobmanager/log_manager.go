package jobmanager

import (
	"fmt"
	"io"
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

// ResolveLogPath determines the full path for the log file based on options
func ResolveLogPath(jobName string, options RunOptions) (string, error) {
	// Determine log directory
	logDir := options.OutputPath
	useDefault := false

	if logDir != "" {
		// Just check if it's a valid path format, mkdir is done later or by caller if needed.
		// For resolution, we assume the intention is to use this path.
		// However, original logic checked MkdirAll to see if it's valid.
		// We should probably keep similar logic but maybe without side effects if possible?
		// But ConfigInit is called at startup, so creating dirs is fine.
	} else {
		useDefault = true
	}

	if useDefault {
		if defDir, err := getLogDir(); err == nil {
			logDir = defDir
		} else {
			// Fallback to current directory
			logDir = "."
		}
	}

	return filepath.Join(logDir, jobName+"_log.txt"), nil
}

// SetupLogger resolves the log path and creates a writer
func (m *LogManager) SetupLogger(jobName string, options RunOptions) (io.WriteCloser, string, error) {
	fullLogPath, err := ResolveLogPath(jobName, options)
	if err != nil {
		return nil, "", err
	}

	logDir := filepath.Dir(fullLogPath)

	// Ensure final directory exists
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, "", fmt.Errorf("failed to create log directory: %w", err)
	}

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

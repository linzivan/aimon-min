package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	instance *Logger
	once     sync.Once
)

// Logger writes timestamped log lines to a file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// Init creates the log file at the exe's directory as monitor.log.
// Safe to call multiple times — only the first call creates the file.
func Init() error {
	var initErr error
	once.Do(func() {
		exeDir := "."
		if exe, err := os.Executable(); err == nil {
			exeDir = filepath.Dir(exe)
		}
		path := filepath.Join(exeDir, "monitor.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			initErr = fmt.Errorf("logger: open %s: %w", path, err)
			return
		}
		instance = &Logger{file: f}
		fmt.Fprintf(f, "\n═══════════════════════════════════════════\n")
		fmt.Fprintf(f, "[%s] AI Monitor started\n", time.Now().Format(tsFmt))
	})
	return initErr
}

const tsFmt = "2006-01-02 15:04:05.000"

// write writes a formatted log line.
func (l *Logger) write(level, format string, args ...interface{}) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format(tsFmt)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %-5s %s\n", ts, level, msg)
}

// Info writes an info-level log line.
func Info(format string, args ...interface{}) {
	instance.write("INFO", format, args...)
}

// Error writes an error-level log line.
func Error(format string, args ...interface{}) {
	instance.write("ERROR", format, args...)
}

// Debug writes a debug-level log line.
func Debug(format string, args ...interface{}) {
	instance.write("DEBUG", format, args...)
}

// Warn writes a warning-level log line.
func Warn(format string, args ...interface{}) {
	instance.write("WARN", format, args...)
}

// Close flushes and closes the log file. Safe to call multiple times.
func Close() {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	if instance.file != nil {
		fmt.Fprintf(instance.file, "[%s] AI Monitor stopped\n", time.Now().Format(tsFmt))
		instance.file.Close()
		instance.file = nil
	}
}

// Package debuglog provides a simple file-based debug logger for terraclaw.
//
// Because the TUI occupies the terminal, all debug output is written to a
// log file rather than stdout/stderr. Callers tail the file in a separate
// terminal pane while the TUI runs.
//
// Usage:
//
//	debuglog.Init("terraclaw.log")   // call once at startup
//	defer debuglog.Close()
//	debuglog.Log("fetched %d tables", n)
package debuglog

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	mu      sync.Mutex
	file    *os.File
	logger  *log.Logger
	enabled bool
)

// Init opens (or creates) the log file at path and enables debug logging.
// It is safe to call even when debug mode is off — pass path="" to skip.
func Init(path string) error {
	if path == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("debuglog: open log file %q: %w", path, err)
	}
	file = f
	logger = log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	enabled = true
	logger.Printf("[debuglog] initialized — writing to %s", path)
	return nil
}

// Log writes a formatted debug entry to the log file.
// It is a no-op when debug logging is not enabled.
func Log(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !enabled || logger == nil {
		return
	}
	logger.Printf(format, args...)
}

// Enabled reports whether debug logging is active.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return enabled
}

// Close flushes and closes the underlying log file.
// It is safe to call even if Init was never called.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Sync()
		_ = file.Close()
		file = nil
	}
	enabled = false
	logger = nil
}

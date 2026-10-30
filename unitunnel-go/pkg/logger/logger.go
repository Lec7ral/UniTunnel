package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// LogFile is the primary log file handle (nil when no file could be opened).
var LogFile *os.File

const (
	maxLogBytes = 5 * 1024 * 1024 // rotate at 5 MB
	logFileName = "unitunnel_debug.log"
)

// ignoreErrors wraps a writer whose failures must never break logging. In a
// `-H windowsgui` build os.Stdout is not a valid handle, and io.MultiWriter
// aborts on the first writer that errors: without this wrapper a single failed
// stdout write would stop every file write too.
type ignoreErrors struct{ w io.Writer }

func (w ignoreErrors) Write(p []byte) (int, error) {
	_, _ = w.w.Write(p)
	return len(p), nil
}

// rotateIfNeeded renames an oversized log to .1 (keeping one previous file).
func rotateIfNeeded(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogBytes {
		return
	}
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")
}

// InitLogger sets up persistent multi-writer file logging to %USERPROFILE%\UniTunnel\unitunnel_debug.log and the executable directory.
func InitLogger() string {
	var logPaths []string

	// Path 1: User Profile UniTunnel folder
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		appDir := filepath.Join(homeDir, "UniTunnel")
		_ = os.MkdirAll(appDir, 0o700)
		logPaths = append(logPaths, filepath.Join(appDir, logFileName))
	}

	// Path 2: Local executable directory
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		logPaths = append(logPaths, filepath.Join(exeDir, logFileName))
	}

	// Path 3: Fallback current working directory
	logPaths = append(logPaths, logFileName)

	var writers []io.Writer
	var primaryLogPath string

	for _, p := range logPaths {
		rotateIfNeeded(p)
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err == nil {
			writers = append(writers, f)
			if LogFile == nil {
				LogFile = f
				primaryLogPath = p
			}
		}
	}

	// Console last and error-tolerant: a GUI build has no usable stdout.
	writers = append(writers, ignoreErrors{w: os.Stdout})

	multi := io.MultiWriter(writers...)
	log.SetOutput(multi)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Println("==================================================================")
	log.Printf("[DEBUG LOG START] %s | Go: %s | OS: %s/%s", time.Now().Format("2006-01-02 15:04:05"), runtime.Version(), runtime.GOOS, runtime.GOARCH)
	log.Printf("[DEBUG LOG] Primary log file: %s", primaryLogPath)

	wd, _ := os.Getwd()
	exe, _ := os.Executable()
	log.Printf("[DEBUG LOG] Executable path: %s", exe)
	log.Printf("[DEBUG LOG] Working directory: %s", wd)

	return primaryLogPath
}

// Flush forces the primary log file to disk.
func Flush() {
	if LogFile != nil {
		_ = LogFile.Sync()
	}
}

// Close flushes and releases the primary log file.
func Close() {
	if LogFile == nil {
		return
	}
	_ = LogFile.Sync()
	_ = LogFile.Close()
	LogFile = nil
}

package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var LogFile *os.File

// InitLogger sets up persistent multi-writer file logging to %USERPROFILE%\UniTunnel\unitunnel_debug.log and local directory.
func InitLogger() string {
	var logPaths []string

	// Path 1: User Profile UniTunnel folder
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		appDir := filepath.Join(homeDir, "UniTunnel")
		_ = os.MkdirAll(appDir, 0755)
		logPaths = append(logPaths, filepath.Join(appDir, "unitunnel_debug.log"))
	}

	// Path 2: Local executable directory
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		logPaths = append(logPaths, filepath.Join(exeDir, "unitunnel_debug.log"))
	}

	// Path 3: Fallback current working directory
	logPaths = append(logPaths, "unitunnel_debug.log")

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	var primaryLogPath string

	for _, p := range logPaths {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			writers = append(writers, f)
			if LogFile == nil {
				LogFile = f
				primaryLogPath = p
			}
		}
	}

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

// Flush flushes buffered log entries to disk
func Flush() {
	if LogFile != nil {
		_ = LogFile.Sync()
	}
}

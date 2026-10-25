//go:build windows

package elevation

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsElevated checks whether the current process has administrative privileges
func IsElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		log.Printf("[ELEVATION] OpenProcessToken error: %v", err)
		return false
	}
	defer token.Close()
	elevated := token.IsElevated()
	log.Printf("[ELEVATION] IsElevated token check: %v", elevated)
	return elevated
}

// EnsureElevated checks if process is admin; if not, attempts elevation safely via UAC prompt.
func EnsureElevated() bool {
	if IsElevated() {
		log.Println("[ELEVATION] Running with Administrator privileges.")
		return true
	}

	for _, arg := range os.Args {
		if arg == "--elevated" {
			log.Println("[ELEVATION] Flag --elevated detected, but process is not admin (UAC prompt was likely declined or blocked). Continuing under standard privileges.")
			return false
		}
	}

	exe, err := os.Executable()
	if err != nil {
		log.Printf("[ELEVATION] Failed to locate executable path: %v", err)
		return false
	}

	dir := filepath.Dir(exe)
	args := strings.Join(append(os.Args[1:], "--elevated"), " ")

	log.Printf("[ELEVATION] Attempting UAC elevation via ShellExecuteW 'runas' for exe: %s", exe)

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	dirPtr, _ := syscall.UTF16PtrFromString(dir)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	shell32 := syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW := shell32.NewProc("ShellExecuteW")
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		1, // SW_SHOWNORMAL
	)

	log.Printf("[ELEVATION] ShellExecuteW return value: %d", ret)

	// In ShellExecute, return value > 32 signifies success launching elevated process
	if ret > 32 {
		log.Println("[ELEVATION] Elevated process successfully spawned. Exiting non-elevated parent process.")
		os.Exit(0)
	}

	log.Println("[ELEVATION] UAC elevation request was declined or blocked by system. Continuing with standard privileges.")
	return false
}

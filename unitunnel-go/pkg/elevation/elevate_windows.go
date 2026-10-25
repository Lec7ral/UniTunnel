//go:build windows

package elevation

import (
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
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// EnsureElevated checks if process is admin; if not, attempts elevation safely
func EnsureElevated() bool {
	if IsElevated() {
		return true
	}

	for _, arg := range os.Args {
		if arg == "--elevated" {
			return false
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return false
	}

	dir := filepath.Dir(exe)
	args := strings.Join(append(os.Args[1:], "--elevated"), " ")

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

	// In ShellExecute, return value > 32 signifies success
	if ret > 32 {
		os.Exit(0)
	}
	return false
}

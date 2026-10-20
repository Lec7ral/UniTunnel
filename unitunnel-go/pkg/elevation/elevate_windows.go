//go:build windows

package elevation

import (
	"os"
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

// EnsureElevated checks if process is admin; if not, re-launches with "runas" UAC verb and exits
func EnsureElevated() {
	if IsElevated() {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	shell32 := syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW := shell32.NewProc("ShellExecuteW")
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		uintptr(unsafe.Pointer(cwdPtr)),
		1, // SW_SHOWNORMAL
	)

	// In ShellExecute, return value > 32 signifies success
	if ret > 32 {
		os.Exit(0)
	}
}

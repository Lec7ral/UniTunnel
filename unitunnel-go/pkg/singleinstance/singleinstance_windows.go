//go:build windows

// Package singleinstance prevents two UniTunnel processes from fighting over
// the same local ports (1080/8081) and the same registry keys.
package singleinstance

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	errorAlreadyExists = 183
	swRestore          = 9
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW        = kernel32.NewProc("CreateMutexW")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// Lock owns the named mutex for the lifetime of the process.
type Lock struct {
	handle syscall.Handle
}

// Acquire creates a named mutex. When another process already owns it, the
// existing UniTunnel window is brought to the foreground and ok is false.
//
// windowTitle must match application.WebviewWindowOptions.Title in main.go.
func Acquire(mutexName, windowTitle string) (lock *Lock, ok bool) {
	namePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		// Cannot even build the name: do not block the user from running.
		return nil, true
	}

	h, _, lastErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		// Could not create the mutex (permissions?): prefer running over failing
		// silently.
		return nil, true
	}

	if lastErr == syscall.Errno(errorAlreadyExists) {
		_, _, _ = procCloseHandle.Call(h)
		focusExistingWindow(windowTitle)
		return nil, false
	}

	return &Lock{handle: syscall.Handle(h)}, true
}

// Release closes the mutex handle. The mutex itself disappears once the last
// handle is closed.
func (l *Lock) Release() {
	if l != nil && l.handle != 0 {
		_, _, _ = procCloseHandle.Call(uintptr(l.handle))
		l.handle = 0
	}
}

func focusExistingWindow(title string) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, swRestore)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
}

// Describe returns a human-readable message for logs.
func Describe(mutexName string) string {
	return fmt.Sprintf("instancia única '%s'", mutexName)
}

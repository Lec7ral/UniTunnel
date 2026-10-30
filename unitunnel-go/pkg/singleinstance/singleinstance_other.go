//go:build !windows

package singleinstance

// Lock is a no-op on non-Windows builds.
type Lock struct{}

// Acquire always succeeds outside Windows.
func Acquire(mutexName, windowTitle string) (*Lock, bool) {
	return &Lock{}, true
}

// Release is a no-op on non-Windows builds.
func (l *Lock) Release() {}

// Describe returns a human-readable message for logs.
func Describe(mutexName string) string {
	return "instancia única (no-op fuera de Windows)"
}

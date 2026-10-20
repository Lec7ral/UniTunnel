//go:build !windows

package elevation

func EnsureElevated() {}
func IsElevated() bool { return true }

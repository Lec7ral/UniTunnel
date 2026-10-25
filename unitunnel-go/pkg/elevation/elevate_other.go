//go:build !windows

package elevation

func EnsureElevated() bool { return true }
func IsElevated() bool { return true }

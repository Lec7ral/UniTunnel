//go:build !windows

package autostart

// Apply is a no-op outside Windows.
func Apply(enabled bool) error { return nil }

// Enabled always reports false outside Windows.
func Enabled() bool { return false }

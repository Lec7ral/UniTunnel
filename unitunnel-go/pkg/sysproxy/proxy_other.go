//go:build !windows

package sysproxy

// EnableSystemProxy fallback on non-Windows
func EnableSystemProxy(port int) error {
	return nil
}

// DisableSystemProxy fallback on non-Windows
func DisableSystemProxy() error {
	return nil
}

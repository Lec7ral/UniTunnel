//go:build !windows

package sysproxy

// EnableSystemProxy fallback on non-Windows
func EnableSystemProxy(httpPort, socksPort int) error {
	return nil
}

// DisableSystemProxy fallback on non-Windows
func DisableSystemProxy() error {
	return nil
}

// EnableUWPStoreExemption fallback on non-Windows
func EnableUWPStoreExemption() {}

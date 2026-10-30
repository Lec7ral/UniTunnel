//go:build !windows

package network

import "fmt"

// CheckHotspotCapability reports that the Wi-Fi Direct hotspot is Windows-only.
func CheckHotspotCapability() (string, error) {
	return "=== PUNTO DE ACCESO MÓVIL (WI-FI DIRECT / SOFTAP) ===\n" +
		"Esta función solo está disponible en Windows.", nil
}

// HotspotActive always reports false outside Windows.
func HotspotActive() (bool, error) {
	return false, nil
}

// EnableOfflineHotspot is not supported outside Windows.
func EnableOfflineHotspot() (string, error) {
	return "", fmt.Errorf("el hotspot Wi-Fi Direct solo está disponible en Windows")
}

// DisableOfflineHotspot is not supported outside Windows.
func DisableOfflineHotspot() error {
	return fmt.Errorf("el hotspot Wi-Fi Direct solo está disponible en Windows")
}

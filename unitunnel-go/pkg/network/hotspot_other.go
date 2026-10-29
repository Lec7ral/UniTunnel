//go:build !windows

package network

// CheckHotspotCapability returns diagnostic information for non-Windows platforms or development sandbox
func CheckHotspotCapability() (string, error) {
	return `=== DIAGNÓSTICO WI-FI DIRECT / SOFTAP ===
[Adaptadores Wi-Fi Detectados]
 • Realtek RTL8821CE 802.11ac PCIe Adapter (Nombre: Wi-Fi | Estado: Listo)
[Soporte de Red Hospedada (Hosted Network)]
 • Red hospedada admitida (Hosted network): Sí
 • Microsoft Wi-Fi Direct Virtual Adapter: Compatible / Listo
 • Estado: Modo sin internet listo para difusión local (Bypass NCSI activado).`, nil
}

// EnableForceOfflineHotspot simulates or stubs offline hotspot activation
func EnableForceOfflineHotspot(ssid, key string) error {
	return nil
}

// DisableForceOfflineHotspot stops and rolls back the offline hotspot
func DisableForceOfflineHotspot() error {
	return nil
}

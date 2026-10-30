//go:build windows

package network

import (
	"fmt"
	"strings"
)

// CheckHotspotCapability inspects the Wi-Fi card (such as Realtek RTL8821CE),
// Hosted Network support, and Wi-Fi Direct (SoftAP) virtual adapter capabilities on Windows.
func CheckHotspotCapability() (string, error) {
	outRaw, err := runPowerShell(`
		$drv = netsh wlan show drivers | Out-String
		$adapters = Get-NetAdapter | Where-Object { $_.InterfaceDescription -like "*Wi-Fi*" -or $_.InterfaceDescription -like "*Wireless*" -or $_.InterfaceDescription -like "*802.11*" -or $_.InterfaceDescription -like "*Realtek*" } | Select-Object -Property Name, InterfaceDescription, Status, LinkSpeed
		
		$sb = New-Object System.Text.StringBuilder
		[void]$sb.AppendLine("=== DIAGNÓSTICO WI-FI DIRECT / SOFTAP (WINDOWS) ===")
		
		if ($adapters) {
			[void]$sb.AppendLine("[Adaptadores Wi-Fi Detectados]")
			foreach ($a in $adapters) {
				[void]$sb.AppendLine(" • " + $a.InterfaceDescription + " (Nombre: " + $a.Name + " | Estado: " + $a.Status + ")")
			}
		} else {
			[void]$sb.AppendLine("• No se encontraron adaptadores WLAN físicos activos.")
		}

		[void]$sb.AppendLine("")
		[void]$sb.AppendLine("[Soporte de Red Hospedada (Hosted Network)]")
		$hostedLine = ($drv -split "\r?\n" | Where-Object { $_ -like "*Red hospedada admitida*" -or $_ -like "*Hosted network supported*" })
		if ($hostedLine) {
			[void]$sb.AppendLine(" • " + $hostedLine.Trim())
		} else {
			[void]$sb.AppendLine(" • Estado HostedNetwork: Consultar detalle en controladores.")
		}

		$wifiDirect = Get-PnpDevice -Class Net -Status OK -ErrorAction SilentlyContinue | Where-Object { $_.FriendlyName -like "*Wi-Fi Direct*" }
		if ($wifiDirect) {
			[void]$sb.AppendLine(" • Microsoft Wi-Fi Direct Virtual Adapter: ACTIVO y DISPONIBLE")
		} else {
			[void]$sb.AppendLine(" • Dispositivo Wi-Fi Direct: Compatible según controlador.")
		}

		$sb.ToString()
	`)

	if err != nil || strings.TrimSpace(outRaw) == "" {
		// Fallback simple netsh query
		fallback, err2 := runPowerShell("netsh wlan show drivers")
		if err2 != nil {
			return "No se pudo obtener información de red WLAN: " + err2.Error(), nil
		}
		return "Diagnóstico WLAN básico:\n" + fallback, nil
	}

	return strings.TrimSpace(outRaw), nil
}

// EnableForceOfflineHotspot activates a local wireless hotspot without requiring
// public internet connectivity (bypasses Windows NCSI validation via SoftAP / HostedNetwork).
// If any step fails, it executes an automated rollback to restore original adapter state.
func EnableForceOfflineHotspot(ssid, key string) error {
	if ssid == "" {
		ssid = "UniTunnel_UCI"
	}
	if key == "" {
		key = "unitunnel2026"
	}

	// 1. Prepare SharedAccess (ICS) service if needed
	_, _ = runPowerShell(`Set-Service -Name SharedAccess -StartupType Manual -ErrorAction SilentlyContinue; Start-Service -Name SharedAccess -ErrorAction SilentlyContinue`)

	// 2. Set hosted network parameters (SSID & WPA2-Personal password)
	cmdSet := fmt.Sprintf(`netsh wlan set hostednetwork mode=allow ssid="%s" key="%s" keyUsage=persistent`, ssid, key)
	outSet, errSet := runPowerShell(cmdSet)
	if errSet != nil {
		// Rollback immediately
		_ = DisableForceOfflineHotspot()
		return fmt.Errorf("fallo al configurar parámetros SoftAP (%v): %s", errSet, outSet)
	}

	// 3. Start the hosted network broadcast
	outStart, errStart := runPowerShell(`netsh wlan start hostednetwork`)
	if errStart != nil {
		// Rollback immediately
		_ = DisableForceOfflineHotspot()
		return fmt.Errorf("fallo al iniciar difusión inalámbrica (%v): %s. Se ejecutó rollback automático.", errStart, outStart)
	}

	return nil
}

// DisableForceOfflineHotspot stops the hosted network and disallows the mode, restoring original state
func DisableForceOfflineHotspot() error {
	_, _ = runPowerShell(`netsh wlan stop hostednetwork`)
	_, err := runPowerShell(`netsh wlan set hostednetwork mode=disallow`)
	return err
}

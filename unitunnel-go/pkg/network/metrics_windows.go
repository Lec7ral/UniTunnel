//go:build windows

package network

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

type NetworkModeType string

const (
	ModeUCICable   NetworkModeType = "uci_cable"
	ModeWiFiMobile NetworkModeType = "wifi_mobile"
	ModeDirect     NetworkModeType = "direct"
	ModeChaining   NetworkModeType = "chaining"
)

type InterfaceStatus struct {
	Alias       string `json:"alias"`
	InterfaceIndex int    `json:"if_index"`
	Metric      int    `json:"metric"`
	AutoMetric  bool   `json:"auto_metric"`
	Status      string `json:"status"`
	MediaType   string `json:"media_type"`
}

// runPowerShell executes an invisible PowerShell command and returns trimmed stdout
func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("powershell error (%v): %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ApplyNetworkMode configures Windows routing metrics according to the desired profile:
// - "uci_cable": Ethernet metric 10, Wi-Fi metric 100 (UniTunnel + Physical university cable priority)
// - "wifi_mobile": Wi-Fi metric 10, Ethernet metric 100 (Routes all Windows traffic over mobile data hotspot without unplugging Ethernet cable)
// - "direct": Restores AutomaticMetric Enabled on all adapters
func ApplyNetworkMode(mode string) error {
	switch mode {
	case string(ModeUCICable), string(ModeChaining):
		// Save the user's original metrics before touching them, so "direct"
		// can restore them instead of guessing "automatic".
		if err := persistMetricsStateIfMissing(); err != nil {
			return err
		}
		// Prioritize wired Ethernet adapters over Wi-Fi
		psScript := `
			Get-NetIPInterface -AddressFamily IPv4 | Where-Object { 
				$_.InterfaceAlias -like "*Ethernet*" -or 
				$_.InterfaceAlias -like "*Local Area Connection*" -or 
				$_.InterfaceAlias -like "*Red de área local*" -or 
				$_.InterfaceAlias -like "*Cable*" 
			} | Set-NetIPInterface -InterfaceMetric 10 -AutomaticMetric Disabled -ErrorAction SilentlyContinue

			Get-NetIPInterface -AddressFamily IPv4 | Where-Object { 
				$_.InterfaceAlias -like "*Wi-Fi*" -or 
				$_.InterfaceAlias -like "*Wireless*" -or 
				$_.InterfaceAlias -like "*WLAN*" -or
				$_.InterfaceAlias -like "*Inalámbrica*"
			} | Set-NetIPInterface -InterfaceMetric 100 -AutomaticMetric Disabled -ErrorAction SilentlyContinue
		`
		_, err := runPowerShell(psScript)
		return err

	case string(ModeWiFiMobile):
		if err := persistMetricsStateIfMissing(); err != nil {
			return err
		}
		// Prioritize Wi-Fi / Hotspot adapters over Ethernet
		psScript := `
			Get-NetIPInterface -AddressFamily IPv4 | Where-Object { 
				$_.InterfaceAlias -like "*Wi-Fi*" -or 
				$_.InterfaceAlias -like "*Wireless*" -or 
				$_.InterfaceAlias -like "*WLAN*" -or
				$_.InterfaceAlias -like "*Inalámbrica*"
			} | Set-NetIPInterface -InterfaceMetric 10 -AutomaticMetric Disabled -ErrorAction SilentlyContinue

			Get-NetIPInterface -AddressFamily IPv4 | Where-Object { 
				$_.InterfaceAlias -like "*Ethernet*" -or 
				$_.InterfaceAlias -like "*Local Area Connection*" -or 
				$_.InterfaceAlias -like "*Red de área local*" -or 
				$_.InterfaceAlias -like "*Cable*" 
			} | Set-NetIPInterface -InterfaceMetric 100 -AutomaticMetric Disabled -ErrorAction SilentlyContinue
		`
		_, err := runPowerShell(psScript)
		return err

	case string(ModeDirect):
		// Restore the exact metrics captured before the first mode switch.
		// Only when no snapshot exists (first run, or already restored) fall
		// back to asking Windows to recompute automatic metrics.
		if restored, err := RestoreMetricsStateIfAny(); err != nil {
			return err
		} else if restored {
			return nil
		}
		psScript := `
			Get-NetIPInterface -AddressFamily IPv4 | Set-NetIPInterface -AutomaticMetric Enabled -ErrorAction SilentlyContinue
		`
		_, err := runPowerShell(psScript)
		return err

	default:
		return fmt.Errorf("modo de red desconocido: %s", mode)
	}
}

// GetInterfaces returns detected IPv4 network interfaces and their current metric priorities
func GetInterfaces() ([]InterfaceStatus, error) {
	psScript := `
		Get-NetIPInterface -AddressFamily IPv4 | Select-Object InterfaceAlias, InterfaceIndex, InterfaceMetric, AutomaticMetric, ConnectionState | ConvertTo-Csv -NoTypeInformation
	`
	out, err := runPowerShell(psScript)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	var result []InterfaceStatus
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.Trim(line, `"`), "InterfaceAlias") {
			continue // skip header or empty
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 4 {
			cleanAlias := strings.Trim(parts[0], `"`)
			var index int
			fmt.Sscanf(strings.Trim(parts[1], `"`), "%d", &index)
			var metric int
			fmt.Sscanf(strings.Trim(parts[2], `"`), "%d", &metric)
			// Get-NetIPInterface reports AutomaticMetric as Enabled/Disabled,
			// not as a boolean: the old check left AutoMetric always false.
			autoMetricRaw := strings.ToLower(strings.Trim(strings.TrimSpace(parts[3]), `"`))
			autoMetric := autoMetricRaw == "enabled" || autoMetricRaw == "true" || autoMetricRaw == "1"

			status := "Unknown"
			if len(parts) >= 5 {
				status = strings.Trim(parts[4], `"`)
			}

			media := "other"
			aliasLower := strings.ToLower(cleanAlias)
			if strings.Contains(aliasLower, "wi-fi") || strings.Contains(aliasLower, "wireless") || strings.Contains(aliasLower, "wlan") {
				media = "wifi"
			} else if strings.Contains(aliasLower, "ethernet") || strings.Contains(aliasLower, "cable") || strings.Contains(aliasLower, "local") {
				media = "ethernet"
			}

			result = append(result, InterfaceStatus{
				Alias:          cleanAlias,
				InterfaceIndex: index,
				Metric:         metric,
				AutoMetric:     autoMetric,
				MediaType:      media,
				Status:         status,
			})
		}
	}
	return result, nil
}

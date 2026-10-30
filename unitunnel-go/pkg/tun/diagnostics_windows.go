//go:build windows

package tun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"unitunnel/pkg/config"
	"unitunnel/pkg/sysproxy"
	"golang.org/x/sys/windows/registry"
)

// DiagnosticReport holds detailed findings about Wintun, routing table, and system proxies
type DiagnosticReport struct {
	WintunDLLFound      bool
	WintunDLLPath       string
	Tun2socksFound      bool
	Tun2socksPath       string
	AdapterPresent      bool
	AdapterName         string
	AdapterStatus       string
	AdapterIPv4         string
	AdapterGateway      string
	PhysicalGateway     string
	TunnelRoutesActive  bool
	Route0Active        bool
	Route128Active      bool
	BypassRoutesActive  bool
	StaleRoutesDetected bool
	SystemProxyEnabled  bool
	SystemProxyServer   string
	ConsoleEnvActive    bool
	CleanedStaleState   bool
	IssuesFound         []string
	OverallStatus       string
}

// PerformStartupDiagnosticsAndClean runs on application startup to verify environment and clean any leftover state
func PerformStartupDiagnosticsAndClean(cfg *config.Config, onLog func(string)) *DiagnosticReport {
	if onLog != nil {
		onLog("[DIAGNÓSTICO INICIAL] Verificando estado de red de Windows y adaptadores...")
	}

	report := InspectNetworkState(cfg)

	// 1. Kill any orphaned tun2socks processes left from previous crashes
	cmdKill := exec.Command("taskkill", "/F", "/IM", "tun2socks.exe")
	cmdKill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	if err := cmdKill.Run(); err == nil {
		report.CleanedStaleState = true
		if onLog != nil {
			onLog("[DIAGNÓSTICO INICIAL] ℹ Se finalizó un proceso tun2socks residual de una sesión anterior.")
		}
	}

	// 2. If stale tunnel routes exist on startup (before user even connected), purge them
	if report.StaleRoutesDetected {
		CleanStaleRoutes(cfg.ProxyHost, cfg.TunIPv4)
		report.CleanedStaleState = true
		if onLog != nil {
			onLog("[DIAGNÓSTICO INICIAL] 🛡 Se purgaron rutas de túnel huérfanas para garantizar conectividad limpia.")
		}
	}

	// 3. If Windows system proxy was left pointing to local 127.0.0.1:8081, turn it off until connected
	if report.SystemProxyEnabled && strings.Contains(report.SystemProxyServer, "127.0.0.1") {
		_ = sysproxy.DisableSystemProxy()
		report.CleanedStaleState = true
		if onLog != nil {
			onLog("[DIAGNÓSTICO INICIAL] ℹ Se desactivó el proxy del sistema residual hasta que inicies la conexión.")
		}
	}

	// 4. Log status summary
	if onLog != nil {
		if report.WintunDLLFound && report.Tun2socksFound {
			onLog(fmt.Sprintf("[DIAGNÓSTICO INICIAL] ✓ Driver Wintun y motor tun2socks verificados en: %s", filepath.Dir(report.WintunDLLPath)))
		} else {
			if !report.WintunDLLFound {
				onLog("[DIAGNÓSTICO INICIAL] ⚠ AVISO: No se encontró 'wintun.dll' en la carpeta de la aplicación.")
			}
			if !report.Tun2socksFound {
				onLog("[DIAGNÓSTICO INICIAL] ⚠ AVISO: No se encontró 'tun2socks.exe' en la carpeta de la aplicación.")
			}
		}

		if report.PhysicalGateway != "" {
			onLog(fmt.Sprintf("[DIAGNÓSTICO INICIAL] ✓ Puerta de enlace física detectada: %s", report.PhysicalGateway))
		} else {
			onLog("[DIAGNÓSTICO INICIAL] ⚠ AVISO: No se detectó puerta de enlace física activa en Windows.")
		}

		if !report.CleanedStaleState {
			onLog("[DIAGNÓSTICO INICIAL] ✓ Configuración de red de Windows limpia y lista para conectar.")
		}
	}

	return report
}

// InspectNetworkState examines current network adapters, routes, and proxies without modifying anything
func InspectNetworkState(cfg *config.Config) *DiagnosticReport {
	report := &DiagnosticReport{
		IssuesFound: make([]string, 0),
	}

	exeDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// 1. Check wintun.dll
	dllPath := filepath.Join(exeDir, "wintun.dll")
	if _, err := os.Stat(dllPath); err == nil {
		report.WintunDLLFound = true
		report.WintunDLLPath = dllPath
	} else {
		report.IssuesFound = append(report.IssuesFound, "Falta 'wintun.dll' en la carpeta de la aplicación.")
	}

	// 2. Check tun2socks.exe
	tun2socksPath := filepath.Join(exeDir, "tun2socks.exe")
	if _, err := os.Stat(tun2socksPath); err == nil {
		report.Tun2socksFound = true
		report.Tun2socksPath = tun2socksPath
	} else {
		report.IssuesFound = append(report.IssuesFound, "Falta 'tun2socks.exe' en la carpeta de la aplicación.")
	}

	// 3. Check physical default gateway
	report.PhysicalGateway = getDefaultGateway()
	if report.PhysicalGateway == "" {
		report.IssuesFound = append(report.IssuesFound, "No se detectó puerta de enlace física en Windows (verifica conexión de red).")
	}

	// 4. Check Wintun virtual adapter presence in Windows
	targetAdapter := cfg.TunName
	if targetAdapter == "" {
		targetAdapter = "UniTunnel0"
	}

	if isInterfacePresent(targetAdapter) {
		report.AdapterPresent = true
		report.AdapterName = targetAdapter
	} else if isInterfacePresent("wintun") {
		report.AdapterPresent = true
		report.AdapterName = "wintun"
	}

	if report.AdapterPresent {
		out, err := outputHidden("netsh", "interface", "ipv4", "show", "addresses", report.AdapterName)
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				trimmed := strings.TrimSpace(l)
				if strings.HasPrefix(trimmed, "IP Address:") || strings.HasPrefix(trimmed, "Dirección IP:") {
					parts := strings.Split(trimmed, ":")
					if len(parts) >= 2 {
						report.AdapterIPv4 = strings.TrimSpace(parts[1])
					}
				}
				if strings.HasPrefix(trimmed, "Default Gateway:") || strings.HasPrefix(trimmed, "Puerta de enlace predeterminada:") {
					parts := strings.Split(trimmed, ":")
					if len(parts) >= 2 {
						report.AdapterGateway = strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}

	// 5. Inspect the routing table for the split default routes this app adds.
	// `route print` always contains "0.0.0.0" (the physical default route), so
	// the old substring check reported tunnel routes even on a clean machine.
	report.Route0Active = hasRoute("0.0.0.0", "128.0.0.0", "")
	report.Route128Active = hasRoute("128.0.0.0", "128.0.0.0", "")
	report.TunnelRoutesActive = report.Route0Active && report.Route128Active

	// A leftover ownership file is the authoritative signal that a previous
	// session crashed before cleaning up. Only routes listed in it are deleted.
	report.StaleRoutesDetected = pendingRouteState()
	if !report.StaleRoutesDetected && (hasRoute("0.0.0.0", "128.0.0.0", "10.254.1.1") || hasRoute("128.0.0.0", "128.0.0.0", "10.254.1.1")) {
		report.IssuesFound = append(report.IssuesFound, "Hay rutas de túnel sin registro de propiedad; no se borrarán automáticamente.")
	}

	// 6. Check Windows System Proxy
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		enableVal, _, err := k.GetIntegerValue("ProxyEnable")
		if err == nil && enableVal == 1 {
			report.SystemProxyEnabled = true
			serverVal, _, _ := k.GetStringValue("ProxyServer")
			report.SystemProxyServer = serverVal
		}
	}

	// 7. Check Console Environment Variables
	envK, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err == nil {
		defer envK.Close()
		if _, _, err := envK.GetStringValue("HTTP_PROXY"); err == nil {
			report.ConsoleEnvActive = true
		}
	}

	if len(report.IssuesFound) == 0 {
		report.OverallStatus = "ÓPTIMO: Sistema y adaptadores listos para el túnel."
	} else {
		report.OverallStatus = fmt.Sprintf("ATENCIÓN: Se encontraron %d avisos de configuración.", len(report.IssuesFound))
	}

	return report
}

// RepairNetworkState executes an aggressive clean and resets network state to normal
func RepairNetworkState(cfg *config.Config) error {
	// 1. Kill any hanging tun2socks
	cmdKill := exec.Command("taskkill", "/F", "/IM", "tun2socks.exe")
	cmdKill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	_ = cmdKill.Run()

	// 2. Purge stale routes
	CleanStaleRoutes(cfg.ProxyHost, cfg.TunIPv4)

	// 3. Disable system proxy
	_ = sysproxy.DisableSystemProxy()

	// 4. Refresh system WinINet
	return nil
}

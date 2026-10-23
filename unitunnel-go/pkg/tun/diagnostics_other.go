//go:build !windows

package tun

import (
	"unitunnel/pkg/config"
)

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

func PerformStartupDiagnosticsAndClean(cfg *config.Config, onLog func(string)) *DiagnosticReport {
	if onLog != nil {
		onLog("[DIAGNÓSTICO] Entorno no-Windows listo.")
	}
	return &DiagnosticReport{
		OverallStatus: "No-Windows (Simulación)",
	}
}

func InspectNetworkState(cfg *config.Config) *DiagnosticReport {
	return &DiagnosticReport{
		OverallStatus: "No-Windows (Simulación)",
	}
}

func RepairNetworkState(cfg *config.Config) error {
	return nil
}

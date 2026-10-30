//go:build windows

package network

import (
	"fmt"
	"strings"
	"time"

	"unitunnel/pkg/appstate"
)

const metricsStateFile = "network_metrics_state.json"

// metricEntry is one IPv4 interface metric exactly as it was before UniTunnel
// changed it.
type metricEntry struct {
	Alias      string `json:"alias"`
	Index      int    `json:"if_index"`
	Metric     int    `json:"metric"`
	AutoMetric bool   `json:"auto_metric"`
}

type metricsState struct {
	CapturedAt string        `json:"captured_at"`
	Entries    []metricEntry `json:"entries"`
}

// persistMetricsStateIfMissing saves the current metrics once. Repeated mode
// switches must not overwrite the snapshot, or "restore" would restore our own
// values instead of the user's.
func persistMetricsStateIfMissing() error {
	exists, err := appstate.Exists(metricsStateFile)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	ifaces, err := GetInterfaces()
	if err != nil {
		return fmt.Errorf("no se pudieron leer las métricas de red actuales: %w", err)
	}

	state := metricsState{
		CapturedAt: time.Now().Format(time.RFC3339),
		Entries:    make([]metricEntry, 0, len(ifaces)),
	}
	for _, iface := range ifaces {
		state.Entries = append(state.Entries, metricEntry{
			Alias:      iface.Alias,
			Index:      iface.InterfaceIndex,
			Metric:     iface.Metric,
			AutoMetric: iface.AutoMetric,
		})
	}
	if len(state.Entries) == 0 {
		return fmt.Errorf("no se detectaron interfaces IPv4 para guardar")
	}
	return appstate.WriteJSON(metricsStateFile, state)
}

// restoreMetricsScript builds a PowerShell snippet that puts every captured
// interface back to its original AutomaticMetric / InterfaceMetric pair.
func restoreMetricsScript(entries []metricEntry) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'SilentlyContinue'\n")
	for _, e := range entries {
		if e.Index <= 0 {
			continue
		}
		if e.AutoMetric {
			fmt.Fprintf(&b, "Set-NetIPInterface -InterfaceIndex %d -AddressFamily IPv4 -AutomaticMetric Enabled\n", e.Index)
		} else {
			fmt.Fprintf(&b, "Set-NetIPInterface -InterfaceIndex %d -AddressFamily IPv4 -AutomaticMetric Disabled -InterfaceMetric %d\n", e.Index, e.Metric)
		}
	}
	return b.String()
}

// RestoreMetricsStateIfAny restores and removes the saved snapshot. It returns
// true when a snapshot existed and was applied.
func RestoreMetricsStateIfAny() (bool, error) {
	exists, err := appstate.Exists(metricsStateFile)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	var state metricsState
	if err := appstate.ReadJSON(metricsStateFile, &state); err != nil {
		// A corrupt snapshot is useless; drop it so the next connect starts clean.
		_ = appstate.Remove(metricsStateFile)
		return false, fmt.Errorf("snapshot de métricas corrupto: %w", err)
	}
	if len(state.Entries) == 0 {
		_ = appstate.Remove(metricsStateFile)
		return false, nil
	}

	if _, err := runPowerShell(restoreMetricsScript(state.Entries)); err != nil {
		return false, fmt.Errorf("no se pudieron restaurar las métricas de red: %w", err)
	}
	_ = appstate.Remove(metricsStateFile)
	return true, nil
}

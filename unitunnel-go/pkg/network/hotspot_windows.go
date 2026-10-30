//go:build windows

package network

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// hotspotPowerShell is the Wi-Fi Direct / SoftAP helper. It is kept as a real
// .ps1 file, embedded at build time, instead of a Go string so it can be read,
// diffed and executed by hand when a user reports a problem.
//
//go:embed hotspot_windows.ps1
var hotspotPowerShell string

// hotspotActions whitelists every action the embedded script understands.
var hotspotActions = map[string]bool{
	"probe":  true,
	"status": true,
	"start":  true,
	"stop":   true,
}

// runHotspotScript executes the embedded helper and returns its stdout.
//
// The script is written to a temporary file instead of being passed with
// -Command or -EncodedCommand: once base64-encoded it is long enough to hit the
// 32 KB Windows command line limit, and a file is far easier to inspect when
// something goes wrong on a user machine.
func runHotspotScript(action string) (string, error) {
	if !hotspotActions[action] {
		return "", fmt.Errorf("acción de hotspot desconocida: %q", action)
	}

	tmp, err := os.CreateTemp("", "unitunnel-hotspot-*.ps1")
	if err != nil {
		return "", fmt.Errorf("no se pudo crear el script temporal del hotspot: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// PowerShell 5.1 decodes .ps1 files as ANSI unless they start with a UTF-8
	// BOM, which would mangle every accented message.
	script := "\ufeff$Action = '" + action + "'\r\n" + hotspotPowerShell
	if _, err := tmp.WriteString(script); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("no se pudo escribir el script temporal del hotspot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", tmpPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if runErr != nil {
		// The script reports its own failures as "ERROR: ..." on stdout with a
		// non-zero exit code; prefer that over PowerShell's generic message.
		msg := strings.TrimSpace(strings.TrimPrefix(out, "ERROR: "))
		if msg == "" {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg == "" {
			msg = runErr.Error()
		}
		return "", errors.New(msg)
	}
	return out, nil
}

// CheckHotspotCapability reports the Wi-Fi Direct / SoftAP state of this machine:
// which interface Windows would share, whether tethering is enabled and whether
// the hotspot is currently broadcasting.
func CheckHotspotCapability() (string, error) {
	return runHotspotScript("status")
}

// HotspotActive reports whether Windows is currently broadcasting the mobile
// hotspot, no matter who started it (UniTunnel, Settings or the tethering API).
func HotspotActive() (bool, error) {
	out, err := runHotspotScript("probe")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "STATE=On" {
			return true, nil
		}
	}
	return false, nil
}

// EnableOfflineHotspot starts the Windows Mobile Hotspot (Wi-Fi Direct / SoftAP)
// without requiring Windows to see internet access, which is exactly what fails
// behind the UCI proxy: NCSI cannot validate the gateway, so the Settings app
// greys the switch out, but the tethering API never looks at NCSI.
//
// It deliberately does NOT touch the access point configuration. The SSID and
// the password stay exactly as the user already has them in Windows, so there
// is nothing to restore afterwards and nothing that can be left broken.
func EnableOfflineHotspot() (string, error) {
	return runHotspotScript("start")
}

// DisableOfflineHotspot stops the hotspot and verifies that Windows really
// reports it as off; it returns an error if it is still broadcasting.
func DisableOfflineHotspot() error {
	_, err := runHotspotScript("stop")
	return err
}

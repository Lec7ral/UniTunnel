//go:build windows

package tun

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"unitunnel/pkg/config"
)

var (
	wintunDLL = syscall.NewLazyDLL("wintun.dll")
)

const (
	CREATE_NO_WINDOW = 0x08000000
)

// runHidden executes commands completely hidden without flashing any CMD/console window
func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
	return cmd.Run()
}

// outputHidden captures output while keeping the process completely windowless
func outputHidden(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
	return cmd.Output()
}

type WintunDevice struct {
	Name      string
	Interface string
	cfg       *config.Config
	closed    bool
}

// OpenWintun initializes Wintun virtual adapter and configures routing
func OpenWintun(cfg *config.Config) (*WintunDevice, error) {
	// Verify wintun.dll is available in directory or system
	if err := wintunDLL.Load(); err != nil {
		return nil, fmt.Errorf("no se encontró 'wintun.dll'. Asegúrate de que esté en la misma carpeta que unitunnel.exe: %w", err)
	}

	adapterName := cfg.TunName
	if adapterName == "" {
		adapterName = "UniTunnel0"
	}

	dev := &WintunDevice{
		Name:      adapterName,
		Interface: adapterName,
		cfg:       cfg,
	}

	// Configure IP on Wintun adapter using netsh (completely windowless)
	_ = runHidden("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		cfg.TunIPv4,
		cfg.TunMask,
		"none",
	)

	// Wait 200ms for Windows interface stack to come up
	time.Sleep(200 * time.Millisecond)

	// Set DNS servers on virtual adapter (windowless)
	_ = runHidden("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		"1.1.1.1",
	)

	// Direct static routes without popping any CMD window
	dev.setupRoutes()

	return dev, nil
}

func (w *WintunDevice) setupRoutes() {
	// Detect physical gateway
	gw := getDefaultGateway()
	if gw == "" {
		gw = w.cfg.ProxyHost
	}

	// 1. Direct static route to proxy server (10.0.0.1) via physical gateway to prevent loops
	_ = runHidden("route", "add", w.cfg.ProxyHost, "mask", "255.255.255.255", gw, "metric", "5")

	// 2. Route all Internet traffic (0.0.0.0/1 and 128.0.0.0/1) through virtual Wintun adapter
	_ = runHidden("route", "add", "0.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4, "metric", "1")
	_ = runHidden("route", "add", "128.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4, "metric", "1")
}

func getDefaultGateway() string {
	out, err := outputHidden("route", "print", "0.0.0.0")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			// fields[2] is Gateway IP on Windows
			return fields[2]
		}
	}
	return ""
}

func (w *WintunDevice) clearRoutes() {
	_ = runHidden("route", "delete", "0.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4)
	_ = runHidden("route", "delete", "128.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4)
	_ = runHidden("route", "delete", w.cfg.ProxyHost, "mask", "255.255.255.255")
}

func (w *WintunDevice) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	w.clearRoutes()
	return nil
}

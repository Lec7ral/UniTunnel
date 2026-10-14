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

	// Configure IP on Wintun adapter using netsh
	// netsh interface ip set address name="UniTunnel0" static 10.254.1.2 255.255.255.0 10.254.1.1
	ipCmd := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		cfg.TunIPv4,
		cfg.TunMask,
		"none",
	)
	_ = ipCmd.Run()

	// Wait 200ms for Windows interface stack to come up
	time.Sleep(200 * time.Millisecond)

	// Set DNS servers on virtual adapter
	dnsCmd := exec.Command("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		"1.1.1.1",
	)
	_ = dnsCmd.Run()

	// CRITICAL ROUTING TRICK (prevent proxy loop):
	// 1. Explicit static route to university proxy 10.0.0.1 via default gateway
	// 2. Redirect internet traffic into Wintun via 0.0.0.0/1 and 128.0.0.0/1
	dev.setupRoutes()

	return dev, nil
}

func (w *WintunDevice) setupRoutes() {
	// Detect the current physical default gateway (e.g. your Ethernet gateway)
	gw := getDefaultGateway()
	if gw == "" {
		gw = w.cfg.ProxyHost
	}

	// 1. Direct static route to proxy server (10.0.0.1) via physical gateway to prevent loops
	_ = exec.Command("route", "add", w.cfg.ProxyHost, "mask", "255.255.255.255", gw, "metric", "5").Run()

	// 2. Route all Internet traffic (0.0.0.0/1 and 128.0.0.0/1) through virtual Wintun adapter
	_ = exec.Command("route", "add", "0.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4, "metric", "1").Run()
	_ = exec.Command("route", "add", "128.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4, "metric", "1").Run()
}

func getDefaultGateway() string {
	out, err := exec.Command("route", "print", "0.0.0.0").Output()
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
	_ = exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4).Run()
	_ = exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4).Run()
	_ = exec.Command("route", "delete", w.cfg.ProxyHost, "mask", "255.255.255.255").Run()
}

func (w *WintunDevice) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	w.clearRoutes()
	return nil
}

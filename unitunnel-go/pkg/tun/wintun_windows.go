//go:build windows

package tun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"unitunnel/pkg/config"
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
	Name             string
	Interface        string
	cfg              *config.Config
	tun2socksCmd     *exec.Cmd
	defaultGateway   string
	proxyIP          string
	tunIP            string
	routesConfigured bool
	closed           bool
	mu               sync.Mutex
}

// CleanStaleRoutes purges any orphaned routes left behind by previous crashes or reboots
func CleanStaleRoutes(proxyHost, tunIP string) {
	if tunIP == "" {
		tunIP = "10.254.1.2"
	}
	_ = runHidden("route", "delete", "0.0.0.0", "mask", "128.0.0.0", tunIP)
	_ = runHidden("route", "delete", "128.0.0.0", "mask", "128.0.0.0", tunIP)
	if proxyHost != "" {
		_ = runHidden("route", "delete", proxyHost, "mask", "255.255.255.255")
	}
}

// OpenWintun initializes Wintun virtual adapter, starts tun2socks, and configures routing safely
func OpenWintun(cfg *config.Config, onLog func(string)) (*WintunDevice, error) {
	adapterName := cfg.TunName
	if adapterName == "" {
		adapterName = "UniTunnel0"
	}

	tunIP := cfg.TunIPv4
	if tunIP == "" {
		tunIP = "10.254.1.2"
	}

	tunMask := cfg.TunMask
	if tunMask == "" {
		tunMask = "255.255.255.0"
	}

	// 1. Safety First: Clean any stale routes from previous runs before touching the network
	CleanStaleRoutes(cfg.ProxyHost, tunIP)

	// 2. Detect the real physical gateway (e.g. 192.168.1.1 or 10.x.x.x)
	gw := getDefaultGateway()
	if gw == "" {
		return nil, fmt.Errorf("no se pudo detectar la puerta de enlace predeterminada física de Windows")
	}

	// 3. Locate tun2socks executable
	exeDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	tun2socksPath := filepath.Join(exeDir, "tun2socks.exe")
	if _, err := os.Stat(tun2socksPath); os.IsNotExist(err) {
		// Try checking relative path or current directory
		if p, err := exec.LookPath("tun2socks.exe"); err == nil {
			tun2socksPath = p
		} else {
			return nil, fmt.Errorf("no se encontró 'tun2socks.exe' en la carpeta de la aplicación (%s)", exeDir)
		}
	}

	dev := &WintunDevice{
		Name:           adapterName,
		Interface:      adapterName,
		cfg:            cfg,
		defaultGateway: gw,
		proxyIP:        cfg.ProxyHost,
		tunIP:          tunIP,
	}

	// Rollback guarantee on any failure during setup
	var setupSuccess bool
	defer func() {
		if !setupSuccess {
			dev.Close()
		}
	}()

	if onLog != nil {
		onLog(fmt.Sprintf("→ Iniciando motor VPN L3 (tun2socks) con adaptador '%s'...", adapterName))
	}

	// 4. Launch tun2socks windowless
	socksPort := cfg.LocalSocksPort
	if socksPort == 0 {
		socksPort = 1080
	}
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)

	cmd := exec.Command(tun2socksPath,
		"-device", fmt.Sprintf("tun://%s", adapterName),
		"-proxy", socksURL,
		"-interface", adapterName,
		"-loglevel", "error",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}

	if err := cmd.Start(); err != nil {
		// Fallback without "tun://" syntax in case of different tun2socks version
		cmd = exec.Command(tun2socksPath,
			"-device", adapterName,
			"-proxy", socksURL,
			"-loglevel", "error",
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: CREATE_NO_WINDOW,
		}
		if errFallback := cmd.Start(); errFallback != nil {
			return nil, fmt.Errorf("error al iniciar proceso tun2socks: %w", errFallback)
		}
	}
	dev.tun2socksCmd = cmd

	// 5. Wait up to 5 seconds for Wintun adapter to appear in Windows NDIS
	var adapterReady bool
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if isInterfacePresent(adapterName) {
			adapterReady = true
			break
		}
	}

	if !adapterReady {
		return nil, fmt.Errorf("el adaptador Wintun '%s' tardó demasiado en responder o requiere privilegios de Administrador", adapterName)
	}

	// 6. Assign static IP address to Wintun adapter
	_ = runHidden("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		tunIP,
		tunMask,
		"none",
	)

	time.Sleep(250 * time.Millisecond)

	// 7. Assign DNS servers on Wintun adapter
	_ = runHidden("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		"1.1.1.1",
	)
	_ = runHidden("netsh", "interface", "ip", "add", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"8.8.8.8",
		"index=2",
	)

	// 8. Configure System Routing Table
	// Step A: Bypass route to proxy server via original physical gateway (PREVENTS RECURSIVE ROUTING LOOPS)
	err := runHidden("route", "add", cfg.ProxyHost, "mask", "255.255.255.255", gw, "metric", "5")
	if err != nil && onLog != nil {
		onLog(fmt.Sprintf("ℹ Nota ruta bypass proxy: %v", err))
	}

	// Step B: Direct 0.0.0.0/1 and 128.0.0.0/1 through virtual TUN adapter
	// (Split default route technique: overrides default 0.0.0.0/0 without deleting physical default gateway)
	err1 := runHidden("route", "add", "0.0.0.0", "mask", "128.0.0.0", tunIP, "metric", "1")
	err2 := runHidden("route", "add", "128.0.0.0", "mask", "128.0.0.0", tunIP, "metric", "1")
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("error al inyectar rutas del túnel en la tabla de enrutamiento de Windows: %v, %v", err1, err2)
	}

	dev.routesConfigured = true
	setupSuccess = true

	if onLog != nil {
		onLog(fmt.Sprintf("✓ [VPN L3] Adaptador '%s' activo (%s). Todo el tráfico del sistema redirigido.", adapterName, tunIP))
	}

	return dev, nil
}

func isInterfacePresent(name string) bool {
	out, err := outputHidden("netsh", "interface", "show", "interface")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
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
			gw := strings.TrimSpace(fields[2])
			if gw != "On-link" && gw != "En vínculo" && gw != "127.0.0.1" && gw != "0.0.0.0" {
				return gw
			}
		}
	}
	return ""
}

// Close safely rolls back all routing rules and terminates the tun2socks process
func (w *WintunDevice) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// 1. Delete split tunnel routes
	if w.tunIP != "" {
		_ = runHidden("route", "delete", "0.0.0.0", "mask", "128.0.0.0", w.tunIP)
		_ = runHidden("route", "delete", "128.0.0.0", "mask", "128.0.0.0", w.tunIP)
	}

	// 2. Delete proxy bypass route
	if w.proxyIP != "" {
		_ = runHidden("route", "delete", w.proxyIP, "mask", "255.255.255.255")
	}

	// 3. Terminate tun2socks process
	if w.tun2socksCmd != nil && w.tun2socksCmd.Process != nil {
		_ = w.tun2socksCmd.Process.Kill()
		_ = w.tun2socksCmd.Wait()
		w.tun2socksCmd = nil
	}

	// 4. Clean interface address
	if w.Name != "" {
		_ = runHidden("netsh", "interface", "ip", "delete", "address", fmt.Sprintf("name=\"%s\"", w.Name), "addr="+w.tunIP)
	}

	return nil
}

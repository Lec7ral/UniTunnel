//go:build windows

package tun

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"unitunnel/pkg/appstate"
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
	tunGw            string
	routesConfigured bool
	routes           *routesState
	closed           bool
	mu               sync.Mutex
}

// CleanStaleRoutes is defined in routes_state_windows.go: it deletes only the
// routes a previous UniTunnel session registered, never arbitrary subnets.

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
		routes:         &routesState{},
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

	// 4. Launch tun2socks windowless with working directory set to application directory
	socksPort := cfg.LocalSocksPort
	if socksPort == 0 {
		socksPort = 1080
	}
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)

	cmd := exec.Command(tun2socksPath,
		"-device", fmt.Sprintf("tun://%s", adapterName),
		"-proxy", socksURL,
		"-loglevel", "error",
	)
	cmd.Dir = exeDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}

	if err := cmd.Start(); err != nil {
		// Fallback with standard "-device wintun" syntax
		cmd = exec.Command(tun2socksPath,
			"-device", "wintun",
			"-proxy", socksURL,
			"-loglevel", "error",
		)
		cmd.Dir = exeDir
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
	var detectedAdapter string
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if isInterfacePresent(adapterName) {
			detectedAdapter = adapterName
			break
		}
		if isInterfacePresent("wintun") {
			detectedAdapter = "wintun"
			break
		}
	}

	if detectedAdapter == "" {
		return nil, fmt.Errorf("el adaptador Wintun tardó demasiado en responder o requiere privilegios de Administrador (verifica que wintun.dll esté en %s)", exeDir)
	}

	adapterName = detectedAdapter
	dev.Name = detectedAdapter
	dev.Interface = detectedAdapter
	tunGw := "10.254.1.1"
	dev.tunGw = tunGw

	// 6. Assign static IP address and virtual gateway to Wintun adapter
	_ = runHidden("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		tunIP,
		tunMask,
		tunGw,
		"1",
	)

	time.Sleep(250 * time.Millisecond)

	// 7. Configure DNS servers on Wintun adapter pointing to our local DoH resolver
	_ = runHidden("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		"static",
		"127.0.0.1",
		"validate=no",
	)
	_ = runHidden("netsh", "interface", "ip", "add", "dns",
		fmt.Sprintf("name=\"%s\"", adapterName),
		tunIP,
		"index=2",
		"validate=no",
	)
	// Prioritize Wintun interface over all others for DNS resolution and default routing
	_ = runHidden("netsh", "interface", "ip", "set", "interface", fmt.Sprintf("name=\"%s\"", adapterName), "metric=1")
	_ = runHidden("netsh", "interface", "ipv4", "set", "interface", fmt.Sprintf("name=\"%s\"", adapterName), "metric=1")
	_ = runHidden("ipconfig", "/flushdns")

	// 8. Configure Subnet Routing and bypasses:
	// Only routes Windows actually accepts are recorded, so Close deletes
	// exactly this session's routes and nothing else.
	proxyRouteIP := cfg.ProxyHost
	if proxyRouteIP != "" && net.ParseIP(proxyRouteIP) == nil {
		// `route add` only accepts IPs; resolve a hostname once, here, where the
		// physical DNS still works.
		if addrs, err := net.LookupIP(proxyRouteIP); err == nil {
			for _, addr := range addrs {
				if v4 := addr.To4(); v4 != nil {
					proxyRouteIP = v4.String()
					break
				}
			}
		}
	}
	if proxyRouteIP != "" {
		_ = addRouteAndRecord(dev.routes, proxyRouteIP, "255.255.255.255", gw, "5")
	}

	bypassRoutes := cfg.BypassRoutes
	if len(bypassRoutes) == 0 {
		bypassRoutes = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	}
	for _, cidr := range bypassRoutes {
		dest, mask, ok := cidrToDestMask(cidr)
		if !ok {
			if onLog != nil {
				onLog(fmt.Sprintf("⚠ Ruta de bypass inválida ignorada: %q", cidr))
			}
			continue
		}
		_ = addRouteAndRecord(dev.routes, dest, mask, gw, "5")
	}

	// 8. Direct all public internet traffic (0.0.0.0/1 and 128.0.0.0/1) through virtual TUN gateway
	// (Split default route technique: overrides default 0.0.0.0/0 without deleting physical default gateway)
	err1 := runHidden("route", "add", "0.0.0.0", "mask", "128.0.0.0", tunGw, "metric", "1")
	if err1 != nil {
		err1 = runHidden("netsh", "interface", "ipv4", "add", "route", "0.0.0.0/1", adapterName, tunGw, "metric=1")
	}
	if err1 == nil {
		_ = dev.routes.add(routeSpec{Destination: "0.0.0.0", Mask: "128.0.0.0", Gateway: tunGw})
	}
	err2 := runHidden("route", "add", "128.0.0.0", "mask", "128.0.0.0", tunGw, "metric", "1")
	if err2 != nil {
		err2 = runHidden("netsh", "interface", "ipv4", "add", "route", "128.0.0.0/1", adapterName, tunGw, "metric=1")
	}
	if err2 == nil {
		_ = dev.routes.add(routeSpec{Destination: "128.0.0.0", Mask: "128.0.0.0", Gateway: tunGw})
	}
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

// cidrToDestMask converts "10.0.0.0/8" into the destination/mask pair that
// `route add` expects ("10.0.0.0", "255.0.0.0").
func cidrToDestMask(cidr string) (dest, mask string, ok bool) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ip.To4() == nil {
		return "", "", false
	}
	return ipnet.IP.To4().String(), net.IP(ipnet.Mask).String(), true
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

	// 1. Delete only the routes this session registered (see routes_state_windows.go).
	// Blanket deletes of 10.0.0.0/8 etc. would also destroy routes UniTunnel
	// never created.
	if w.routes != nil {
		w.routes.deleteAll()
	}
	_ = appstate.Remove(routesStateFile)

	// 2. Terminate tun2socks process
	if w.tun2socksCmd != nil && w.tun2socksCmd.Process != nil {
		_ = w.tun2socksCmd.Process.Kill()
		_ = w.tun2socksCmd.Wait()
		w.tun2socksCmd = nil
	}

	// 3. Clean interface address
	if w.Name != "" {
		_ = runHidden("netsh", "interface", "ip", "delete", "address", fmt.Sprintf("name=\"%s\"", w.Name), "addr="+w.tunIP)
	}

	// 4. Clear stale DNS cache
	_ = runHidden("ipconfig", "/flushdns")

	return nil
}

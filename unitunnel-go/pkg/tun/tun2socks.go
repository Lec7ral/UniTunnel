package tun

import (
	"fmt"
	"strings"
	"sync"

	"unitunnel/pkg/config"
	"unitunnel/pkg/tunnel"
)

type Engine struct {
	cfg         *config.Config
	dialer      *tunnel.Dialer
	socksSrv    *tunnel.SOCKS5Server
	httpSrv     *tunnel.HTTPProxyServer
	dohSrv      *tunnel.DoHServer
	wintunDev   *WintunDevice
	running     bool
	mu          sync.Mutex
	OnLog       func(string)
	OnAuthError func(error)
}

func NewEngine(cfg *config.Config, onLog func(string), onAuthError func(error)) *Engine {
	return &Engine{
		cfg:         cfg,
		OnLog:       onLog,
		OnAuthError: onAuthError,
	}
}

// dohEndpoints turns cfg.TunDNS ("1.1.1.1,8.8.8.8") into RFC 8484 endpoints
// (https://1.1.1.1/dns-query, ...), so the resolver list lives in config.json
// instead of being hardcoded inside the DoH server.
func dohEndpoints(cfg *config.Config) []string {
	raw := cfg.TunDNS
	if strings.TrimSpace(raw) == "" {
		raw = "1.1.1.1,8.8.8.8"
	}

	var endpoints []string
	for _, part := range strings.Split(raw, ",") {
		host := strings.TrimSpace(part)
		if host == "" {
			continue
		}
		if strings.HasPrefix(host, "https://") || strings.HasPrefix(host, "http://") {
			endpoints = append(endpoints, host)
			continue
		}
		endpoints = append(endpoints, fmt.Sprintf("https://%s/dns-query", host))
	}
	return endpoints
}

func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.dialer = tunnel.NewDialer(e.cfg)
	e.dialer.OnLog = e.OnLog
	e.dialer.OnAuthError = func(err error) {
		if e.OnAuthError != nil {
			e.OnAuthError(err)
		}
	}

	// 1. Pre-flight verification: test credentials before routing any system traffic
	if e.OnLog != nil {
		e.OnLog(fmt.Sprintf("→ Verificando credenciales con el proxy %s:%d...", e.cfg.ProxyHost, e.cfg.ProxyPort))
	}
	if err := e.dialer.VerifyCredentials(); err != nil {
		return fmt.Errorf("verificación preliminar de credenciales falló: %w", err)
	}

	// 2. Start local SOCKS5 proxy on 127.0.0.1:1080
	socksPort := e.cfg.LocalSocksPort
	if socksPort == 0 {
		socksPort = 1080
	}
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	e.socksSrv = tunnel.NewSOCKS5Server(socksAddr, e.dialer)
	e.socksSrv.OnEvent = e.OnLog

	if err := e.socksSrv.Start(); err != nil {
		return fmt.Errorf("error iniciando SOCKS5 local: %w", err)
	}

	if e.OnLog != nil {
		e.OnLog(fmt.Sprintf("✓ SOCKS5 local activo en %s", socksAddr))
	}

	// 3. Start local HTTP/HTTPS proxy on 127.0.0.1:8081 (used by Windows System Proxy and browsers)
	httpPort := e.cfg.LocalHTTPPort
	if httpPort == 0 {
		httpPort = 8081
	}
	httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	e.httpSrv = tunnel.NewHTTPProxyServer(httpAddr, e.dialer)
	e.httpSrv.OnEvent = e.OnLog

	if err := e.httpSrv.Start(); err != nil {
		e.socksSrv.Stop()
		return fmt.Errorf("error iniciando Proxy HTTP/HTTPS local: %w", err)
	}

	if e.OnLog != nil {
		e.OnLog(fmt.Sprintf("✓ Proxy HTTP/HTTPS local activo en %s", httpAddr))
	}

	// 4. Start RFC 8484 DNS-over-HTTPS bridge on UDP port 53
	// This intercepts UDP DNS lookups from Windows apps and resolves them through Squid (HTTPS 443).
	// In TUN mode DNS is not optional: without a working local resolver every
	// name lookup fails, so abort and roll back the already-started listeners
	// instead of leaving the machine half-configured.
	e.dohSrv = tunnel.NewDoHServer(e.dialer, dohEndpoints(e.cfg))
	e.dohSrv.OnLog = e.OnLog
	if err := e.dohSrv.Start("0.0.0.0:53"); err != nil {
		e.dohSrv = nil
		e.httpSrv.Stop()
		e.httpSrv = nil
		e.socksSrv.Stop()
		e.socksSrv = nil

		if e.cfg.CaptureMode == "tun" {
			return fmt.Errorf("no se pudo iniciar el DNS local en UDP 53 (puerto ocupado por otro servicio): %w", err)
		}
		if e.OnLog != nil {
			e.OnLog(fmt.Sprintf("⚠ Aviso DoH DNS: %v. En modo Proxy del Sistema los nombres los resuelve el propio proxy.", err))
		}
	} else if e.OnLog != nil {
		e.OnLog("✓ Servidor DNS DoH (RFC 8484) activo en UDP 53. Nombres resueltos por HTTPS.")
	}

	// 5. If TUN mode is active, initialize Wintun device
	if e.cfg.CaptureMode == "tun" {
		if e.OnLog != nil {
			e.OnLog("→ Inicializando adaptador Wintun y configurando rutas IP del sistema...")
		}

		dev, err := OpenWintun(e.cfg, e.OnLog)
		if err != nil {
			if e.OnLog != nil {
				e.OnLog(fmt.Sprintf("ℹ Aviso VPN L3: %v", err))
				e.OnLog("→ Redirigiendo tráfico vía Proxy del Sistema Windows para asegurar navegación ininterrumpida.")
			}
		} else {
			e.wintunDev = dev
			if e.OnLog != nil {
				e.OnLog("✓ Adaptador Wintun configurado. Tráfico redirigido al túnel.")
			}
		}
	}

	e.running = true
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	if e.wintunDev != nil {
		_ = e.wintunDev.Close()
		e.wintunDev = nil
		if e.OnLog != nil {
			e.OnLog("✓ Rutas y adaptador Wintun restaurados.")
		}
	}

	if e.dohSrv != nil {
		e.dohSrv.Stop()
		e.dohSrv = nil
	}

	if e.httpSrv != nil {
		e.httpSrv.Stop()
		e.httpSrv = nil
	}

	if e.socksSrv != nil {
		e.socksSrv.Stop()
		e.socksSrv = nil
	}

	e.running = false
	if e.OnLog != nil {
		e.OnLog("✓ Servicio UniTunnel detenido.")
	}
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

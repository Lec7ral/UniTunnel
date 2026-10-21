package tun

import (
	"fmt"
	"sync"

	"unitunnel/pkg/config"
	"unitunnel/pkg/tunnel"
)

type Engine struct {
	cfg         *config.Config
	dialer      *tunnel.Dialer
	socksSrv    *tunnel.SOCKS5Server
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
	socksAddr := fmt.Sprintf("127.0.0.1:%d", e.cfg.LocalSocksPort)
	e.socksSrv = tunnel.NewSOCKS5Server(socksAddr, e.dialer)
	e.socksSrv.OnEvent = e.OnLog

	if err := e.socksSrv.Start(); err != nil {
		return fmt.Errorf("error iniciando SOCKS5 local: %w", err)
	}

	if e.OnLog != nil {
		e.OnLog(fmt.Sprintf("✓ SOCKS5 local activo en %s", socksAddr))
	}

	// 3. If TUN mode is active, initialize Wintun device and routing
	if e.cfg.CaptureMode == "tun" {
		if e.OnLog != nil {
			e.OnLog("→ Inicializando adaptador Wintun y configurando rutas IP del sistema...")
		}

		dev, err := OpenWintun(e.cfg)
		if err != nil {
			e.socksSrv.Stop()
			return fmt.Errorf("error al abrir Wintun: %w", err)
		}
		e.wintunDev = dev

		if e.OnLog != nil {
			e.OnLog("✓ Adaptador Wintun configurado. Tráfico redirigido al túnel.")
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

package tunnel

import (
	"fmt"
	"sync"

	"unitunnel/pkg/config"
)

type Engine struct {
	cfg         *config.Config
	dialer      *Dialer
	socksServer *SOCKS5Server
	httpServer  *HTTPProxyServer
	mu          sync.Mutex
	running     bool
}

func NewEngine(cfg *config.Config) *Engine {
	dialer := NewDialer(cfg)
	socksAddr := fmt.Sprintf("127.0.0.1:%d", cfg.LocalSocksPort)
	httpAddr := fmt.Sprintf("127.0.0.1:%d", cfg.LocalHTTPPort)

	socksServer := NewSOCKS5Server(socksAddr, dialer)
	httpServer := NewHTTPProxyServer(httpAddr, dialer)

	return &Engine{
		cfg:         cfg,
		dialer:      dialer,
		socksServer: socksServer,
		httpServer:  httpServer,
	}
}

func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	if err := e.socksServer.Start(); err != nil {
		return err
	}

	if err := e.httpServer.Start(); err != nil {
		e.socksServer.Stop()
		return err
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

	e.running = false
	if e.socksServer != nil {
		e.socksServer.Stop()
	}
	if e.httpServer != nil {
		e.httpServer.Stop()
	}
}

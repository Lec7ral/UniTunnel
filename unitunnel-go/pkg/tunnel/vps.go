package tunnel

import (
	"crypto/tls"
	"fmt"
	"net"

	"unitunnel/pkg/config"
)

// VPSTunnel bridges connections to an external personal VPS over HTTP CONNECT
// Useful when full UDP (Voice, Discord, Gaming) or non-standard ports are required
type VPSTunnel struct {
	cfg     *config.Config
	dialer  *Dialer
	running bool
}

func NewVPSTunnel(cfg *config.Config, dialer *Dialer) *VPSTunnel {
	return &VPSTunnel{
		cfg:    cfg,
		dialer: dialer,
	}
}

// DialVPS establishes an encrypted TLS or WebSocket tunnel to user's VPS through university proxy
func (v *VPSTunnel) DialVPS() (net.Conn, error) {
	if !v.cfg.VPSEnabled || v.cfg.VPSServer == "" {
		return nil, fmt.Errorf("VPS no configurado o deshabilitado")
	}

	// 1. Dial VPS via HTTP CONNECT to proxy 10.0.0.1:8080
	rawConn, err := v.dialer.Dial("tcp", v.cfg.VPSServer)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con el VPS a través del proxy: %w", err)
	}

	// 2. Perform TLS handshake over the CONNECT tunnel
	host, _, err := net.SplitHostPort(v.cfg.VPSServer)
	if err != nil {
		host = v.cfg.VPSServer
	}

	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	})

	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("error en apretón TLS con el VPS: %w", err)
	}

	return tlsConn, nil
}

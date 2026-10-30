package tunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// DoHServer runs a high-performance local DNS server on UDP port 53.
// It intercepts standard DNS queries (UDP 53) from Windows applications and
// resolves them over HTTPS (RFC 8484) through the authenticated Squid proxy.
type DoHServer struct {
	dialer    *Dialer
	conn      net.PacketConn
	running   bool
	mu        sync.Mutex
	client    *http.Client
	Endpoints []string
	OnLog     func(string)
}

func NewDoHServer(dialer *Dialer, endpoints []string) *DoHServer {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	if len(endpoints) == 0 {
		// Only used when the caller does not pass cfg.TunDNS.
		endpoints = []string{
			"https://1.1.1.1/dns-query",
			"https://dns.google/dns-query",
		}
	}

	return &DoHServer{
		dialer:    dialer,
		Endpoints: endpoints,
		client: &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		},
	}
}

// Start begins listening on the specified UDP address (e.g. "0.0.0.0:53" or "10.254.1.2:53")
func (s *DoHServer) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		// Fallback to 0.0.0.0:53 if specific IP bind fails
		pc, err = net.ListenPacket("udp", "0.0.0.0:53")
		if err != nil {
			return fmt.Errorf("no se pudo abrir el servidor DNS local en %s: %w", addr, err)
		}
	}

	s.conn = pc
	s.running = true

	go s.serve()
	return nil
}

func (s *DoHServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running && s.conn != nil {
		s.running = false
		_ = s.conn.Close()
		s.conn = nil
	}
}

func (s *DoHServer) serve() {
	buf := make([]byte, 4096)
	for {
		s.mu.Lock()
		if !s.running || s.conn == nil {
			s.mu.Unlock()
			return
		}
		conn := s.conn
		s.mu.Unlock()

		n, clientAddr, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}

		query := make([]byte, n)
		copy(query, buf[:n])

		go s.handleQuery(query, clientAddr)
	}
}

func (s *DoHServer) handleQuery(query []byte, clientAddr net.Addr) {
	// Try every resolver configured in cfg.TunDNS (Cloudflare, Google, ...)
	// instead of hardcoding them here.
	var reply []byte
	var err error
	for _, endpoint := range s.Endpoints {
		reply, err = s.resolveDoH(endpoint, query)
		if err == nil && len(reply) > 0 {
			break
		}
	}

	if err != nil || len(reply) == 0 {
		return
	}

	// Register IP -> Domain mapping in cache so TCP connections to these IPs know their real domain
	ParseAndRecordDNSResponse(reply)

	s.mu.Lock()
	if s.running && s.conn != nil {
		_, _ = s.conn.WriteTo(reply, clientAddr)
	}
	s.mu.Unlock()
}

func (s *DoHServer) resolveDoH(endpoint string, query []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(query))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

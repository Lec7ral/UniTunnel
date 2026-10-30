package tunnel

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"unitunnel/pkg/traffic"
)

// HTTPProxyServer runs a local HTTP/HTTPS CONNECT proxy server on 127.0.0.1:8081
// to service Windows System Proxy, browsers, and standard web clients.
type HTTPProxyServer struct {
	addr     string
	dialer   *Dialer
	listener net.Listener
	mu       sync.Mutex
	running  bool
	OnEvent  func(msg string)
}

func NewHTTPProxyServer(addr string, dialer *Dialer) *HTTPProxyServer {
	return &HTTPProxyServer{
		addr:   addr,
		dialer: dialer,
	}
}

func (s *HTTPProxyServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el puerto local HTTP %s: %w", s.addr, err)
	}

	s.listener = l
	s.running = true

	go s.acceptLoop()
	return nil
}

func (s *HTTPProxyServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running && s.listener != nil {
		s.running = false
		_ = s.listener.Close()
	}
}

func (s *HTTPProxyServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleClient(conn)
	}
}

func (s *HTTPProxyServer) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	if s.dialer != nil {
		locked, _ := s.dialer.IsAuthLocked()
		if locked {
			_, _ = clientConn.Write([]byte("HTTP/1.1 503 Service Unavailable\r\n\r\nUniTunnel: Auth Locked\r\n"))
			return
		}
	}

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	// Case 1: HTTPS CONNECT tunneling (used by browsers for all HTTPS traffic)
	if req.Method == http.MethodConnect {
		target := req.Host
		if !strings.Contains(target, ":") {
			target += ":443"
		}

		var remoteConn net.Conn
		var err error
		if IsCampusBypass(req.Host) {
			remoteConn, err = net.DialTimeout("tcp", target, 10*time.Second)
			if err != nil && s.OnEvent != nil {
				s.OnEvent(fmt.Sprintf("[Bypass Intranet UCI] Error conectando directo a %s: %v", target, err))
			}
		} else {
			remoteConn, err = s.dialer.Dial("tcp", target)
		}
		if err != nil {
			_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer remoteConn.Close()

		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		if err != nil {
			return
		}

		traffic.CountingPipe(clientConn, remoteConn)
		return
	}

	// Case 2: Plain HTTP requests (GET, POST, etc.)
	if IsCampusBypass(req.Host) {
		// Forward campus request directly without going through Squid
		directTransport := &http.Transport{
			Proxy: nil,
			ResponseHeaderTimeout: 15 * time.Second,
		}
		outReq := req.Clone(req.Context())
		if outReq.URL.Scheme == "" {
			outReq.URL.Scheme = "http"
		}
		if outReq.URL.Host == "" {
			outReq.URL.Host = req.Host
		}
		outReq.RequestURI = ""
		resp, err := directTransport.RoundTrip(outReq)
		if err != nil {
			if s.OnEvent != nil {
				s.OnEvent(fmt.Sprintf("[Bypass Intranet UCI] HTTP %s → %v", req.Host, err))
			}
			_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer resp.Body.Close()

		if err := resp.Write(clientConn); err != nil {
			return
		}
		return
	}

	// Squid denies CONNECT to port 80, so external HTTP is forwarded with an
	// absolute URI via the authenticated dialer.
	resp, err := s.dialer.DoHTTP(req)
	if err != nil {
		if s.OnEvent != nil {
			s.OnEvent(fmt.Sprintf("HTTP %s → %v", req.Host, err))
		}
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer resp.Body.Close()

	if err := resp.Write(clientConn); err != nil {
		return
	}
}

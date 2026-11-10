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

	// Case 1: HTTPS CONNECT tunneling (used by browsers, Psiphon, and HTTPS clients)
	if req.Method == http.MethodConnect {
		target := req.Host
		if !strings.Contains(target, ":") {
			target += ":443"
		}

		host, portStr, pErr := net.SplitHostPort(target)
		if pErr != nil {
			host = req.Host
			portStr = "443"
		}

		// Step 1: If destination is an IP address, check DNS cache mapping
		if net.ParseIP(host) != nil {
			if dom := LookupDomainByIP(host); dom != "" {
				host = dom
				target = net.JoinHostPort(host, portStr)
			}
		}

		// Campus bypass for internal UCI intranet
		if IsCampusBypass(host) {
			remoteConn, err := net.DialTimeout("tcp", target, 10*time.Second)
			if err != nil {
				if s.OnEvent != nil {
					s.OnEvent(fmt.Sprintf("[Bypass Intranet UCI] Error conectando directo a %s: %v", target, err))
				}
				_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
				return
			}
			defer remoteConn.Close()

			if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
				return
			}
			traffic.CountingPipe(clientConn, remoteConn)
			return
		}

		// Step 2: If port is 443 and host is still a numeric IP, perform TLS SNI sniffing.
		// Squid at UCI blocks numeric IPs (numeric_IPs ACL). By acknowledging the CONNECT tunnel
		// to the client, the client (Psiphon, browser) sends its TLS ClientHello. We extract
		// the SNI domain name, rewrite target to that domain, and dial Squid successfully!
		if portStr == "443" && net.ParseIP(host) != nil {
			if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
				return
			}

			_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			peekBuf := make([]byte, 4096)
			n, err := reader.Read(peekBuf)
			_ = clientConn.SetReadDeadline(time.Time{})
			if err != nil || n == 0 {
				return
			}

			clientData := peekBuf[:n]
			sni := ExtractSNI(clientData)
			if sni != "" {
				target = net.JoinHostPort(sni, "443")
				RecordDNSMapping(host, sni)
				if s.OnEvent != nil {
					s.OnEvent(fmt.Sprintf("✓ [SNI Sniff HTTP] IP %s traducida a '%s'", host, sni))
				}
			}

			remoteConn, err := s.dialer.Dial("tcp", target)
			if err != nil {
				if s.OnEvent != nil {
					s.OnEvent(fmt.Sprintf("[Proxy HTTP] Falló conexión túnel a %s: %v", target, err))
				}
				return
			}
			defer remoteConn.Close()

			// Forward the initial TLS ClientHello to destination
			if _, err := remoteConn.Write(clientData); err != nil {
				return
			}

			traffic.CountingPipe(clientConn, remoteConn)
			return
		}

		// Step 3: Standard CONNECT (Domain name already known, or non-numeric destination)
		remoteConn, err := s.dialer.Dial("tcp", target)
		if err != nil {
			if s.OnEvent != nil {
				s.OnEvent(fmt.Sprintf("[Proxy HTTP] Error CONNECT hacia %s: %v", target, err))
			}
			_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer remoteConn.Close()

		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
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

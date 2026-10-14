package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
)

// SOCKS5Server runs a local SOCKS5 server that dials out via the authenticated HTTP CONNECT dialer
type SOCKS5Server struct {
	addr     string
	dialer   *Dialer
	listener net.Listener
	mu       sync.Mutex
	running  bool
	OnEvent  func(msg string)
}

func NewSOCKS5Server(addr string, dialer *Dialer) *SOCKS5Server {
	return &SOCKS5Server{
		addr:   addr,
		dialer: dialer,
	}
}

func (s *SOCKS5Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el puerto local SOCKS5 %s: %w", s.addr, err)
	}

	s.listener = l
	s.running = true

	go s.acceptLoop()
	return nil
}

func (s *SOCKS5Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running && s.listener != nil {
		s.running = false
		_ = s.listener.Close()
	}
}

func (s *SOCKS5Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleClient(conn)
	}
}

func (s *SOCKS5Server) handleClient(conn net.Conn) {
	defer conn.Close()

	// Handshake
	// Client: [VER, NMETHODS, METHODS...]
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 {
		return // Not SOCKS5
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}

	// Server: [VER (0x05), METHOD (0x00: No Auth)]
	_, err := conn.Write([]byte{0x05, 0x00})
	if err != nil {
		return
	}

	// Request: [VER (0x05), CMD, RSV, ATYP, DST.ADDR, DST.PORT]
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return
	}

	// CMD 0x01 = CONNECT
	if req[1] != 0x01 {
		// Command not supported
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var host string
	switch req[3] {
	case 0x01: // IPv4 (4 bytes)
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 0x03: // Domain name (1 byte length + domain)
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		domain := make([]byte, int(lenBuf[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return
		}
		host = string(domain)
	case 0x04: // IPv6 (16 bytes)
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	default:
		return
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBuf)
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))

	// Dial remote destination via university proxy
	remoteConn, err := s.dialer.Dial("tcp", target)
	if err != nil {
		// Connection failed
		_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		if s.OnEvent != nil {
			s.OnEvent(fmt.Sprintf("[SOCKS5 Error] Falló túnel a %s: %v", target, err))
		}
		return
	}
	defer remoteConn.Close()

	// Success response: [VER (0x05), REP (0x00), RSV (0x00), ATYP (0x01), BND.ADDR (0.0.0.0), BND.PORT (0)]
	_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	if err != nil {
		return
	}

	if s.OnEvent != nil {
		s.OnEvent(fmt.Sprintf("[Túnel OK] Conectado a %s a través de 10.0.0.1:8080", target))
	}

	Pipe(conn, remoteConn)
}

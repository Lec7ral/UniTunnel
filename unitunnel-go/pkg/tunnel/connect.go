package tunnel

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"unitunnel/pkg/auth"
	"unitunnel/pkg/config"
)

// Dialer connects to remote destinations via the upstream authenticated HTTP proxy
type Dialer struct {
	cfg        *config.Config
	authScheme auth.AuthType
	mu         sync.RWMutex
}

func NewDialer(cfg *config.Config) *Dialer {
	return &Dialer{
		cfg:        cfg,
		authScheme: auth.AuthType(cfg.AuthType),
	}
}

// Dial connects to target (e.g. "github.com:443") through proxy (e.g. "10.0.0.1:8080")
func (d *Dialer) Dial(network, targetAddr string) (net.Conn, error) {
	proxyAddr := fmt.Sprintf("%s:%d", d.cfg.ProxyHost, d.cfg.ProxyPort)

	conn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con el proxy universitario %s: %w", proxyAddr, err)
	}

	// Step 1: Initial CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: UniTunnel/1.0\r\nProxy-Connection: keep-alive\r\n", targetAddr, targetAddr)

	// If scheme is basic or user specified credentials
	if d.cfg.Username != "" && d.authScheme == auth.AuthTypeBasic {
		cred := base64.StdEncoding.EncodeToString([]byte(d.cfg.Username + ":" + d.cfg.Password))
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", cred)
	} else if d.cfg.Username != "" && d.authScheme == auth.AuthTypeNTLM {
		type1 := auth.CreateNegotiateMessage(d.cfg.Domain, "UNITUNNEL-PC")
		connectReq += fmt.Sprintf("Proxy-Authorization: NTLM %s\r\n", type1)
	}
	connectReq += "\r\n"

	_, err = conn.Write([]byte(connectReq))
	if err != nil {
		conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Check if already 200 OK (no auth required or Basic worked immediately)
	if strings.Contains(respLine, "200") {
		// Read remaining response headers until empty line
		for {
			line, err := reader.ReadString('\n')
			if err != nil || line == "\r\n" || line == "\n" {
				break
			}
		}
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}

	// If 407 Proxy Authentication Required, inspect headers for challenge
	if strings.Contains(respLine, "407") {
		var ntlmChallenge string
		hasNTLM := false
		hasBasic := false

		for {
			line, err := reader.ReadString('\n')
			if err != nil || line == "\r\n" || line == "\n" {
				break
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "proxy-authenticate:") {
				val := strings.TrimSpace(line[len("proxy-authenticate:"):])
				if strings.HasPrefix(strings.ToLower(val), "ntlm") {
					hasNTLM = true
					parts := strings.Fields(val)
					if len(parts) >= 2 {
						ntlmChallenge = parts[1]
					}
				} else if strings.Contains(strings.ToLower(val), "basic") {
					hasBasic = true
				}
			}
		}

		// If NTLM challenge received (Type 2), send Type 3
		if hasNTLM && ntlmChallenge != "" {
			d.authScheme = auth.AuthTypeNTLM
			chBytes, err := auth.ParseChallengeMessage(ntlmChallenge)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("error decodificando challenge NTLM: %w", err)
			}

			type3 := auth.CreateAuthenticateMessage(d.cfg.Domain, d.cfg.Username, d.cfg.Password, "UNITUNNEL-PC", chBytes)
			type3Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: NTLM %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, type3)

			_, err = conn.Write([]byte(type3Req))
			if err != nil {
				conn.Close()
				return nil, err
			}

			resp2, err := reader.ReadString('\n')
			if err != nil {
				conn.Close()
				return nil, err
			}

			if strings.Contains(resp2, "200") {
				for {
					line, err := reader.ReadString('\n')
					if err != nil || line == "\r\n" || line == "\n" {
						break
					}
				}
				return &bufferedConn{Conn: conn, reader: reader}, nil
			}
			conn.Close()
			return nil, fmt.Errorf("NTLM Type-3 rechazado por el proxy: %s", strings.TrimSpace(resp2))
		}

		// If proxy only asked for basic or Type-1 needs to be re-sent
		if hasBasic && d.cfg.Username != "" {
			d.authScheme = auth.AuthTypeBasic
			cred := base64.StdEncoding.EncodeToString([]byte(d.cfg.Username + ":" + d.cfg.Password))
			req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, cred)
			_, err = conn.Write([]byte(req))
			if err != nil {
				conn.Close()
				return nil, err
			}
			resp3, err := reader.ReadString('\n')
			if err == nil && strings.Contains(resp3, "200") {
				for {
					l, e := reader.ReadString('\n')
					if e != nil || l == "\r\n" || l == "\n" {
						break
					}
				}
				return &bufferedConn{Conn: conn, reader: reader}, nil
			}
		}

		conn.Close()
		return nil, errors.New("autenticacion rechazada por el proxy 10.0.0.1:8080 (verifica usuario y contrasena)")
	}

	conn.Close()
	return nil, fmt.Errorf("proxy retorno estado inesperado: %s", strings.TrimSpace(respLine))
}

// Pipe forwards bidirectional traffic between two connections
func Pipe(src, dst net.Conn) {
	defer src.Close()
	defer dst.Close()

	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(dst, src)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(src, dst)
		errChan <- err
	}()
	<-errChan
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (b *bufferedConn) Read(p []byte) (n int, err error) {
	if b.reader.Buffered() > 0 {
		return b.reader.Read(p)
	}
	return b.Conn.Read(p)
}

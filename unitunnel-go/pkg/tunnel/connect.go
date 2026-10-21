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
	cfg            *config.Config
	authScheme     auth.AuthType
	mu             sync.RWMutex
	authLocked     bool
	authLockReason string
	OnAuthError    func(err error)
	OnLog          func(msg string)
}

func NewDialer(cfg *config.Config) *Dialer {
	scheme := auth.AuthType(cfg.AuthType)
	if scheme == "" {
		scheme = auth.AuthTypeNTLM
	}
	return &Dialer{
		cfg:        cfg,
		authScheme: scheme,
	}
}

// ProxyAddress returns the configured host:port of the proxy
func (d *Dialer) ProxyAddress() string {
	return fmt.Sprintf("%s:%d", d.cfg.ProxyHost, d.cfg.ProxyPort)
}

// ResetAuthLock clears any anti-lockout trip when user edits credentials or initiates connect
func (d *Dialer) ResetAuthLock() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.authLocked = false
	d.authLockReason = ""
}

// IsAuthLocked checks if authentication failed and auto-retries are blocked
func (d *Dialer) IsAuthLocked() (bool, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.authLocked, d.authLockReason
}

func (d *Dialer) tripAuthLock(err error, reason string) {
	d.mu.Lock()
	d.authLocked = true
	d.authLockReason = reason
	d.mu.Unlock()

	if d.OnLog != nil {
		d.OnLog(fmt.Sprintf("🛡 [PROTECCIÓN ACTIVA] %s", reason))
		d.OnLog("🛡 [PROTECCIÓN ACTIVA] Reintentos cancelados para prevenir el bloqueo de tu cuenta institucional.")
	}

	if d.OnAuthError != nil {
		go d.OnAuthError(err)
	}
}

// VerifyCredentials performs a single controlled pre-flight check to test proxy authentication
// without routing system traffic or flooding requests.
func (d *Dialer) VerifyCredentials() error {
	d.ResetAuthLock()
	proxyAddr := d.ProxyAddress()

	conn, err := net.DialTimeout("tcp", proxyAddr, 8*time.Second)
	if err != nil {
		return fmt.Errorf("no se pudo alcanzar el proxy en %s: %w", proxyAddr, err)
	}
	defer conn.Close()

	account := d.cfg.GetActiveAccount()
	if account == nil || account.Username == "" {
		return errors.New("no hay credenciales de usuario configuradas")
	}

	cleanDomain, cleanUser := auth.SplitDomainUser(account.Domain, account.Username)
	if d.OnLog != nil {
		domStr := ""
		if cleanDomain != "" {
			domStr = fmt.Sprintf(" (Dominio: %s)", cleanDomain)
		}
		d.OnLog(fmt.Sprintf("[AUTH-PRECHECK] Verificando usuario '%s'%s con %s...", cleanUser, domStr, proxyAddr))
	}

	// Test CONNECT to standard target
	testTarget := "www.google.com:443"
	tunnelConn, err := d.negotiateConnect(conn, testTarget, account)
	if err != nil {
		d.tripAuthLock(err, "Credenciales rechazadas en verificación preliminar")
		return err
	}
	_ = tunnelConn.Close()

	if d.OnLog != nil {
		d.OnLog(fmt.Sprintf("✓ [AUTH-OK] Proxy %s autenticó exitosamente al usuario '%s'.", proxyAddr, cleanUser))
	}
	return nil
}

// Dial connects to target (e.g. "github.com:443") through upstream proxy
func (d *Dialer) Dial(network, targetAddr string) (net.Conn, error) {
	locked, reason := d.IsAuthLocked()
	if locked {
		return nil, fmt.Errorf("conexión bloqueada por seguridad: %s (detenido para proteger tu cuenta de dominio)", reason)
	}

	proxyAddr := d.ProxyAddress()
	conn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con el proxy universitario %s: %w", proxyAddr, err)
	}

	account := d.cfg.GetActiveAccount()
	tunnelConn, err := d.negotiateConnect(conn, targetAddr, account)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return tunnelConn, nil
}

// negotiateConnect executes the HTTP CONNECT handshake including NTLM/Basic negotiation
func (d *Dialer) negotiateConnect(conn net.Conn, targetAddr string, account *config.UserAccount) (net.Conn, error) {
	proxyAddr := d.ProxyAddress()
	cleanDomain := ""
	cleanUser := ""
	if account != nil {
		cleanDomain, cleanUser = auth.SplitDomainUser(account.Domain, account.Username)
	}

	// Step 1: Initial CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: UniTunnel/2.4\r\nProxy-Connection: keep-alive\r\n", targetAddr, targetAddr)

	useNTLM := (d.authScheme == auth.AuthTypeNTLM || d.authScheme == "auto" || d.authScheme == "")
	useBasic := (d.authScheme == auth.AuthTypeBasic)

	if account != nil && cleanUser != "" {
		if useBasic {
			cred := base64.StdEncoding.EncodeToString([]byte(cleanUser + ":" + account.Password))
			connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", cred)
		} else if useNTLM {
			type1 := auth.CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
			connectReq += fmt.Sprintf("Proxy-Authorization: NTLM %s\r\n", type1)
		}
	}
	connectReq += "\r\n"

	_, err := conn.Write([]byte(connectReq))
	if err != nil {
		return nil, fmt.Errorf("error al enviar petición CONNECT a %s: %w", proxyAddr, err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta inicial del proxy %s: %w", proxyAddr, err)
	}

	// 200 OK: Tunnel established
	if strings.Contains(statusLine, "200") {
		for {
			line, err := reader.ReadString('\n')
			if err != nil || line == "\r\n" || line == "\n" {
				break
			}
		}
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}

	// 407: Proxy Authentication Required
	if strings.Contains(statusLine, "407") {
		var ntlmChallenge string
		var squidError string
		var proxyServer string
		hasNTLM := false
		hasBasic := false

		for {
			line, err := reader.ReadString('\n')
			if err != nil || line == "\r\n" || line == "\n" {
				break
			}
			trimmed := strings.TrimSpace(line)
			lower := strings.ToLower(trimmed)

			if strings.HasPrefix(lower, "proxy-authenticate:") {
				val := strings.TrimSpace(trimmed[len("proxy-authenticate:"):])
				if strings.HasPrefix(strings.ToLower(val), "ntlm") {
					hasNTLM = true
					parts := strings.Fields(val)
					if len(parts) >= 2 {
						ntlmChallenge = parts[1]
					}
				} else if strings.Contains(strings.ToLower(val), "basic") {
					hasBasic = true
				}
			} else if strings.HasPrefix(lower, "x-squid-error:") {
				squidError = strings.TrimSpace(trimmed[len("x-squid-error:"):])
			} else if strings.HasPrefix(lower, "server:") {
				proxyServer = strings.TrimSpace(trimmed[len("server:"):])
			}
		}

		// Case 1: Proxy returned Type 2 NTLM Challenge
		if hasNTLM && ntlmChallenge != "" && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeNTLM
			chDetails, err := auth.ParseChallengeMessage(ntlmChallenge)
			if err != nil {
				return nil, fmt.Errorf("error decodificando challenge NTLM del proxy: %w", err)
			}

			// If domain was not supplied by user, infer from proxy challenge
			authDomain := cleanDomain
			if authDomain == "" && chDetails.TargetName != "" {
				authDomain = chDetails.TargetName
			}

			type3 := auth.CreateAuthenticateMessage(authDomain, cleanUser, account.Password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
			type3Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: NTLM %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, type3)

			_, err = conn.Write([]byte(type3Req))
			if err != nil {
				return nil, fmt.Errorf("error enviando NTLM Type-3: %w", err)
			}

			statusLine2, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("error leyendo respuesta NTLM Type-3: %w", err)
			}

			if strings.Contains(statusLine2, "200") {
				for {
					line, err := reader.ReadString('\n')
					if err != nil || line == "\r\n" || line == "\n" {
						break
					}
				}
				return &bufferedConn{Conn: conn, reader: reader}, nil
			}

			// Parse Type-3 rejection headers
			type3SquidErr := ""
			for {
				line, err := reader.ReadString('\n')
				if err != nil || line == "\r\n" || line == "\n" {
					break
				}
				lower := strings.ToLower(line)
				if strings.HasPrefix(lower, "x-squid-error:") {
					type3SquidErr = strings.TrimSpace(line[len("x-squid-error:"):])
				}
			}

			errMsg := fmt.Sprintf("autenticación NTLM rechazada por el servidor %s (%s). Código: %s",
				proxyAddr, proxyServer, strings.TrimSpace(statusLine2))
			if type3SquidErr != "" {
				errMsg += fmt.Sprintf(" · Squid: %s", type3SquidErr)
			}
			authErr := fmt.Errorf("%s (verifica usuario, dominio o contraseña)", errMsg)
			d.tripAuthLock(authErr, "Credenciales NTLM rechazadas por el proxy")
			return nil, authErr
		}

		// Case 2: Proxy offers NTLM but did not receive Type 1 yet (auto mode or unauthenticated first probe)
		if hasNTLM && ntlmChallenge == "" && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeNTLM
			type1 := auth.CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
			type1Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: NTLM %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, type1)

			_, err = conn.Write([]byte(type1Req))
			if err != nil {
				return nil, fmt.Errorf("error enviando NTLM Type-1: %w", err)
			}

			statusLineType1, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("error leyendo respuesta Type-1: %w", err)
			}

			if strings.Contains(statusLineType1, "407") {
				var chB64 string
				for {
					l, e := reader.ReadString('\n')
					if e != nil || l == "\r\n" || l == "\n" {
						break
					}
					t := strings.TrimSpace(l)
					if strings.HasPrefix(strings.ToLower(t), "proxy-authenticate:") {
						val := strings.TrimSpace(t[len("proxy-authenticate:"):])
						parts := strings.Fields(val)
						if len(parts) >= 2 && strings.EqualFold(parts[0], "ntlm") {
							chB64 = parts[1]
						}
					}
				}

				if chB64 != "" {
					chDetails, err := auth.ParseChallengeMessage(chB64)
					if err == nil {
						authDomain := cleanDomain
						if authDomain == "" && chDetails.TargetName != "" {
							authDomain = chDetails.TargetName
						}
						type3 := auth.CreateAuthenticateMessage(authDomain, cleanUser, account.Password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
						type3Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: NTLM %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, type3)
						_, _ = conn.Write([]byte(type3Req))

						statusFinal, err := reader.ReadString('\n')
						if err == nil && strings.Contains(statusFinal, "200") {
							for {
								l, e := reader.ReadString('\n')
								if e != nil || l == "\r\n" || l == "\n" {
									break
								}
							}
							return &bufferedConn{Conn: conn, reader: reader}, nil
						}
					}
				}
			}
		}

		// Case 3: Basic authentication fallback
		if hasBasic && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeBasic
			cred := base64.StdEncoding.EncodeToString([]byte(cleanUser + ":" + account.Password))
			req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\nProxy-Connection: keep-alive\r\n\r\n", targetAddr, targetAddr, cred)
			_, err = conn.Write([]byte(req))
			if err != nil {
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

		errDesc := fmt.Sprintf("autenticación rechazada por el proxy %s (servidor: %s)", proxyAddr, proxyServer)
		if squidError != "" {
			errDesc += fmt.Sprintf(" [Squid: %s]", squidError)
		}
		authErr := fmt.Errorf("%s. Revisa tus credenciales.", errDesc)
		d.tripAuthLock(authErr, "Proxy rechazó credenciales")
		return nil, authErr
	}

	return nil, fmt.Errorf("proxy retornó estado inesperado: %s", strings.TrimSpace(statusLine))
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

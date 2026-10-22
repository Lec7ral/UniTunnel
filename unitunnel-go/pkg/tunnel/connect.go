package tunnel

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
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
	inPrecheck     bool
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
	alreadyLocked := d.authLocked
	d.authLocked = true
	d.authLockReason = reason
	d.mu.Unlock()

	if !alreadyLocked {
		if d.OnLog != nil {
			d.OnLog(fmt.Sprintf("🛡 [PROTECCIÓN ACTIVA] %s", reason))
			d.OnLog("🛡 [PROTECCIÓN ACTIVA] Reintentos cancelados para prevenir el bloqueo de tu cuenta institucional.")
		}

		if d.OnAuthError != nil && !d.inPrecheck {
			go d.OnAuthError(err)
		}
	}
}

// httpResponse represents a parsed HTTP CONNECT response
type httpResponse struct {
	statusCode    int
	statusText    string
	proto         string
	headers       map[string]string
	ntlmChallenge string
	hasNTLM       bool
	hasBasic      bool
	server        string
	via           string
	squidError    string
	contentLength int
	connection    string
	proxyConn     string
}

// readHTTPResponse reads status line, headers, and drains entity body to keep socket in sync
func readHTTPResponse(reader *bufio.Reader) (*httpResponse, error) {
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	statusLine = strings.TrimSpace(statusLine)
	if statusLine == "" {
		return nil, errors.New("respuesta vacía del servidor proxy")
	}

	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("línea de estado HTTP inválida recibida del proxy: '%s'", statusLine)
	}

	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("código de estado numérico inválido en '%s': %w", statusLine, err)
	}

	statusText := ""
	if len(parts) >= 3 {
		statusText = parts[2]
	}

	resp := &httpResponse{
		statusCode:    code,
		statusText:    statusText,
		proto:         parts[0],
		headers:       make(map[string]string),
		contentLength: -1,
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}

		colonIdx := strings.Index(trimmed, ":")
		if colonIdx > 0 {
			key := strings.ToLower(strings.TrimSpace(trimmed[:colonIdx]))
			val := strings.TrimSpace(trimmed[colonIdx+1:])
			resp.headers[key] = val

			switch key {
			case "server":
				resp.server = val
			case "via":
				resp.via = val
			case "x-squid-error":
				resp.squidError = val
			case "connection":
				resp.connection = strings.ToLower(val)
			case "proxy-connection":
				resp.proxyConn = strings.ToLower(val)
			case "content-length":
				if cl, err := strconv.Atoi(val); err == nil {
					resp.contentLength = cl
				}
			case "proxy-authenticate":
				valLower := strings.ToLower(val)
				if strings.Contains(valLower, "ntlm") {
					resp.hasNTLM = true
					fields := strings.Fields(val)
					if len(fields) >= 2 {
						resp.ntlmChallenge = fields[1]
					}
				}
				if strings.Contains(valLower, "basic") {
					resp.hasBasic = true
				}
			}
		}
	}

	// Drain any response body so the socket stream remains perfectly synchronized
	// CONNECT 200 responses do not contain bodies (the raw TCP tunnel begins immediately)
	if resp.statusCode != 200 {
		if resp.contentLength > 0 {
			_, _ = io.CopyN(io.Discard, reader, int64(resp.contentLength))
		} else if strings.EqualFold(resp.headers["transfer-encoding"], "chunked") {
			for {
				chunkLenLine, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				chunkLenLine = strings.TrimSpace(chunkLenLine)
				chunkLen, err := strconv.ParseInt(chunkLenLine, 16, 64)
				if err != nil || chunkLen <= 0 {
					_, _ = reader.ReadString('\n')
					break
				}
				_, _ = io.CopyN(io.Discard, reader, chunkLen)
				_, _ = reader.ReadString('\n')
			}
		}
	}

	return resp, nil
}

// VerifyCredentials performs a single controlled pre-flight check to test proxy authentication
// without routing system traffic or flooding requests.
func (d *Dialer) VerifyCredentials() error {
	d.ResetAuthLock()
	d.inPrecheck = true
	defer func() { d.inPrecheck = false }()

	proxyAddr := d.ProxyAddress()

	conn, err := net.DialTimeout("tcp", proxyAddr, 8*time.Second)
	if err != nil {
		return fmt.Errorf("no se pudo conectar con el proxy en %s: %w", proxyAddr, err)
	}
	defer conn.Close()

	account := d.cfg.GetActiveAccount()
	if account == nil || strings.TrimSpace(account.Username) == "" {
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

	useNTLM := (d.authScheme == auth.AuthTypeNTLM || d.authScheme == "auto" || d.authScheme == "")
	useBasic := (d.authScheme == auth.AuthTypeBasic)

	reader := bufio.NewReader(conn)

	// Step 1: Initial CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"User-Agent: UniTunnel/2.4\r\n"+
		"Proxy-Connection: keep-alive\r\n"+
		"Connection: keep-alive\r\n", targetAddr, targetAddr)

	if account != nil && cleanUser != "" {
		if useBasic {
			cred := base64.StdEncoding.EncodeToString([]byte(cleanUser + ":" + account.Password))
			connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", cred)
		} else if useNTLM {
			type1 := auth.CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
			connectReq += fmt.Sprintf("Proxy-Authorization: NTLM %s\r\n", type1)
			if d.OnLog != nil && d.inPrecheck {
				d.OnLog(fmt.Sprintf("[AUTH-NTLM] Paso 1: Enviando negociación inicial NTLM (Type-1) hacia %s...", proxyAddr))
			}
		}
	}
	connectReq += "\r\n"

	if _, err := conn.Write([]byte(connectReq)); err != nil {
		return nil, fmt.Errorf("error al enviar petición CONNECT a %s: %w", proxyAddr, err)
	}

	resp1, err := readHTTPResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta inicial del proxy %s: %w", proxyAddr, err)
	}

	// 200 OK: Tunnel established directly (unauthenticated proxy or already authorized)
	if resp1.statusCode == 200 {
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}

	// 407: Proxy Authentication Required
	if resp1.statusCode == 407 {
		// Case 1: Proxy responded to Type-1 with Type-2 Challenge
		if resp1.hasNTLM && resp1.ntlmChallenge != "" && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeNTLM
			chDetails, err := auth.ParseChallengeMessage(resp1.ntlmChallenge)
			if err != nil {
				return nil, fmt.Errorf("error decodificando challenge NTLM del proxy: %w", err)
			}

			// Domain resolution:
			// If server challenge specifies TargetName (e.g., "UCI"), use it if user domain is empty or matches prefix
			authDomain := cleanDomain
			if chDetails.TargetName != "" {
				if authDomain == "" || strings.HasPrefix(strings.ToUpper(authDomain), strings.ToUpper(chDetails.TargetName)) || strings.HasPrefix(strings.ToUpper(chDetails.TargetName), strings.ToUpper(authDomain)) {
					authDomain = chDetails.TargetName
				}
			}

			if d.OnLog != nil && d.inPrecheck {
				serverDesc := resp1.server
				if serverDesc == "" {
					serverDesc = "Squid"
				}
				d.OnLog(fmt.Sprintf("[AUTH-NTLM] Paso 2: Desafío recibido de %s (%s). Dominio detectado: '%s'",
					proxyAddr, serverDesc, authDomain))
				d.OnLog(fmt.Sprintf("[AUTH-NTLM] Paso 3: Enviando credenciales para usuario '%s' (Dominio: '%s')...",
					cleanUser, authDomain))
			}

			type3 := auth.CreateAuthenticateMessage(authDomain, cleanUser, account.Password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
			type3Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n"+
				"Host: %s\r\n"+
				"User-Agent: UniTunnel/2.4\r\n"+
				"Proxy-Connection: keep-alive\r\n"+
				"Connection: keep-alive\r\n"+
				"Proxy-Authorization: NTLM %s\r\n\r\n", targetAddr, targetAddr, type3)

			if _, err := conn.Write([]byte(type3Req)); err != nil {
				return nil, fmt.Errorf("error enviando NTLM Type-3: %w", err)
			}

			resp2, err := readHTTPResponse(reader)
			if err != nil {
				return nil, fmt.Errorf("error leyendo respuesta NTLM Type-3: %w", err)
			}

			if resp2.statusCode == 200 {
				if d.OnLog != nil && d.inPrecheck {
					d.OnLog(fmt.Sprintf("✓ [AUTH-EXITO] ¡Autenticación NTLM exitosa! Túnel establecido con %s.", proxyAddr))
				}
				return &bufferedConn{Conn: conn, reader: reader}, nil
			}

			// Type-3 rejected
			serverDesc := resp2.server
			if serverDesc == "" {
				serverDesc = resp1.server
			}
			if resp2.statusCode == 403 {
				// HTTP 403 Forbidden is an institutional proxy ACL restriction (e.g. CONNECT to port 80 blocked, or blocked domain)
				// It is NOT a credential failure and MUST NOT trip the user account lockout!
				return nil, fmt.Errorf("destino %s bloqueado por política del proxy (HTTP 403 Forbidden)", targetAddr)
			}

			errMsg := fmt.Sprintf("autenticación NTLM rechazada por el proxy %s (%s, HTTP %d)",
				proxyAddr, serverDesc, resp2.statusCode)
			if resp2.squidError != "" {
				errMsg += fmt.Sprintf(" · Error Squid: %s", resp2.squidError)
			}
			authErr := fmt.Errorf("%s (verifica usuario '%s', dominio '%s' o contraseña)", errMsg, cleanUser, authDomain)
			if d.inPrecheck {
				d.tripAuthLock(authErr, "Credenciales NTLM rechazadas por el proxy")
			}
			return nil, authErr
		}

		// Case 2: Proxy offers NTLM but did not receive Type-1 yet
		if resp1.hasNTLM && resp1.ntlmChallenge == "" && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeNTLM
			type1 := auth.CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
			type1Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n"+
				"Host: %s\r\n"+
				"User-Agent: UniTunnel/2.4\r\n"+
				"Proxy-Connection: keep-alive\r\n"+
				"Connection: keep-alive\r\n"+
				"Proxy-Authorization: NTLM %s\r\n\r\n", targetAddr, targetAddr, type1)

			if _, err := conn.Write([]byte(type1Req)); err != nil {
				return nil, fmt.Errorf("error enviando NTLM Type-1: %w", err)
			}

			respT1, err := readHTTPResponse(reader)
			if err != nil {
				return nil, fmt.Errorf("error leyendo respuesta Type-1: %w", err)
			}

			if respT1.statusCode == 407 && respT1.ntlmChallenge != "" {
				chDetails, err := auth.ParseChallengeMessage(respT1.ntlmChallenge)
				if err == nil {
					authDomain := cleanDomain
					if chDetails.TargetName != "" {
						if authDomain == "" || strings.HasPrefix(strings.ToUpper(authDomain), strings.ToUpper(chDetails.TargetName)) || strings.HasPrefix(strings.ToUpper(chDetails.TargetName), strings.ToUpper(authDomain)) {
							authDomain = chDetails.TargetName
						}
					}

					type3 := auth.CreateAuthenticateMessage(authDomain, cleanUser, account.Password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
					type3Req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n"+
						"Host: %s\r\n"+
						"User-Agent: UniTunnel/2.4\r\n"+
						"Proxy-Connection: keep-alive\r\n"+
						"Connection: keep-alive\r\n"+
						"Proxy-Authorization: NTLM %s\r\n\r\n", targetAddr, targetAddr, type3)
					if _, err := conn.Write([]byte(type3Req)); err == nil {
						respFinal, err := readHTTPResponse(reader)
						if err == nil && respFinal.statusCode == 200 {
							if d.OnLog != nil {
								d.OnLog(fmt.Sprintf("✓ [AUTH-EXITO] ¡Autenticación NTLM exitosa! Túnel establecido con %s.", proxyAddr))
							}
							return &bufferedConn{Conn: conn, reader: reader}, nil
						}
					}
				}
			}
		}

		// Case 3: Basic authentication fallback
		if resp1.hasBasic && account != nil && cleanUser != "" {
			d.authScheme = auth.AuthTypeBasic
			cred := base64.StdEncoding.EncodeToString([]byte(cleanUser + ":" + account.Password))
			req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\n"+
				"Host: %s\r\n"+
				"User-Agent: UniTunnel/2.4\r\n"+
				"Proxy-Connection: keep-alive\r\n"+
				"Connection: keep-alive\r\n"+
				"Proxy-Authorization: Basic %s\r\n\r\n", targetAddr, targetAddr, cred)
			if _, err := conn.Write([]byte(req)); err != nil {
				return nil, err
			}
			respBasic, err := readHTTPResponse(reader)
			if err == nil && respBasic.statusCode == 200 {
				if d.OnLog != nil {
					d.OnLog(fmt.Sprintf("✓ [AUTH-EXITO] ¡Autenticación Basic exitosa! Túnel establecido con %s.", proxyAddr))
				}
				return &bufferedConn{Conn: conn, reader: reader}, nil
			}
		}

		errDesc := fmt.Sprintf("autenticación rechazada por el proxy %s (servidor: %s, HTTP %d)",
			proxyAddr, resp1.server, resp1.statusCode)
		if resp1.squidError != "" {
			errDesc += fmt.Sprintf(" [Squid: %s]", resp1.squidError)
		}
		authErr := fmt.Errorf("%s. Revisa tus credenciales.", errDesc)
		d.tripAuthLock(authErr, "Proxy rechazó credenciales")
		return nil, authErr
	}

	errMsg := fmt.Sprintf("proxy rechazó conexión hacia %s (HTTP %d %s)", targetAddr, resp1.statusCode, resp1.statusText)
	if resp1.squidError != "" {
		errMsg += fmt.Sprintf(" [Squid: %s]", resp1.squidError)
	}
	if resp1.statusCode == 403 {
		errMsg += " · El proxy bloqueó este puerto o IP (Squid solo permite puerto 443/HTTPS; en Telegram usa Proxy HTTP hacia 127.0.0.1:8081)"
	}
	return nil, errors.New(errMsg)
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

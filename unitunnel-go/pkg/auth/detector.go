package auth

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeNTLM   AuthType = "ntlm"
	AuthTypeDigest AuthType = "digest"
)

type ProbeResult struct {
	SupportedTypes    []AuthType
	PrimaryType       AuthType
	ProxyServer       string
	ProxyVia          string
	SquidError        string
	Diagnostic        string
	RelevantHeaders   []string

	// Authenticated probe data
	AuthTested       bool
	AuthSuccess      bool
	AuthStatusText   string
	TargetNetBIOS    string
	TargetDNSDomain  string
	TargetServerName string
	Latency          time.Duration
}

// ProbeProxy checks what authentication schemes the upstream proxy requires
func ProbeProxy(proxyAddr string, timeout time.Duration) (*ProbeResult, error) {
	return ProbeProxyWithAuth(proxyAddr, "", "", "", timeout)
}

// ProbeProxyWithAuth probes the proxy and verifies credentials, extracting Active Directory domain info
func ProbeProxyWithAuth(proxyAddr, username, domain, password string, timeout time.Duration) (*ProbeResult, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con el proxy en %s: %w", proxyAddr, err)
	}
	defer conn.Close()

	latency := time.Since(start)

	cleanDomain, cleanUser := SplitDomainUser(domain, username)
	req := "CONNECT www.google.com:443 HTTP/1.1\r\n" +
		"Host: www.google.com:443\r\n" +
		"User-Agent: UniTunnel/2.5\r\n" +
		"Proxy-Connection: keep-alive\r\n"

	if cleanUser != "" {
		type1 := CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
		req += fmt.Sprintf("Proxy-Authorization: NTLM %s\r\n", type1)
	}
	req += "\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		return nil, fmt.Errorf("error enviando sonda al proxy: %w", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta del proxy: %w", err)
	}

	result := &ProbeResult{
		SupportedTypes:  make([]AuthType, 0),
		PrimaryType:     AuthTypeNone,
		ProxyServer:     "Desconocido",
		RelevantHeaders: make([]string, 0),
		Latency:         latency,
	}

	if strings.Contains(statusLine, "200") {
		result.SupportedTypes = append(result.SupportedTypes, AuthTypeNone)
		result.PrimaryType = AuthTypeNone
		result.Diagnostic = "El proxy permite conexiones directas sin autenticación (IP autorizada)."
		result.AuthTested = true
		result.AuthSuccess = true
		result.AuthStatusText = "Conexión directa autorizada (sin requerimiento de credenciales)."
		return result, nil
	}

	var ntlmChallenge string
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "date:") ||
			strings.HasPrefix(lower, "mime-version:") ||
			strings.HasPrefix(lower, "content-type:") ||
			strings.HasPrefix(lower, "content-length:") ||
			strings.HasPrefix(lower, "connection:") {
			continue
		}

		if strings.HasPrefix(lower, "proxy-authenticate:") {
			val := strings.TrimSpace(trimmed[len("proxy-authenticate:"):])
			valLower := strings.ToLower(val)

			if strings.HasPrefix(valLower, "ntlm") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeNTLM)
				parts := strings.Fields(val)
				if len(parts) >= 2 {
					ntlmChallenge = parts[1]
				}
			}
			if strings.Contains(valLower, "basic") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeBasic)
			}
			if strings.Contains(valLower, "digest") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeDigest)
			}
			result.RelevantHeaders = append(result.RelevantHeaders, trimmed)
		} else if strings.HasPrefix(lower, "server:") {
			result.ProxyServer = strings.TrimSpace(trimmed[len("server:"):])
			result.RelevantHeaders = append(result.RelevantHeaders, trimmed)
		} else if strings.HasPrefix(lower, "via:") {
			result.ProxyVia = strings.TrimSpace(trimmed[len("via:"):])
			result.RelevantHeaders = append(result.RelevantHeaders, trimmed)
		} else if strings.HasPrefix(lower, "x-squid-error:") {
			result.SquidError = strings.TrimSpace(trimmed[len("x-squid-error:"):])
			result.RelevantHeaders = append(result.RelevantHeaders, trimmed)
		} else if strings.HasPrefix(lower, "x-cache:") {
			result.RelevantHeaders = append(result.RelevantHeaders, trimmed)
		}
	}

	hasNTLM := false
	hasBasic := false
	for _, t := range result.SupportedTypes {
		if t == AuthTypeNTLM {
			hasNTLM = true
		}
		if t == AuthTypeBasic {
			hasBasic = true
		}
	}

	if hasNTLM {
		result.PrimaryType = AuthTypeNTLM
		result.Diagnostic = "El proxy requiere autenticación NTLM de dominio institucional (Active Directory)."
	} else if hasBasic {
		result.PrimaryType = AuthTypeBasic
		result.Diagnostic = "El proxy requiere autenticación básica (usuario y contraseña estándar)."
	} else if len(result.SupportedTypes) > 0 {
		result.PrimaryType = result.SupportedTypes[0]
		result.Diagnostic = fmt.Sprintf("El proxy requiere autenticación tipo %s.", result.PrimaryType)
	}

	// Parse NTLM challenge if available
	if ntlmChallenge != "" {
		chDetails, err := ParseChallengeMessage(ntlmChallenge)
		if err == nil {
			result.TargetNetBIOS = chDetails.TargetName
			if len(chDetails.TargetInfo) > 0 {
				nbComp := ExtractAvPair(chDetails.TargetInfo, 0x0001)
				if len(nbComp) > 0 {
					result.TargetServerName = strings.TrimRight(fromUnicode(nbComp), "\x00")
				}
				dnsDom := ExtractAvPair(chDetails.TargetInfo, 0x0004)
				if len(dnsDom) > 0 {
					result.TargetDNSDomain = strings.TrimRight(fromUnicode(dnsDom), "\x00")
				}
			}

			// If credentials provided, execute Type-3 handshake test
			if cleanUser != "" && password != "" {
				result.AuthTested = true
				authDomain := cleanDomain
				if result.TargetNetBIOS != "" {
					if authDomain == "" || strings.EqualFold(authDomain, result.TargetNetBIOS) || strings.EqualFold(authDomain, result.TargetDNSDomain) {
						authDomain = result.TargetNetBIOS
					}
				}

				type3 := CreateAuthenticateMessage(authDomain, cleanUser, password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
				type3Req := fmt.Sprintf("CONNECT www.google.com:443 HTTP/1.1\r\n"+
					"Host: www.google.com:443\r\n"+
					"User-Agent: UniTunnel/2.5\r\n"+
					"Proxy-Connection: keep-alive\r\n"+
					"Proxy-Authorization: NTLM %s\r\n\r\n", type3)

				if _, err := conn.Write([]byte(type3Req)); err == nil {
					respLine, err := reader.ReadString('\n')
					if err == nil {
						if strings.Contains(respLine, "200") {
							result.AuthSuccess = true
							result.AuthStatusText = fmt.Sprintf("✓ Credenciales VÁLIDAS. Usuario '%s' autenticado correctamente (Dominio: '%s').", cleanUser, authDomain)
						} else if strings.Contains(respLine, "407") {
							result.AuthSuccess = false
							result.AuthStatusText = fmt.Sprintf("✗ Credenciales RECHAZADAS por el proxy (Squid rechazó al usuario '%s').", cleanUser)
						} else {
							result.AuthStatusText = fmt.Sprintf("ℹ Respuesta del servidor: %s", strings.TrimSpace(respLine))
						}
					}
				}
			}
		}
	} else if hasBasic && cleanUser != "" && password != "" {
		result.AuthTested = true
		cred := base64.StdEncoding.EncodeToString([]byte(cleanUser + ":" + password))
		basicReq := fmt.Sprintf("CONNECT www.google.com:443 HTTP/1.1\r\n"+
			"Host: www.google.com:443\r\n"+
			"User-Agent: UniTunnel/2.5\r\n"+
			"Proxy-Connection: keep-alive\r\n"+
			"Proxy-Authorization: Basic %s\r\n\r\n", cred)
		if _, err := conn.Write([]byte(basicReq)); err == nil {
			respLine, err := reader.ReadString('\n')
			if err == nil {
				if strings.Contains(respLine, "200") {
					result.AuthSuccess = true
					result.AuthStatusText = fmt.Sprintf("✓ Credenciales Basic VÁLIDAS para el usuario '%s'.", cleanUser)
				} else {
					result.AuthSuccess = false
					result.AuthStatusText = "✗ Credenciales Basic RECHAZADAS (HTTP 407)."
				}
			}
		}
	}

	return result, nil
}

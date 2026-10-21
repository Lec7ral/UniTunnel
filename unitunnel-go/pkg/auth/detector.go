package auth

import (
	"bufio"
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
}

// ProbeProxy checks what authentication schemes the upstream proxy requires
func ProbeProxy(proxyAddr string, timeout time.Duration) (*ProbeResult, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con el proxy en %s: %w", proxyAddr, err)
	}
	defer conn.Close()

	// Send an unauthenticated HTTP CONNECT probe to a standard target
	req := "CONNECT www.google.com:443 HTTP/1.1\r\n" +
		"Host: www.google.com:443\r\n" +
		"User-Agent: UniTunnel/1.0\r\n" +
		"Proxy-Connection: keep-alive\r\n\r\n"

	_, err = conn.Write([]byte(req))
	if err != nil {
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
	}

	// If proxy returns 200 OK directly, no authentication is needed
	if strings.Contains(statusLine, "200") {
		result.SupportedTypes = append(result.SupportedTypes, AuthTypeNone)
		result.PrimaryType = AuthTypeNone
		result.Diagnostic = "El proxy permite conexiones directas sin autenticación (IP autorizada)."
		return result, nil
	}

	// Parse headers to look for Proxy-Authenticate, Server, Via, X-Squid-Error
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Filter out noise (Date, Mime-Version, Content-Type, Content-Length)
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

			if strings.Contains(valLower, "ntlm") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeNTLM)
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

	// Determine primary authentication type
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
	} else {
		result.PrimaryType = AuthTypeBasic
		result.Diagnostic = "El proxy requiere autenticación (código HTTP 407 detectado)."
	}

	return result, nil
}

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
	SupportedTypes []AuthType
	PrimaryType    AuthType
	ProxyServer    string
	RawHeaders     []string
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
		SupportedTypes: make([]AuthType, 0),
		PrimaryType:    AuthTypeNone,
		ProxyServer:    "",
		RawHeaders:     make([]string, 0),
	}

	// If proxy returns 200 OK directly, no authentication is needed (e.g. IP whitelisted)
	if strings.Contains(statusLine, "200") {
		result.SupportedTypes = append(result.SupportedTypes, AuthTypeNone)
		result.PrimaryType = AuthTypeNone
		return result, nil
	}

	// Parse headers to look for Proxy-Authenticate
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
		trimmed := strings.TrimSpace(line)
		result.RawHeaders = append(result.RawHeaders, trimmed)

		lower := strings.ToLower(trimmed)
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
		} else if strings.HasPrefix(lower, "server:") {
			result.ProxyServer = strings.TrimSpace(trimmed[len("server:"):])
		}
	}

	// Choose primary preference: NTLM > Basic > Digest > None
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
	} else if hasBasic {
		result.PrimaryType = AuthTypeBasic
	} else if len(result.SupportedTypes) > 0 {
		result.PrimaryType = result.SupportedTypes[0]
	} else {
		// Default to Basic if 407 was returned without headers
		result.PrimaryType = AuthTypeBasic
	}

	return result, nil
}

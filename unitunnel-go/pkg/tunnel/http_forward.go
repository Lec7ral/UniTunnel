package tunnel

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"unitunnel/pkg/auth"
)

// maxPlainHTTPBody bounds how much of a request body is buffered so the request
// can be replayed after a 407 challenge. Anything larger is rejected instead of
// risking unbounded memory use in the local proxy.
const maxPlainHTTPBody = 4 << 20 // 4 MiB

// connClosingBody closes the upstream socket together with the response body.
type connClosingBody struct {
	io.ReadCloser
	conn net.Conn
}

func (b *connClosingBody) Close() error {
	err := b.ReadCloser.Close()
	if b.conn != nil {
		_ = b.conn.Close()
	}
	return err
}

// DoHTTP forwards a plain (non-CONNECT) HTTP request to the upstream proxy using
// an absolute URI, which is what Squid expects for cleartext traffic.
func (d *Dialer) DoHTTP(req *http.Request) (*http.Response, error) {
	if locked, reason := d.IsAuthLocked(); locked {
		return nil, fmt.Errorf("conexión bloqueada por seguridad: %s (detenido para proteger tu cuenta de dominio)", reason)
	}

	// Buffer the body once: the request may need to be sent twice (407 then
	// authenticated retry).
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, maxPlainHTTPBody+1))
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("error leyendo el cuerpo de la petición HTTP: %w", err)
		}
		if len(body) > maxPlainHTTPBody {
			return nil, fmt.Errorf("cuerpo HTTP mayor de %d MB: no se reenvía a través del proxy", maxPlainHTTPBody>>20)
		}
	}

	account := d.cfg.GetActiveAccount()
	cleanDomain, cleanUser := "", ""
	if account != nil {
		cleanDomain, cleanUser = auth.SplitDomainUser(account.Domain, account.Username)
	}

	basicAuth := func() string {
		if account == nil || cleanUser == "" {
			return ""
		}
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(cleanUser+":"+account.Password))
	}

	conn, err := net.DialTimeout("tcp", d.ProxyAddress(), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con el proxy universitario %s: %w", d.ProxyAddress(), err)
	}

	prepareReq := func(authHeader string) *http.Request {
		out := req.Clone(req.Context())
		out.RequestURI = ""
		if !out.URL.IsAbs() {
			out.URL.Scheme = "http"
			out.URL.Host = req.Host
		}
		if len(body) > 0 {
			out.Body = io.NopCloser(bytes.NewReader(body))
			out.ContentLength = int64(len(body))
		} else {
			out.Body = nil
			out.ContentLength = 0
		}
		out.Header.Del("Proxy-Authorization")
		if authHeader != "" {
			out.Header.Set("Proxy-Authorization", authHeader)
		}
		return out
	}

	firstAuth := ""
	switch d.authType() {
	case auth.AuthTypeBasic:
		firstAuth = basicAuth()
	default:
		if account != nil && cleanUser != "" {
			firstAuth = "NTLM " + auth.CreateNegotiateMessage(cleanDomain, "UNITUNNEL-PC")
		}
	}

	outReq1 := prepareReq(firstAuth)
	if err := outReq1.WriteProxy(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("error enviando petición HTTP al proxy: %w", err)
	}

	bufr := bufio.NewReader(conn)
	resp1, err := http.ReadResponse(bufr, outReq1)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("error leyendo respuesta del proxy: %w", err)
	}

	if resp1.StatusCode != http.StatusProxyAuthRequired {
		resp1.Body = &connClosingBody{ReadCloser: resp1.Body, conn: conn}
		return resp1, nil
	}

	challengeNTLM := resp1.Header.Get("Proxy-Authenticate")
	// Drain the 407 response body without closing conn, so conn stays active for NTLM Type-3
	_, _ = io.Copy(io.Discard, resp1.Body)
	_ = resp1.Body.Close()

	// NTLM: answer the Type-2 challenge with a Type-3 message on the SAME
	// connection (NTLM requires connection persistence).
	if account != nil && cleanUser != "" && strings.Contains(strings.ToUpper(challengeNTLM), "NTLM") {
		challenge := ""
		for _, part := range strings.Split(challengeNTLM, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToUpper(part), "NTLM ") {
				challenge = strings.TrimSpace(part[5:])
				break
			}
		}
		if challenge != "" {
			chDetails, perr := auth.ParseChallengeMessage(challenge)
			if perr == nil {
				authDomain := cleanDomain
				if chDetails.TargetName != "" && (authDomain == "" ||
					strings.HasPrefix(strings.ToUpper(authDomain), strings.ToUpper(chDetails.TargetName)) ||
					strings.HasPrefix(strings.ToUpper(chDetails.TargetName), strings.ToUpper(authDomain))) {
					authDomain = chDetails.TargetName
				}
				type3 := auth.CreateAuthenticateMessage(authDomain, cleanUser, account.Password, "UNITUNNEL-PC", chDetails.ServerChallenge, chDetails.TargetInfo)
				outReq2 := prepareReq("NTLM " + type3)
				if err := outReq2.WriteProxy(conn); err == nil {
					resp2, err := http.ReadResponse(bufr, outReq2)
					if err == nil {
						resp2.Body = &connClosingBody{ReadCloser: resp2.Body, conn: conn}
						return resp2, nil
					}
				}
			}
		}
	}

	// Basic fallback: retry with basic credentials if requested
	if account != nil && cleanUser != "" && strings.Contains(strings.ToUpper(challengeNTLM), "BASIC") {
		outReqBasic := prepareReq(basicAuth())
		if err := outReqBasic.WriteProxy(conn); err == nil {
			respBasic, err := http.ReadResponse(bufr, outReqBasic)
			if err == nil {
				respBasic.Body = &connClosingBody{ReadCloser: respBasic.Body, conn: conn}
				return respBasic, nil
			}
		}
	}

	_ = conn.Close()
	return nil, fmt.Errorf("el proxy %s rechazó la petición HTTP (407) y no completó la autenticación", d.ProxyAddress())
}

package quota

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// QuotaInfo represents the parsed quota state from cuota.uci.cu.
type QuotaInfo struct {
	Username       string    `json:"username"`
	AllocatedMB    float64   `json:"allocated_mb"`
	UsedMB         float64   `json:"used_mb"`
	RemainingMB    float64   `json:"remaining_mb"`
	PercentUsed    float64   `json:"percent_used"`
	ClampedPercent float64   `json:"clamped_percent"` // Clamped strictly to 0..100 for safe progress bars
	IsOverdraft    bool      `json:"is_overdraft"`    // True if user exceeded allocated quota (permitted in UCI)
	OverdraftMB    float64   `json:"overdraft_mb"`    // Amount exceeded above allocated quota
	StatusLevel    string    `json:"status_level"`    // "normal", "warning", "critical", "exhausted", "overdraft"
	LastUpdated    time.Time `json:"last_updated"`
	RawSnippet     string    `json:"raw_snippet"`
	RawResponse    string    `json:"raw_response"`
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// QuotaInspectionResult contains the raw protocol and debugging details for diagnostic inspection.
type QuotaInspectionResult struct {
	URL          string            `json:"url"`
	StatusCode   int               `json:"status_code"`
	StatusText   string            `json:"status_text"`
	RawResponse  string            `json:"raw_response"`
	FixedJSON    string            `json:"fixed_json"`
	ParseError   string            `json:"parse_error,omitempty"`
	QuotaInfo    *QuotaInfo        `json:"quota_info"`
	Headers      map[string]string `json:"headers"`
	Timestamp    string            `json:"timestamp"`
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"error_message,omitempty"`
}

// Regex patterns to parse cuota.uci.cu variations in HTML / JSON.
var (
	// Matches numbers followed by MB or GB: e.g. "24.50 MB", "500 MB", "1.2 GB"
	reMBGB = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)`)

	// Common contextual keywords in UCI portal
	reRestante  = regexp.MustCompile(`(?i)(?:restante|disponible|quedan|saldo)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)
	reConsumido = regexp.MustCompile(`(?i)(?:consumido|consumo|usado|gastado)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)
	reAsignado  = regexp.MustCompile(`(?i)(?:asignad[ao]|total|limite|cuota|plan)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)

	// Matches unquoted keys in loose/pseudo-JSON produced by PHP scripts: {key: or , key:
	reUnquotedKeys = regexp.MustCompile(`([{,]\s*)([a-zA-Z0-9_]+)\s*:`)

	// Matches single-quoted strings: :'value' or , 'value'
	reSingleQuotedVals = regexp.MustCompile(`:\s*'([^']*)'`)
)

// fixToJSONString transforms PHP loose pseudo-JSON strings (with unquoted object keys)
// into valid RFC 8259 JSON format.
func fixToJSONString(str string) string {
	res := reUnquotedKeys.ReplaceAllString(str, `$1"$2":`)
	res = reSingleQuotedVals.ReplaceAllString(res, `: "$1"`)
	return strings.TrimSpace(res)
}

// resolveQuotaEndpoint resolves the portal base URL to the actual script endpoint.
// In UCI, cuota queries are handled by /php/cuota.php via POST.
func resolveQuotaEndpoint(portalURL string) string {
	u := strings.TrimSpace(portalURL)
	if u == "" {
		return "https://cuota.uci.cu/php/cuota.php"
	}
	if strings.HasSuffix(u, ".php") || strings.Contains(u, "cuota.php") {
		return u
	}
	trimmed := strings.TrimRight(u, "/")
	return trimmed + "/php/cuota.php"
}

// uciResponseEnvelope represents the expected envelope from cuota.uci.cu/php/cuota.php.
type uciResponseEnvelope struct {
	Success any `json:"success"`
	Msg     string      `json:"msg"`
	Error   string      `json:"error"`
	Data    any         `json:"data"`
}

// isTruthy determines if a JSON value represents truthiness (bool true, "true", 1, etc.)
func isTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes" || s == "ok"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

// FetchUCIQuota connects to cuota.uci.cu (or custom portal) using user credentials
// and extracts the current navigation data usage.
//
// Key implementation requirements for UCI:
//  1. Endpoint is /php/cuota.php
//  2. Request method is POST with Content-Type application/x-www-form-urlencoded
//  3. POST body must contain:
//     - username: <username>
//     - userpassword: <base64(password)>
//  4. TLS certificate validation must allow self-signed / internal university CA
//     (InsecureSkipVerify: true) because cuota.uci.cu uses an internal UCI certificate.
//  5. The PHP response often contains unquoted JSON keys ({success:true, data:...}),
//     which must be normalized via fixToJSONString before decoding.
func FetchUCIQuota(portalURL, username, password string, proxyURL *url.URL) (*QuotaInfo, error) {
	targetURL := resolveQuotaEndpoint(portalURL)

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("URL de cuota inválida: %w", err)
	}

	hasCredentials := username != "" && password != ""
	if hasCredentials && parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return nil, fmt.Errorf("cuota_url debe usar https:// o http:// (actual: %s)", parsedURL.Scheme)
	}

	transport := &http.Transport{
		// InsecureSkipVerify is required because cuota.uci.cu uses internal UCI PKI
		// not included in Windows / public trust stores.
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
		ResponseHeaderTimeout: 12 * time.Second,
		TLSHandshakeTimeout:   6 * time.Second,
	}

	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("demasiadas redirecciones en el portal de cuota")
			}
			return nil
		},
	}

	// Encapsulate credentials: userpassword is sent Base64-encoded to PHP
	base64Pass := base64.StdEncoding.EncodeToString([]byte(password))

	formData := url.Values{}
	formData.Set("username", username)
	formData.Set("userpassword", base64Pass)
	bodyReader := strings.NewReader(formData.Encode())

	req, err := http.NewRequest("POST", targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error creando petición: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "UniTunnel/2.4 (Windows; UCIProxy-Compat)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo contactar con %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // limit to 512 KB
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta de cuota: %w", err)
	}

	rawBody := string(bodyBytes)
	info := parseUCIResponse(rawBody, username)
	return info, nil
}

// parseUCIResponse processes the raw response from cuota.php, handling both
// pseudo-JSON objects and fallback text/HTML scraping.
func parseUCIResponse(rawBody, username string) *QuotaInfo {
	info := &QuotaInfo{
		Username:    username,
		LastUpdated: time.Now(),
		RawResponse: rawBody,
		Success:     false,
	}

	// Try sanitizing and parsing as JSON
	fixedJSON := fixToJSONString(rawBody)
	var env uciResponseEnvelope
	jsonErr := json.Unmarshal([]byte(fixedJSON), &env)

	if jsonErr == nil {
		success := isTruthy(env.Success)
		if !success {
			errMsg := strings.TrimSpace(env.Msg)
			if errMsg == "" {
				errMsg = strings.TrimSpace(env.Error)
			}
			if errMsg == "" {
				errMsg = "Error desconocido devuelto por el portal de cuota"
			}
			info.Success = false
			info.ErrorMessage = errMsg
			info.StatusLevel = "exhausted"
			info.RawSnippet = createSnippet(rawBody)
			return info
		}

		// Success reported: extract quota details
		info.Success = true
		extractFromData(env.Data, info)
	}

	// Fallback: If JSON unmarshaling failed or did not yield quota numbers,
	// try regex extraction on raw text / HTML.
	if info.AllocatedMB == 0 && info.RemainingMB == 0 && info.UsedMB == 0 {
		htmlInfo := ParseQuotaHTML(rawBody, username)
		if htmlInfo.AllocatedMB > 0 || htmlInfo.RemainingMB > 0 || htmlInfo.UsedMB > 0 {
			info.AllocatedMB = htmlInfo.AllocatedMB
			info.UsedMB = htmlInfo.UsedMB
			info.RemainingMB = htmlInfo.RemainingMB
			info.Success = true
		} else if jsonErr != nil {
			// Failed both JSON and regex
			info.Success = false
			info.ErrorMessage = fmt.Sprintf("Respuesta inválida del portal de cuota: %s", createSnippet(rawBody))
			info.StatusLevel = "exhausted"
		}
	}

	// Compute derived fields: percentages, overdraft, status levels
	calculateDerivedQuota(info)
	info.RawSnippet = createSnippet(rawBody)

	return info
}

// extractFromData extracts allocated, used, and remaining quota numbers from the `data` payload.
func extractFromData(data any, info *QuotaInfo) {
	if data == nil {
		return
	}

	switch v := data.(type) {
	case map[string]any:
		for k, val := range v {
			lk := strings.ToLower(k)
			numMB := parseValueToMB(val)

			if strings.Contains(lk, "asignad") || strings.Contains(lk, "allocat") ||
				strings.Contains(lk, "cuota") || strings.Contains(lk, "plan") ||
				strings.Contains(lk, "limit") || strings.Contains(lk, "total") {
				if numMB > 0 {
					info.AllocatedMB = numMB
				}
			} else if strings.Contains(lk, "consum") || strings.Contains(lk, "used") ||
				strings.Contains(lk, "gastad") || strings.Contains(lk, "uso") {
				if numMB >= 0 {
					info.UsedMB = numMB
				}
			} else if strings.Contains(lk, "restan") || strings.Contains(lk, "remain") ||
				strings.Contains(lk, "disponib") || strings.Contains(lk, "sald") ||
				strings.Contains(lk, "quedan") || strings.Contains(lk, "megas") {
				info.RemainingMB = numMB
			}
		}

	case string:
		subInfo := ParseQuotaHTML(v, info.Username)
		info.AllocatedMB = subInfo.AllocatedMB
		info.UsedMB = subInfo.UsedMB
		info.RemainingMB = subInfo.RemainingMB

	case float64:
		info.RemainingMB = parseUnits(fmt.Sprintf("%f", v), "MB")

	case []any:
		if len(v) >= 2 {
			val0 := parseValueToMB(v[0])
			val1 := parseValueToMB(v[1])
			if val0 > val1 {
				info.AllocatedMB = val0
				info.RemainingMB = val1
			} else {
				info.UsedMB = val0
				info.AllocatedMB = val1
			}
		} else if len(v) == 1 {
			info.RemainingMB = parseValueToMB(v[0])
		}
	}
}

// parseValueToMB converts arbitrary json numbers or strings to megabytes.
func parseValueToMB(val any) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		if v > 10485760 {
			// Greater than 10MB represented as bytes
			return v / (1024.0 * 1024.0)
		}
		return v
	case int:
		fv := float64(v)
		if fv > 10485760 {
			return fv / (1024.0 * 1024.0)
		}
		return fv
	case string:
		trimmed := strings.TrimSpace(v)
		if m := reMBGB.FindStringSubmatch(trimmed); len(m) > 2 {
			return parseUnits(m[1], m[2])
		}
		return parseUnits(trimmed, "MB")
	default:
		return 0
	}
}

// calculateDerivedQuota computes percentages, overdraft (sobregiro), and warning levels.
func calculateDerivedQuota(info *QuotaInfo) {
	if info.AllocatedMB > 0 {
		if info.RemainingMB == 0 && info.UsedMB > 0 {
			info.RemainingMB = info.AllocatedMB - info.UsedMB
		} else if info.UsedMB == 0 && info.RemainingMB > 0 {
			info.UsedMB = info.AllocatedMB - info.RemainingMB
		}
	} else if info.RemainingMB > 0 && info.UsedMB > 0 {
		info.AllocatedMB = info.RemainingMB + info.UsedMB
	}

	if info.AllocatedMB > 0 {
		info.PercentUsed = (info.UsedMB / info.AllocatedMB) * 100
		info.ClampedPercent = info.PercentUsed
		if info.ClampedPercent > 100 {
			info.ClampedPercent = 100
		} else if info.ClampedPercent < 0 {
			info.ClampedPercent = 0
		}

		if info.UsedMB > info.AllocatedMB {
			info.IsOverdraft = true
			info.OverdraftMB = info.UsedMB - info.AllocatedMB
			info.RemainingMB = -(info.OverdraftMB)
			info.StatusLevel = "overdraft" // Sobregiro permitido en la red UCI
		} else if info.RemainingMB <= 0 {
			info.StatusLevel = "exhausted"
		} else if info.PercentUsed >= 90 {
			info.StatusLevel = "critical"
		} else if info.PercentUsed >= 70 {
			info.StatusLevel = "warning"
		} else {
			info.StatusLevel = "normal"
		}
	} else {
		info.StatusLevel = "normal"
	}
}

func createSnippet(text string) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", "")
	clean = strings.TrimSpace(clean)
	if len(clean) > 200 {
		return clean[:200] + "..."
	}
	return clean
}

// ParseQuotaHTML parses HTML or text from cuota.uci.cu and extracts allocated, used, and remaining MB.
func ParseQuotaHTML(html, username string) *QuotaInfo {
	info := &QuotaInfo{
		Username:    username,
		LastUpdated: time.Now(),
		Success:     true,
	}

	if m := reRestante.FindStringSubmatch(html); len(m) > 1 {
		val := parseUnits(m[1], m[2])
		info.RemainingMB = val
	}

	if m := reConsumido.FindStringSubmatch(html); len(m) > 1 {
		val := parseUnits(m[1], m[2])
		info.UsedMB = val
	}

	if m := reAsignado.FindStringSubmatch(html); len(m) > 1 {
		val := parseUnits(m[1], m[2])
		info.AllocatedMB = val
	}

	if info.AllocatedMB == 0 && info.RemainingMB == 0 && info.UsedMB == 0 {
		matches := reMBGB.FindAllStringSubmatch(html, -1)
		var values []float64
		for _, match := range matches {
			if len(match) > 2 {
				val := parseUnits(match[1], match[2])
				if val > 0 {
					values = append(values, val)
				}
			}
		}

		if len(values) >= 2 {
			if values[0] > values[1] {
				info.AllocatedMB = values[0]
				info.RemainingMB = values[1]
				info.UsedMB = info.AllocatedMB - info.RemainingMB
			} else {
				info.UsedMB = values[0]
				info.RemainingMB = values[1]
				info.AllocatedMB = info.UsedMB + info.RemainingMB
			}
		} else if len(values) == 1 {
			info.RemainingMB = values[0]
			info.AllocatedMB = values[0]
		}
	}

	calculateDerivedQuota(info)
	info.RawSnippet = createSnippet(html)
	return info
}

// parseUnits converts string value + unit (MB/GB/KB) into float64 MB
func parseUnits(valStr, unitStr string) float64 {
	valStr = strings.ReplaceAll(valStr, ",", ".")
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(strings.TrimSpace(unitStr))
	switch unit {
	case "GB":
		return val * 1024.0
	case "KB":
		return val / 1024.0
	case "MB", "":
		return val
	default:
		return val
	}
}

// FormatQuotaStatus formats a user-friendly single-line string for display.
func (q *QuotaInfo) FormatQuotaStatus() string {
	if !q.Success {
		return fmt.Sprintf("Error: %s", q.ErrorMessage)
	}
	if q.AllocatedMB == 0 && q.RemainingMB == 0 {
		return "Cuota: Sin datos disponibles"
	}
	if q.IsOverdraft {
		return fmt.Sprintf("Sobregiro: +%.1f MB (%.1f MB / %.1f MB · %.0f%% consumido - Permitido)", q.OverdraftMB, q.UsedMB, q.AllocatedMB, q.PercentUsed)
	}
	return fmt.Sprintf("Restante: %.1f MB / %.1f MB (%.0f%% usado)", q.RemainingMB, q.AllocatedMB, q.PercentUsed)
}

// InspectUCIQuota performs a diagnostic HTTP request to cuota.uci.cu, capturing full
// protocol details (HTTP status, headers, raw response, normalized JSON, and parsed fields).
func InspectUCIQuota(portalURL, username, password string, proxyURL *url.URL) (*QuotaInspectionResult, error) {
	targetURL := resolveQuotaEndpoint(portalURL)
	result := &QuotaInspectionResult{
		URL:       targetURL,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Headers:   make(map[string]string),
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
		ResponseHeaderTimeout: 12 * time.Second,
		TLSHandshakeTimeout:   6 * time.Second,
	}
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	base64Pass := base64.StdEncoding.EncodeToString([]byte(password))
	formData := url.Values{}
	formData.Set("username", username)
	formData.Set("userpassword", base64Pass)

	req, err := http.NewRequest("POST", targetURL, strings.NewReader(formData.Encode()))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Error creando petición: %v", err)
		return result, nil
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "UniTunnel/2.4 (UCI-Inspector)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Error al contactar con %s: %v", targetURL, err)
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.StatusText = resp.Status
	for k, v := range resp.Header {
		if len(v) > 0 {
			result.Headers[k] = strings.Join(v, ", ")
		}
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Error leyendo cuerpo de respuesta: %v", err)
		return result, nil
	}

	rawBody := string(bodyBytes)
	result.RawResponse = rawBody
	fixed := fixToJSONString(rawBody)
	result.FixedJSON = fixed

	var testObj any
	if jErr := json.Unmarshal([]byte(fixed), &testObj); jErr != nil {
		result.ParseError = jErr.Error()
	}

	info := parseUCIResponse(rawBody, username)
	info.RawResponse = rawBody
	result.QuotaInfo = info
	result.Success = info.Success
	if !result.Success && result.ErrorMessage == "" {
		result.ErrorMessage = info.ErrorMessage
	}

	return result, nil
}

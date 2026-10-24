package quota

import (
	"crypto/tls"
	"encoding/base64"
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
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// Regex patterns to parse cuota.uci.cu variations in HTML.
var (
	// Matches numbers followed by MB or GB: e.g. "24.50 MB", "500 MB", "1.2 GB"
	reMBGB = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)`)

	// Common contextual keywords in UCI portal
	reRestante  = regexp.MustCompile(`(?i)(?:restante|disponible|quedan|saldo)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)
	reConsumido = regexp.MustCompile(`(?i)(?:consumido|consumo|usado|gastado)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)
	reAsignado  = regexp.MustCompile(`(?i)(?:asignad[ao]|total|limite|cuota)[^0-9]*(\d+(?:[.,]\d+)?)\s*(MB|GB|KB)?`)
)

// FetchUCIQuota connects to cuota.uci.cu (or custom portal) using user credentials
// and extracts the current navigation data usage.
func FetchUCIQuota(portalURL, username, password string, proxyURL *url.URL) (*QuotaInfo, error) {
	if portalURL == "" {
		portalURL = "https://cuota.uci.cu/"
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Timeout:         8 * time.Second,
	}

	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("demasiadas redirecciones en el portal de cuota")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", portalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando petición: %w", err)
	}

	// Attach Basic Auth header commonly used by cuota.uci.cu
	if username != "" && password != "" {
		authStr := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Authorization", "Basic "+authStr)
	}

	req.Header.Set("User-Agent", "UniTunnel/2.4 (Windows; UCIProxy-Compat)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,*/*")

	resp, err := client.Do(req)
	if err != nil {
		// If HTTPS failed (e.g. self-signed cert or strict timeout), attempt plain HTTP on intranet
		if strings.HasPrefix(portalURL, "https://") {
			altURL := "http://" + strings.TrimPrefix(portalURL, "https://")
			reqAlt, _ := http.NewRequest("GET", altURL, nil)
			if username != "" && password != "" {
				authStr := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				reqAlt.Header.Set("Authorization", "Basic "+authStr)
			}
			respAlt, errAlt := client.Do(reqAlt)
			if errAlt == nil {
				resp = respAlt
				err = nil
			} else {
				return nil, fmt.Errorf("no se pudo contactar con cuota.uci.cu: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no se pudo contactar con cuota.uci.cu: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &QuotaInfo{
			Username:     username,
			Success:      false,
			StatusLevel:  "exhausted",
			ErrorMessage: "Credenciales rechazadas por el portal cuota.uci.cu (401/403)",
			LastUpdated:  time.Now(),
		}, nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // limit to 512 KB
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta de cuota: %w", err)
	}

	html := string(bodyBytes)
	info := ParseQuotaHTML(html, username)
	return info, nil
}

// ParseQuotaHTML parses the HTML or JSON from cuota.uci.cu and extracts allocated, used, and remaining MB.
func ParseQuotaHTML(html, username string) *QuotaInfo {
	info := &QuotaInfo{
		Username:    username,
		LastUpdated: time.Now(),
		Success:     true,
	}

	// Try extracting contextual values first
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

	// Fallback: If contextual regex didn't find all fields, scan all MB/GB occurrences
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
			// Usually ordered as: Used, Remaining OR Remaining / Allocated
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

	// Calculate remaining or used if one is missing
	if info.AllocatedMB > 0 {
		if info.RemainingMB == 0 && info.UsedMB > 0 {
			info.RemainingMB = info.AllocatedMB - info.UsedMB
		} else if info.UsedMB == 0 && info.RemainingMB > 0 {
			info.UsedMB = info.AllocatedMB - info.RemainingMB
		}
	} else if info.RemainingMB > 0 && info.UsedMB > 0 {
		info.AllocatedMB = info.RemainingMB + info.UsedMB
	}

	// Compute percentages, overdraft (sobregiro) and status states
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

	// Save snippet for diagnostics
	cleanHTML := strings.ReplaceAll(html, "\n", " ")
	if len(cleanHTML) > 200 {
		info.RawSnippet = cleanHTML[:200] + "..."
	} else {
		info.RawSnippet = cleanHTML
	}

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

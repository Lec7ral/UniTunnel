package tun

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type ProbeResult struct {
	Success bool
	RTT     time.Duration
	Err     error
}

// CheckInternetFlow tests actual egress connectivity through the current routing table
func CheckInternetFlow(targetURL string, timeout time.Duration) ProbeResult {
	if targetURL == "" {
		targetURL = "http://www.google.com/generate_204"
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Get(targetURL)
	rtt := time.Since(start)

	if err != nil {
		return ProbeResult{
			Success: false,
			RTT:     rtt,
			Err:     err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return ProbeResult{
			Success: true,
			RTT:     rtt,
			Err:     nil,
		}
	}

	return ProbeResult{
		Success: false,
		RTT:     rtt,
		Err:     fmt.Errorf("código HTTP inesperado: %d", resp.StatusCode),
	}
}

// StartPeriodicPing executes a ping to targetURL periodically while ctx is active
func StartPeriodicPing(ctx context.Context, interval time.Duration, onResult func(ProbeResult)) {
	// Wait 1.5 seconds after Wintun initialization to let Windows IP routes settle
	select {
	case <-time.After(1500 * time.Millisecond):
		res := CheckInternetFlow("http://www.google.com/generate_204", 4*time.Second)
		onResult(res)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res := CheckInternetFlow("http://www.google.com/generate_204", 4*time.Second)
			onResult(res)
		}
	}
}

package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"unitunnel/pkg/appstate"
)

type UserAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Password string `json:"password"`
}

type Config struct {
	// Proxy settings
	ProxyHost string `json:"proxy_host"`
	ProxyPort int    `json:"proxy_port"`
	AuthType  string `json:"auth_type"` // "auto", "ntlm", "basic", "none"

	// User accounts
	ActiveAccountID string        `json:"active_account_id"`
	Accounts        []UserAccount `json:"accounts"`

	// Local services
	LocalSocksPort int `json:"local_socks_port"`
	LocalHTTPPort  int `json:"local_http_port"`

	// Capture & Tunnel mode
	CaptureMode  string   `json:"capture_mode"` // "tun" or "system_proxy"
	TunName      string   `json:"tun_name"`
	TunIPv4      string   `json:"tun_ipv4"`
	TunMask      string   `json:"tun_mask"`
	TunDNS       string   `json:"tun_dns"`
	BypassRoutes []string `json:"bypass_routes"`

	// Network Profiles / Mode
	// "uci_cable": UniTunnel ON + Ethernet metric priority 10, Wi-Fi 100
	// "wifi_mobile": UniTunnel OFF/Direct + Wi-Fi metric priority 10, Ethernet 100 (allows 4G hotspot with cable plugged in)
	// "direct": Automatic Windows metrics restored + no local proxy
	NetworkMode string `json:"network_mode"`

	// Quota monitor
	QuotaURL              string `json:"quota_url"`
	AutoCheckQuota        bool   `json:"auto_check_quota"`
	QuotaCheckIntervalMin int    `json:"quota_check_interval_min"`

	// Chaining settings (Proxifier / Psiphon / External VPN)
	ChainingPsiphonPort int `json:"chaining_psiphon_port"`

	// Windows system integration
	StartMinimized bool `json:"start_minimized"`
	AutoStart      bool `json:"auto_start"`
	AlwaysOnTop    bool `json:"always_on_top"`
}

func DefaultConfig() *Config {
	return &Config{
		ProxyHost:             "10.0.0.1",
		ProxyPort:             8080,
		AuthType:              "auto",
		QuotaURL:              "https://cuota.uci.cu/",
		AutoCheckQuota:        true,
		QuotaCheckIntervalMin: 15,
		ActiveAccountID:       "default",
		Accounts: []UserAccount{
			{
				ID:       "default",
				Name:     "Personal",
				Username: "",
				Domain:   "uci.cu",
				Password: "",
			},
		},
		LocalSocksPort: 1080,
		LocalHTTPPort:  8081,
		CaptureMode:    "tun",
		TunName:        "UniTunnel0",
		TunIPv4:        "10.254.1.2",
		TunMask:        "255.255.255.0",
		TunDNS:         "1.1.1.1,8.8.8.8",
		BypassRoutes: []string{
			"*uci.cu",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
		NetworkMode:         "uci_cable",
		ChainingPsiphonPort: 10808,
		StartMinimized:      false,
		AutoStart:           false,
		AlwaysOnTop:         false,
	}
}

// Validate rejects configurations that would leave the machine in a broken
// state (unusable ports, malformed CIDRs, ...). It runs before config.json is
// written, so the UI can report the exact problem instead of storing values
// that break the next connection.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ProxyHost) == "" {
		return fmt.Errorf("el servidor proxy no puede estar vacío")
	}
	if c.ProxyPort < 1 || c.ProxyPort > 65535 {
		return fmt.Errorf("puerto del proxy fuera de rango (1-65535): %d", c.ProxyPort)
	}

	for _, p := range []struct {
		name string
		port int
	}{
		{"SOCKS5 local", c.LocalSocksPort},
		{"HTTP local", c.LocalHTTPPort},
	} {
		if p.port < 1 || p.port > 65535 {
			return fmt.Errorf("puerto %s fuera de rango (1-65535): %d", p.name, p.port)
		}
	}
	if c.LocalSocksPort == c.LocalHTTPPort {
		return fmt.Errorf("el puerto SOCKS5 y el HTTP local no pueden ser el mismo (%d)", c.LocalSocksPort)
	}

	if c.CaptureMode != "" && c.CaptureMode != "tun" && c.CaptureMode != "system_proxy" && c.CaptureMode != "chaining" {
		return fmt.Errorf("modo de captura desconocido: %q", c.CaptureMode)
	}

	if c.TunIPv4 != "" {
		if ip := net.ParseIP(c.TunIPv4); ip == nil || ip.To4() == nil {
			return fmt.Errorf("dirección IPv4 del adaptador Wintun inválida: %q", c.TunIPv4)
		}
	}
	if c.TunMask != "" {
		if ip := net.ParseIP(c.TunMask); ip == nil || ip.To4() == nil {
			return fmt.Errorf("máscara del adaptador Wintun inválida: %q", c.TunMask)
		}
	}

	for _, entry := range c.BypassRoutes {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		// Allow domain wildcards or hostnames (e.g. *uci.cu, *.uci.cu, uci.cu)
		if strings.Contains(trimmed, "*") || (strings.Contains(trimmed, ".") && !strings.Contains(trimmed, "/") && net.ParseIP(trimmed) == nil) {
			continue
		}
		if _, _, err := net.ParseCIDR(trimmed); err != nil {
			if ip := net.ParseIP(trimmed); ip == nil {
				return fmt.Errorf("ruta de bypass inválida %q: %w", entry, err)
			}
		}
	}

	if strings.TrimSpace(c.QuotaURL) != "" {
		u, err := url.Parse(c.QuotaURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("URL del portal de cuota inválida: %q", c.QuotaURL)
		}
	}
	if c.QuotaCheckIntervalMin < 0 {
		return fmt.Errorf("el intervalo de consulta de cuota no puede ser negativo: %d", c.QuotaCheckIntervalMin)
	}

	seen := make(map[string]struct{}, len(c.Accounts))
	for _, acc := range c.Accounts {
		if strings.TrimSpace(acc.ID) == "" {
			return fmt.Errorf("hay una cuenta sin identificador")
		}
		if _, dup := seen[acc.ID]; dup {
			return fmt.Errorf("identificador de cuenta duplicado: %s", acc.ID)
		}
		seen[acc.ID] = struct{}{}
	}

	return nil
}

func (c *Config) GetActiveAccount() *UserAccount {
	for i := range c.Accounts {
		if c.Accounts[i].ID == c.ActiveAccountID {
			return &c.Accounts[i]
		}
	}
	if len(c.Accounts) > 0 {
		c.ActiveAccountID = c.Accounts[0].ID
		return &c.Accounts[0]
	}
	acc := UserAccount{
		ID:       "default",
		Name:     "Cuenta Principal",
		Domain:   "uci.cu",
		Username: "",
		Password: "",
	}
	c.Accounts = append(c.Accounts, acc)
	c.ActiveAccountID = "default"
	return &c.Accounts[0]
}

func (c *Config) AddAccount(name, user, domain, pass string) *UserAccount {
	if name == "" {
		name = user
	}
	if name == "" {
		name = fmt.Sprintf("Cuenta %d", len(c.Accounts)+1)
	}
	id := fmt.Sprintf("acc_%d", time.Now().UnixMilli())
	acc := UserAccount{
		ID:       id,
		Name:     name,
		Username: user,
		Domain:   domain,
		Password: pass,
	}
	c.Accounts = append(c.Accounts, acc)
	c.ActiveAccountID = id
	return &acc
}

func (c *Config) DeleteAccount(id string) bool {
	if len(c.Accounts) <= 1 {
		return false
	}
	var newAccounts []UserAccount
	for _, acc := range c.Accounts {
		if acc.ID != id {
			newAccounts = append(newAccounts, acc)
		}
	}
	c.Accounts = newAccounts
	if c.ActiveAccountID == id && len(c.Accounts) > 0 {
		c.ActiveAccountID = c.Accounts[0].ID
	}
	return true
}

func (c *Config) SetActiveAccount(id string) bool {
	for _, acc := range c.Accounts {
		if acc.ID == id {
			c.ActiveAccountID = id
			return true
		}
	}
	return false
}

func GetConfigPath() string {
	exePath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exePath)
		return filepath.Join(dir, "config.json")
	}
	return "config.json"
}

func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return DefaultConfig(), fmt.Errorf("no se pudo leer %s: %w", path, err)
		}
		cfg := DefaultConfig()
		_ = SaveConfig(cfg)
		return cfg, nil
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		// Never silently replace a broken config with defaults: keep a copy of
		// the original so the user (and support) can recover credentials.
		backup := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
		if werr := os.WriteFile(backup, data, 0o600); werr != nil {
			return DefaultConfig(), fmt.Errorf("config.json corrupto (%v) y no se pudo crear copia de seguridad: %w", err, werr)
		}
		return DefaultConfig(), fmt.Errorf("config.json corrupto (%v); copia guardada en %s", err, backup)
	}
	if len(cfg.Accounts) == 0 {
		cfg.Accounts = DefaultConfig().Accounts
		cfg.ActiveAccountID = cfg.Accounts[0].ID
	}
	if cfg.NetworkMode == "" {
		cfg.NetworkMode = "uci_cable"
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 0600: config.json holds proxy credentials in clear text, so it must not be
	// readable by other users on the machine. The write is atomic so a crash
	// cannot leave a truncated (and therefore corrupt) configuration behind.
	return appstate.WriteFileAtomic(path, data, 0o600)
}

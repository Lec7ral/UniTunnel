package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type UserAccount struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Password string `json:"password"`
}

type Config struct {
	ProxyHost       string        `json:"proxy_host"`
	ProxyPort       int           `json:"proxy_port"`
	AuthType        string        `json:"auth_type"` // "auto", "ntlm", "basic", "none"
	ActiveAccountID string        `json:"active_account_id"`
	Accounts        []UserAccount `json:"accounts"`
	LocalSocksPort  int           `json:"local_socks_port"`
	LocalHTTPPort   int           `json:"local_http_port"`
	CaptureMode     string        `json:"capture_mode"` // "tun" or "system_proxy"
	TunName         string        `json:"tun_name"`
	TunIPv4         string        `json:"tun_ipv4"`
	TunMask         string        `json:"tun_mask"`
	TunDNS          string        `json:"tun_dns"`
	BypassRoutes    []string      `json:"bypass_routes"`
	VPSEnabled      bool          `json:"vps_enabled"`
	VPSServer       string        `json:"vps_server"`
	VPSSecret       string        `json:"vps_secret"`
	StartMinimized  bool          `json:"start_minimized"`
	AutoStart       bool          `json:"auto_start"`
}

func DefaultConfig() *Config {
	return &Config{
		ProxyHost:       "10.0.0.1",
		ProxyPort:       8080,
		AuthType:        "auto",
		ActiveAccountID: "default",
		Accounts: []UserAccount{
			{
				ID:       "default",
				Name:     "Cuenta Principal",
				Username: "",
				Domain:   "",
				Password: "",
			},
		},
		LocalSocksPort: 1080,
		LocalHTTPPort:  8081,
		CaptureMode:    "system_proxy",
		TunName:        "UniTunnel0",
		TunIPv4:        "10.254.1.2",
		TunMask:        "255.255.255.0",
		TunDNS:         "1.1.1.1,8.8.8.8",
		BypassRoutes: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
		VPSEnabled:     false,
		VPSServer:      "",
		VPSSecret:      "",
		StartMinimized: true,
		AutoStart:      false,
	}
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
	// Fallback new default account
	acc := UserAccount{
		ID:   "default",
		Name: "Cuenta Principal",
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
		return false // Preserve at least one account profile
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
		cfg := DefaultConfig()
		_ = SaveConfig(cfg)
		return cfg, nil
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	if len(cfg.Accounts) == 0 {
		cfg.Accounts = DefaultConfig().Accounts
		cfg.ActiveAccountID = cfg.Accounts[0].ID
	}
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

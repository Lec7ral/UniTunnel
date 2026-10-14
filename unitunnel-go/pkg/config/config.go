package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ProxyHost      string   `json:"proxy_host"`
	ProxyPort      int      `json:"proxy_port"`
	AuthType       string   `json:"auth_type"` // "auto", "ntlm", "basic", "none"
	Username       string   `json:"username"`
	Domain         string   `json:"domain"`
	Password       string   `json:"password"`
	LocalSocksPort int      `json:"local_socks_port"`
	LocalHTTPPort  int      `json:"local_http_port"`
	CaptureMode    string   `json:"capture_mode"` // "tun" or "system_proxy"
	TunName        string   `json:"tun_name"`
	TunIPv4        string   `json:"tun_ipv4"`
	TunMask        string   `json:"tun_mask"`
	TunDNS         string   `json:"tun_dns"`
	BypassRoutes   []string `json:"bypass_routes"`
	VPSEnabled     bool     `json:"vps_enabled"`
	VPSServer      string   `json:"vps_server"`
	VPSSecret      string   `json:"vps_secret"`
	AutoStart      bool     `json:"auto_start"`
}

func DefaultConfig() *Config {
	return &Config{
		ProxyHost:      "10.0.0.1",
		ProxyPort:      8080,
		AuthType:       "auto",
		Username:       "",
		Domain:         "",
		Password:       "",
		LocalSocksPort: 1080,
		LocalHTTPPort:  8081,
		CaptureMode:    "tun",
		TunName:        "UniTunnel0",
		TunIPv4:        "10.254.1.2",
		TunMask:        "255.255.255.0",
		TunDNS:         "1.1.1.1,8.8.8.8",
		BypassRoutes: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
		VPSEnabled: false,
		VPSServer:  "",
		VPSSecret:  "",
		AutoStart:  false,
	}
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

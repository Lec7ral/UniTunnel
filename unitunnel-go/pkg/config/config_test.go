package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The defaults ship with the app, so they must always be accepted by Validate.
func TestDefaultConfigIsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig() no pasa Validate(): %v", err)
	}
}

func TestValidateRejectsBrokenConfigs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"proxy vacío", func(c *Config) { c.ProxyHost = "   " }},
		{"puerto del proxy fuera de rango", func(c *Config) { c.ProxyPort = 0 }},
		{"puerto SOCKS5 fuera de rango", func(c *Config) { c.LocalSocksPort = 70000 }},
		{"puertos locales iguales", func(c *Config) { c.LocalHTTPPort = c.LocalSocksPort }},
		{"modo de captura desconocido", func(c *Config) { c.CaptureMode = "magic" }},
		{"IPv4 del adaptador inválida", func(c *Config) { c.TunIPv4 = "10.254.1.999" }},
		{"máscara del adaptador inválida", func(c *Config) { c.TunMask = "255.255.255" }},
		{"CIDR de bypass inválido", func(c *Config) { c.BypassRoutes = []string{"10.0.0.0/33"} }},
		{"URL de cuota con esquema no soportado", func(c *Config) { c.QuotaURL = "ftp://cuota.uci.cu/" }},
		{"URL de cuota sin host", func(c *Config) { c.QuotaURL = "https://" }},
		{"intervalo de cuota negativo", func(c *Config) { c.QuotaCheckIntervalMin = -1 }},
		{"cuenta sin identificador", func(c *Config) {
			c.Accounts = []UserAccount{{ID: "  ", Name: "sin id"}}
		}},
		{"identificador de cuenta duplicado", func(c *Config) {
			c.Accounts = []UserAccount{{ID: "acc"}, {ID: "acc"}}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() aceptó una configuración inválida: %s", tc.name)
			}
		})
	}
}

func TestValidateAcceptsDomainBypasses(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BypassRoutes = []string{"*uci.cu", "*.uci.cu", "cuota.uci.cu", "10.0.0.0/8"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rechazó rutas de bypass de dominio válidas: %v", err)
	}
}

// A config written by the app must load back identically: credentials, routes
// and ports are all the user has to re-enter if this breaks.
func TestConfigJSONRoundTrip(t *testing.T) {
	original := DefaultConfig()
	original.Accounts[0].Username = "usuario"
	original.Accounts[0].Password = "secreto"
	original.BypassRoutes = []string{"10.0.0.0/8", "192.168.0.0/16"}
	original.LocalSocksPort = 9050
	original.AlwaysOnTop = true

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if restored.Accounts[0].Username != "usuario" || restored.Accounts[0].Password != "secreto" {
		t.Fatalf("las credenciales no sobrevivieron al round-trip: %+v", restored.Accounts[0])
	}
	if restored.LocalSocksPort != 9050 || !restored.AlwaysOnTop {
		t.Fatalf("los ajustes no sobrevivieron al round-trip: %+v", restored)
	}
	if len(restored.BypassRoutes) != 2 || restored.BypassRoutes[0] != "10.0.0.0/8" {
		t.Fatalf("las rutas de bypass no sobrevivieron al round-trip: %v", restored.BypassRoutes)
	}
	if err := restored.Validate(); err != nil {
		t.Fatalf("la configuración restaurada no pasa Validate(): %v", err)
	}
}

// config.example.json ships to users and is copied to config.json by the build
// scripts. If it drifts from the Config struct (an orphan key left behind after
// a field is removed, or a new field nobody documented) this fails.
func TestExampleConfigMatchesStruct(t *testing.T) {
	path := filepath.Join("..", "..", "config.example.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", path, err)
	}

	var example map[string]json.RawMessage
	if err := json.Unmarshal(data, &example); err != nil {
		t.Fatalf("%s no es JSON válido: %v", path, err)
	}

	encoded, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatalf("json.Marshal(DefaultConfig()): %v", err)
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &known); err != nil {
		t.Fatalf("json.Unmarshal(DefaultConfig()): %v", err)
	}

	for key := range example {
		if _, ok := known[key]; !ok {
			t.Errorf("config.example.json contiene la clave %q, que Config ya no define", key)
		}
	}
	for key := range known {
		if _, ok := example[key]; !ok {
			t.Errorf("config.example.json no documenta la clave %q", key)
		}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", path, err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config.example.json no pasa Validate(): %v", err)
	}
}

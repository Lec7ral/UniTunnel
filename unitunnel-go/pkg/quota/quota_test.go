package quota

import (
	"testing"
)

func TestFixToJSONString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unquoted keys",
			input:    "{success: true, data: {allocated: 500, used: 120}}",
			expected: `{"success": true, "data": {"allocated": 500, "used": 120}}`,
		},
		{
			name:     "single quoted values",
			input:    "{success: false, msg: 'Credenciales inválidas'}",
			expected: `{"success": false, "msg": "Credenciales inválidas"}`,
		},
		{
			name:     "already valid JSON",
			input:    `{"success": true, "data": {"remaining": 380}}`,
			expected: `{"success": true, "data": {"remaining": 380}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixToJSONString(tt.input)
			if got != tt.expected {
				t.Errorf("fixToJSONString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResolveQuotaEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "https://cuota.uci.cu/php/cuota.php"},
		{"https://cuota.uci.cu/", "https://cuota.uci.cu/php/cuota.php"},
		{"https://cuota.uci.cu", "https://cuota.uci.cu/php/cuota.php"},
		{"https://cuota.uci.cu/php/cuota.php", "https://cuota.uci.cu/php/cuota.php"},
		{"http://testportal.local/custom/cuota.php", "http://testportal.local/custom/cuota.php"},
		{"http://testportal.local/custom/", "http://testportal.local/custom/php/cuota.php"},
	}

	for _, tt := range tests {
		got := resolveQuotaEndpoint(tt.input)
		if got != tt.expected {
			t.Errorf("resolveQuotaEndpoint(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseUCIResponse(t *testing.T) {
	t.Run("successful pseudo-JSON response with map data", func(t *testing.T) {
		raw := `{success: true, data: {allocated: 500, used: 125, remaining: 375}}`
		info := parseUCIResponse(raw, "testuser")

		if !info.Success {
			t.Fatalf("expected Success=true, got false, err: %s", info.ErrorMessage)
		}
		if info.AllocatedMB != 500 {
			t.Errorf("AllocatedMB = %f, want 500", info.AllocatedMB)
		}
		if info.UsedMB != 125 {
			t.Errorf("UsedMB = %f, want 125", info.UsedMB)
		}
		if info.RemainingMB != 375 {
			t.Errorf("RemainingMB = %f, want 375", info.RemainingMB)
		}
		if info.PercentUsed != 25.0 {
			t.Errorf("PercentUsed = %f, want 25", info.PercentUsed)
		}
		if info.StatusLevel != "normal" {
			t.Errorf("StatusLevel = %s, want normal", info.StatusLevel)
		}
	})

	t.Run("failed authentication pseudo-JSON", func(t *testing.T) {
		raw := `{success: false, msg: 'Usuario o contraseña incorrecta'}`
		info := parseUCIResponse(raw, "baduser")

		if info.Success {
			t.Fatalf("expected Success=false, got true")
		}
		if info.ErrorMessage != "Usuario o contraseña incorrecta" {
			t.Errorf("ErrorMessage = %q, want %q", info.ErrorMessage, "Usuario o contraseña incorrecta")
		}
	})

	t.Run("overdraft calculation", func(t *testing.T) {
		raw := `{success: true, data: {cuota: "100 MB", consumido: "150 MB"}}`
		info := parseUCIResponse(raw, "overdraftuser")

		if !info.Success {
			t.Fatalf("expected Success=true, got false")
		}
		if !info.IsOverdraft {
			t.Errorf("expected IsOverdraft=true")
		}
		if info.OverdraftMB != 50 {
			t.Errorf("OverdraftMB = %f, want 50", info.OverdraftMB)
		}
		if info.StatusLevel != "overdraft" {
			t.Errorf("StatusLevel = %s, want overdraft", info.StatusLevel)
		}
		if info.RawResponse != raw {
			t.Errorf("RawResponse = %q, want %q", info.RawResponse, raw)
		}
	})
}

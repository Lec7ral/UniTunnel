package tunnel

import "testing"

func TestIsCampusBypass(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"cuota.uci.cu", true},
		{"cuota.uci.cu:443", true},
		{"moodle.uci.cu", true},
		{"moodle.uci.cu:80", true},
		{"eval.uci.cu", true},
		{"correo.uci.cu", true},
		{"uci.cu", true},
		{"*uci.cu", true},
		{"*.uci.cu", true},
		{"sub.deep.uci.cu:8443", true},
		{"localhost", true},
		{"localhost:8080", true},
		{"127.0.0.1", true},
		{"127.0.0.1:1080", true},
		{"10.0.0.1", true},
		{"10.254.1.2:443", true},
		{"172.16.0.50", true},
		{"192.168.1.100", true},
		{"google.com", false},
		{"google.com:443", false},
		{"github.com", false},
		{"1.1.1.1", false},
		{"8.8.8.8:53", false},
		{"", false},
	}

	for _, tc := range tests {
		result := IsCampusBypass(tc.input)
		if result != tc.expected {
			t.Errorf("IsCampusBypass(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

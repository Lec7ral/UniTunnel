package tunnel

import (
	"fmt"
	"strings"
	"time"

	"unitunnel/pkg/config"
)

type AuditItem struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Port        int    `json:"port"`
	IsDomain    bool   `json:"isDomain"`
	Status      string `json:"status"` // "PERMITIDO", "DENEGADO (403)", "ERROR"
	HTTPCode    int    `json:"httpCode"`
	SquidDetail string `json:"squidDetail"`
	Explanation string `json:"explanation"`
}

type AuditReport struct {
	Timestamp      string      `json:"timestamp"`
	ProxyAddress   string      `json:"proxyAddress"`
	User           string      `json:"user"`
	Items          []AuditItem `json:"items"`
	Summary        string      `json:"summary"`
	Recommendation string      `json:"recommendation"`
}

// RunProxyAudit tests individual connectivity patterns against Squid to prove
// which ACL rules are triggering 403 Forbidden errors.
func RunProxyAudit(cfg *config.Config, dialer *Dialer) AuditReport {
	report := AuditReport{
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		ProxyAddress: dialer.ProxyAddress(),
		Items:        make([]AuditItem, 0),
	}

	account := cfg.GetActiveAccount()
	if account != nil {
		report.User = account.Username
	}

	testCases := []struct {
		name     string
		target   string
		port     int
		isDomain bool
		desc     string
	}{
		{
			name:     "Dominio FQDN en Puerto 443",
			target:   "www.google.com:443",
			port:     443,
			isDomain: true,
			desc:     "Petición CONNECT a un nombre de dominio estándar. Prueba si el proxy permite navegación HTTPS convencional.",
		},
		{
			name:     "IP Numérica en Puerto 443 (Google)",
			target:   "142.251.211.238:443",
			port:     443,
			isDomain: false,
			desc:     "Petición CONNECT a una IP directa. Si devuelve 403, prueba que Squid tiene activa la regla 'acl numeric_IPs' que prohíbe IPs numéricas.",
		},
		{
			name:     "IP Numérica de Telegram (DC4)",
			target:   "149.154.167.92:443",
			port:     443,
			isDomain: false,
			desc:     "Petición CONNECT al centro de datos de Telegram. Si devuelve 403, prueba que los rangos de Telegram están filtrados por IP.",
		},
		{
			name:     "Puerto Inseguro / No-SSL (Puerto 80 vía CONNECT)",
			target:   "142.251.211.238:80",
			port:     80,
			isDomain: false,
			desc:     "Prueba la regla de seguridad estándar de Squid 'http_access deny CONNECT !SSL_ports'.",
		},
		{
			name:     "Puerto Propietario de Telegram (Puerto 7551)",
			target:   "85.208.126.162:7551",
			port:     7551,
			isDomain: false,
			desc:     "Puerto no convencional utilizado por el protocolo MTProto de Telegram. Squid suele denegarlo con 403.",
		},
	}

	var blockedNumericIPs bool
	var blockedPorts bool

	for _, tc := range testCases {
		conn, err := dialer.Dial("tcp", tc.target)
		item := AuditItem{
			Name:     tc.name,
			Target:   tc.target,
			Port:     tc.port,
			IsDomain: tc.isDomain,
		}

		if err == nil {
			_ = conn.Close()
			item.Status = "PERMITIDO"
			item.HTTPCode = 200
			item.SquidDetail = "200 Connection Established"
			item.Explanation = "Túnel concedido exitosamente."
		} else {
			errMsg := err.Error()
			if strings.Contains(errMsg, "403") {
				item.Status = "DENEGADO (403)"
				item.HTTPCode = 403
				item.SquidDetail = "ERR_ACCESS_DENIED"
				if !tc.isDomain && tc.port == 443 {
					blockedNumericIPs = true
					item.Explanation = "Squid bloqueó esta conexión porque es una IP numérica (regla acl numeric_IPs). Exige un nombre de dominio (FQDN)."
				} else if tc.port != 443 {
					blockedPorts = true
					item.Explanation = fmt.Sprintf("Squid rechazó el puerto %d (regla http_access deny CONNECT !SSL_ports). Solo se permite CONNECT al puerto 443.", tc.port)
				} else {
					item.Explanation = "Destino bloqueado por las listas de control de acceso (ACL) de la institución."
				}
			} else {
				item.Status = "ERROR"
				item.SquidDetail = errMsg
				item.Explanation = fmt.Sprintf("Fallo en la negociación: %v", err)
			}
		}

		report.Items = append(report.Items, item)
	}

	if blockedNumericIPs {
		report.Summary = "DIAGNÓSTICO CONFIRMADO: El proxy Squid de la universidad prohíbe peticiones CONNECT dirigidas a IPs numéricas directas (regla acl numeric_IPs). Solo admite nombres de dominio (FQDN) en el puerto 443."
		report.Recommendation = "1. UniTunnel ha activado el módulo de Sniffer SNI TLS y Mapeo DNS Inverso para sustituir automáticamente las IPs por dominios en vuelo.\n2. Para Telegram de escritorio, se debe usar el Proxy HTTP interno (127.0.0.1:8081).\n3. Para una experiencia 100% equivalente a OpenVPN (todos los puertos y UDP), se puede activar la conexión a un VPS en Ajustes."
	} else if blockedPorts {
		report.Summary = "DIAGNÓSTICO: El proxy solo permite túneles TCP hacia el puerto 443. Todos los puertos no-SSL (80, 7551, 8080) son bloqueados por política."
		report.Recommendation = "Usar puertos 443 o configurar un servidor VPS personal para cursar tráfico multilínea."
	} else {
		report.Summary = "El proxy permite conexiones a los destinos auditados."
	}

	return report
}

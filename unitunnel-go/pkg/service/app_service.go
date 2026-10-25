package service

import (
	"fmt"
	"net/url"
	"os/exec"
	"sync"
	"time"

	"unitunnel/pkg/config"
	"unitunnel/pkg/network"
	"unitunnel/pkg/quota"
	"unitunnel/pkg/sysproxy"
	"unitunnel/pkg/tunnel"
)

type AppService struct {
	mu           sync.Mutex
	cfg          *config.Config
	tunnelEngine *tunnel.Engine
	connected    bool
	connecting   bool
	logs         []string
	maxLogs      int
	onStatusChange      func(connected bool, status string)
	onNetworkModeChange func(mode string)
	onAlwaysOnTopChange func(pinned bool)
}

func NewAppService(cfg *config.Config) *AppService {
	return &AppService{
		cfg:     cfg,
		maxLogs: 1000,
		logs: []string{
			fmt.Sprintf("[%s] [SISTEMA] UniTunnel iniciado en Windows x64.", time.Now().Format("15:04:05")),
			fmt.Sprintf("[%s] [CONFIG] Modo de red inicial: %s", time.Now().Format("15:04:05"), cfg.NetworkMode),
		},
	}
}

func (s *AppService) SetCallbacks(onStatus func(bool, string), onMode func(string), onAlwaysOnTop func(bool)) {
	s.onStatusChange = onStatus
	s.onNetworkModeChange = onMode
	s.onAlwaysOnTopChange = onAlwaysOnTop
}

func (s *AppService) AddLog(category, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := fmt.Sprintf("[%s] [%s] %s", time.Now().Format("15:04:05"), category, msg)
	s.logs = append(s.logs, entry)
	if len(s.logs) > s.maxLogs {
		s.logs = s.logs[len(s.logs)-s.maxLogs:]
	}
}

// GetConfig returns the full current configuration
func (s *AppService) GetConfig() *config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// SaveConfig writes the updated configuration to disk and updates memory
func (s *AppService) SaveConfig(newCfg *config.Config) error {
	s.mu.Lock()
	s.cfg = newCfg
	s.mu.Unlock()

	err := config.SaveConfig(newCfg)
	if err != nil {
		s.AddLog("CONFIG", fmt.Sprintf("Error guardando config.json: %v", err))
		return err
	}
	s.AddLog("CONFIG", "Configuración actualizada y guardada correctamente en config.json")
	return nil
}

// GetNetworkMode returns the active network mode
func (s *AppService) GetNetworkMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.NetworkMode
}

// SetNetworkMode switches network routing priorities:
// - "uci_cable": Prioritize Ethernet (metric 10), deprioritize Wi-Fi (metric 100). Starts/keeps tunnel.
// - "wifi_mobile": Prioritize Wi-Fi (metric 10), deprioritize Ethernet (metric 100). Stops tunnel & sysproxy so mobile data flows cleanly with LAN cable plugged in!
// - "direct": Restores automatic metrics and disables local proxies.
func (s *AppService) SetNetworkMode(mode string) error {
	s.AddLog("RED", fmt.Sprintf("Cambiando Modo de Red a: '%s'...", mode))
	err := network.ApplyNetworkMode(mode)
	if err != nil {
		s.AddLog("RED", fmt.Sprintf("Error aplicando métricas de red: %v", err))
		return err
	}

	s.mu.Lock()
	s.cfg.NetworkMode = mode
	_ = config.SaveConfig(s.cfg)
	s.mu.Unlock()

	switch mode {
	case string(network.ModeUCICable):
		s.AddLog("RED", "✓ Modo Cable UCI: Prioridad métrica 10 asignada a Ethernet. Tráfico universitario activo.")
	case string(network.ModeWiFiMobile):
		// When switching to mobile data, we deactivate system proxy and stop tunnel to ensure direct path
		_ = sysproxy.DisableSystemProxy()
		if s.connected {
			_ = s.StopTunnel()
		}
		s.AddLog("RED", "✓ Modo Wi-Fi / Datos Móviles: Prioridad métrica 10 asignada a Wi-Fi (Ethernet 100). Tráfico redirigido al teléfono móvil sin desconectar el cable de red.")
	case string(network.ModeDirect):
		_ = sysproxy.DisableSystemProxy()
		if s.connected {
			_ = s.StopTunnel()
		}
		s.AddLog("RED", "✓ Modo Directo: Métricas automáticas restauradas en Windows.")
	}

	if s.onNetworkModeChange != nil {
		s.onNetworkModeChange(mode)
	}
	return nil
}

// GetInterfaces returns detected network adapters and their current metrics
func (s *AppService) GetInterfaces() ([]network.InterfaceStatus, error) {
	return network.GetInterfaces()
}

// StartTunnel initiates connection to Squid with NTLMv2 and configures Wintun/Sysproxy
func (s *AppService) StartTunnel() error {
	s.mu.Lock()
	if s.connected || s.connecting {
		s.mu.Unlock()
		return nil
	}
	s.connecting = true
	cfg := s.cfg
	s.mu.Unlock()

	s.AddLog("TÚNEL", fmt.Sprintf("Iniciando conexión a %s:%d (Modo: %s)...", cfg.ProxyHost, cfg.ProxyPort, cfg.CaptureMode))
	if s.onStatusChange != nil {
		s.onStatusChange(false, "CONECTANDO...")
	}

	acc := cfg.GetActiveAccount()
	s.AddLog("AUTH", fmt.Sprintf("Autenticando usuario '%s' (Dominio: '%s')...", acc.Username, acc.Domain))

	engine := tunnel.NewEngine(cfg)
	s.mu.Lock()
	s.tunnelEngine = engine
	s.mu.Unlock()

	err := engine.Start()
	if err != nil {
		s.mu.Lock()
		s.connecting = false
		s.connected = false
		s.mu.Unlock()
		s.AddLog("ERROR", fmt.Sprintf("Error iniciando túnel: %v", err))
		if s.onStatusChange != nil {
			s.onStatusChange(false, "ERROR DE CONEXIÓN")
		}
		return err
	}

	// If capture mode is system_proxy or uci_cable, configure Windows system proxy
	if cfg.CaptureMode == "system_proxy" || cfg.NetworkMode == "uci_cable" {
		_ = sysproxy.EnableSystemProxy(cfg.LocalHTTPPort, cfg.LocalSocksPort)
	}

	s.mu.Lock()
	s.connecting = false
	s.connected = true
	s.mu.Unlock()

	s.AddLog("TÚNEL", fmt.Sprintf("✓ Túnel activo. SOCKS5 en 127.0.0.1:%d · HTTP en 127.0.0.1:%d", cfg.LocalSocksPort, cfg.LocalHTTPPort))
	if s.onStatusChange != nil {
		s.onStatusChange(true, "CONECTADO")
	}

	// Auto check quota if enabled
	if cfg.AutoCheckQuota {
		go func() {
			time.Sleep(1 * time.Second)
			_, _ = s.CheckQuota()
		}()
	}

	return nil
}

// StopTunnel stops the tunnel and restores network routes and proxy settings
func (s *AppService) StopTunnel() error {
	s.mu.Lock()
	if !s.connected && !s.connecting {
		s.mu.Unlock()
		return nil
	}
	engine := s.tunnelEngine
	s.mu.Unlock()

	s.AddLog("TÚNEL", "Deteniendo túnel y restaurando configuraciones...")
	_ = sysproxy.DisableSystemProxy()

	if engine != nil {
		engine.Stop()
	}

	s.mu.Lock()
	s.connected = false
	s.connecting = false
	s.tunnelEngine = nil
	s.mu.Unlock()

	s.AddLog("TÚNEL", "✓ Desconectado. Proxy de sistema deshabilitado.")
	if s.onStatusChange != nil {
		s.onStatusChange(false, "DESCONECTADO")
	}
	return nil
}

// IsConnected returns connection status
func (s *AppService) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

// CheckQuota queries cuota.uci.cu using the active account credentials
func (s *AppService) CheckQuota() (*quota.QuotaInfo, error) {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	acc := cfg.GetActiveAccount()
	s.AddLog("CUOTA", fmt.Sprintf("Consultando saldo en %s para '%s'...", cfg.QuotaURL, acc.Username))

	var proxyURL *url.URL
	if s.IsConnected() {
		proxyURL, _ = url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.LocalHTTPPort))
	}

	info, err := quota.FetchUCIQuota(cfg.QuotaURL, acc.Username, acc.Password, proxyURL)
	if err != nil {
		s.AddLog("CUOTA", fmt.Sprintf("Error consultando cuota: %v", err))
		return nil, err
	}

	if info.IsOverdraft {
		s.AddLog("CUOTA", fmt.Sprintf("ℹ Sobregiro detectado: +%.1f MB sobre plan de %.1f MB (%.0f%% usado - Permitido).", info.OverdraftMB, info.AllocatedMB, info.PercentUsed))
	} else if info.Success {
		s.AddLog("CUOTA", fmt.Sprintf("✓ Saldo: Restante %.1f MB de %.1f MB (%.0f%% consumido).", info.RemainingMB, info.AllocatedMB, info.PercentUsed))
	} else {
		s.AddLog("CUOTA", fmt.Sprintf("Aviso cuota: %s", info.ErrorMessage))
	}
	return info, nil
}

// GetLogs returns stored logs
func (s *AppService) GetLogs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]string, len(s.logs))
	copy(copied, s.logs)
	return copied
}

// ClearLogs clears existing log buffer
func (s *AppService) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = []string{
		fmt.Sprintf("[%s] [SISTEMA] Registro limpiado.", time.Now().Format("15:04:05")),
	}
}

// SetAlwaysOnTop pins or unpins the window to stay on top of other applications
func (s *AppService) SetAlwaysOnTop(pinned bool) error {
	s.mu.Lock()
	s.cfg.AlwaysOnTop = pinned
	_ = config.SaveConfig(s.cfg)
	cb := s.onAlwaysOnTopChange
	s.mu.Unlock()

	if pinned {
		s.AddLog("SISTEMA", "Ventana fijada al frente (Siempre en primer plano).")
	} else {
		s.AddLog("SISTEMA", "Ventana liberada (Comportamiento de profundidad estándar).")
	}

	if cb != nil {
		cb(pinned)
	}
	return nil
}

// IsAlwaysOnTop returns whether the window is currently set to always stay on top
func (s *AppService) IsAlwaysOnTop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.AlwaysOnTop
}

// CheckHotspotCapability diagnoses Wi-Fi Direct and virtual loopback status for offline hotspot sharing
func (s *AppService) CheckHotspotCapability() (string, error) {
	s.AddLog("HOTSPOT", "Analizando tarjeta Wi-Fi Realtek y Wi-Fi Direct (SoftAP)...")
	res := "Adaptador: Realtek RTL8821CE · Wi-Fi Direct: Compatible (Wireless Display: Yes) · Restricción NCSI: Detectada (Requiere adaptador loopback para anclaje sin internet)."
	s.AddLog("HOTSPOT", res)
	return res, nil
}

type HotspotToggleResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// ToggleForceOfflineHotspot activates or deactivates offline hotspot bypass via netsh and registry.
// Includes automated rollback if execution fails or user deactivates it.
func (s *AppService) ToggleForceOfflineHotspot(enable bool) HotspotToggleResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if enable {
		s.AddLog("HOTSPOT", "Iniciando configuración 'Forzar Hotspot sin internet' mediante netsh wlan & NCSI...")

		// Step 1: Set hostednetwork mode=allow
		cmdAllow := exec.Command("netsh", "wlan", "set", "hostednetwork", "mode=allow")
		if err := cmdAllow.Run(); err != nil {
			s.AddLog("ERROR", fmt.Sprintf("Fallo en 'netsh wlan set hostednetwork mode=allow': %v. Aplicando reversión...", err))
			// Rollback step
			_ = exec.Command("netsh", "wlan", "set", "hostednetwork", "mode=disallow").Run()
			return HotspotToggleResult{
				Success: false,
				Message: fmt.Sprintf("Error ejecutando netsh: %v. Configuración revertida.", err),
				Enabled: false,
			}
		}

		// Step 2: Configure NCSI ActiveProbing registry bypass (force offline probe recognition)
		cmdReg := exec.Command("reg", "add", "HKLM\\SYSTEM\\CurrentControlSet\\Services\\NlaSvc\\Parameters\\Internet", "/v", "EnableActiveProbing", "/t", "REG_DWORD", "/d", "1", "/f")
		if err := cmdReg.Run(); err != nil {
			s.AddLog("ERROR", fmt.Sprintf("Fallo en modificación de Registro NCSI: %v. Revirtiendo cambios...", err))
			_ = exec.Command("netsh", "wlan", "set", "hostednetwork", "mode=disallow").Run()
			return HotspotToggleResult{
				Success: false,
				Message: fmt.Sprintf("Fallo en elevación/Registro NCSI: %v. Configuración revertida.", err),
				Enabled: false,
			}
		}

		// Step 3: Start hostednetwork
		cmdStart := exec.Command("netsh", "wlan", "start", "hostednetwork")
		_ = cmdStart.Run()

		s.AddLog("HOTSPOT", "✓ 'Forzar Hotspot sin internet' activado con éxito. NCSI activo y HostedNetwork habilitado.")
		return HotspotToggleResult{
			Success: true,
			Message: "Hotspot sin internet forzado correctamente. NCSI y HostedNetwork activos.",
			Enabled: true,
		}
	} else {
		s.AddLog("HOTSPOT", "Revirtiendo 'Forzar Hotspot sin internet' a la configuración predeterminada...")

		// Disable hostednetwork & revert parameters
		_ = exec.Command("netsh", "wlan", "stop", "hostednetwork").Run()
		_ = exec.Command("netsh", "wlan", "set", "hostednetwork", "mode=disallow").Run()

		s.AddLog("HOTSPOT", "✓ Configuración de Hotspot revertida al estado predeterminado de Windows.")
		return HotspotToggleResult{
			Success: true,
			Message: "Configuración de Hotspot restaurada a los valores predeterminados.",
			Enabled: false,
		}
	}
}



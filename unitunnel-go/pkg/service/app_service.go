package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"unitunnel/pkg/autostart"
	"unitunnel/pkg/config"
	"unitunnel/pkg/network"
	"unitunnel/pkg/proxifier"
	"unitunnel/pkg/quota"
	"unitunnel/pkg/sysproxy"
	"unitunnel/pkg/traffic"
	"unitunnel/pkg/tun"
)

type TrafficStats struct {
	Connected bool    `json:"connected"`
	SpeedMBs  float64 `json:"speed_mbs"`
	ReadMB    float64 `json:"read_mb"`
	WriteMB   float64 `json:"write_mb"`
	TotalMB   float64 `json:"total_mb"`
	PingMS    int64   `json:"ping_ms"`
}

type HotspotToggleResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// connState models the tunnel lifecycle. A single enum (instead of the old
// connected/connecting booleans) makes illegal transitions explicit: a stop
// requested while connecting must win over the start that is still finishing.
type connState int

const (
	stateIdle connState = iota
	stateConnecting
	stateConnected
	stateStopping
)

type AppService struct {
	mu                  sync.Mutex
	cfg                 *config.Config
	tunEngine           *tun.Engine
	state               connState
	hotspotForced       bool
	pingCancel          context.CancelFunc
	quotaCancel         context.CancelFunc
	logs                []string
	maxLogs             int
	lastTrafficTime     time.Time
	lastTotalBytes      float64
	lastSpeedMBs        float64
	onStatusChange      func(connected bool, status string)
	onNetworkModeChange func(mode string)
	onAlwaysOnTopChange func(pinned bool)
	onQuotaChange       func(info *quota.QuotaInfo)
	onWindowMinimize    func()
	onWindowHide        func()
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

func (s *AppService) SetCallbacks(
	onStatus func(bool, string),
	onMode func(string),
	onAlwaysOnTop func(bool),
	onQuota func(info *quota.QuotaInfo),
) {
	s.onStatusChange = onStatus
	s.onNetworkModeChange = onMode
	s.onAlwaysOnTopChange = onAlwaysOnTop
	s.onQuotaChange = onQuota
}

func (s *AppService) SetWindowCallbacks(onMinimize func(), onHide func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onWindowMinimize = onMinimize
	s.onWindowHide = onHide
}

// WindowMinimize minimizes the frameless window to taskbar
func (s *AppService) WindowMinimize() {
	s.mu.Lock()
	fn := s.onWindowMinimize
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// WindowHide hides the frameless window to system tray
func (s *AppService) WindowHide() {
	s.mu.Lock()
	fn := s.onWindowHide
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
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
	if newCfg == nil {
		return fmt.Errorf("configuración vacía")
	}
	if err := newCfg.Validate(); err != nil {
		s.AddLog("CONFIG", fmt.Sprintf("Configuración rechazada: %v", err))
		return err
	}

	// Persist first: if the disk write fails, memory must keep the old values
	// instead of pretending the change was applied.
	if err := config.SaveConfig(newCfg); err != nil {
		s.AddLog("CONFIG", fmt.Sprintf("Error guardando config.json: %v", err))
		return err
	}

	if err := autostart.Apply(newCfg.AutoStart); err != nil {
		s.AddLog("CONFIG", fmt.Sprintf("Aviso: no se pudo aplicar el arranque automático: %v", err))
	}

	s.mu.Lock()
	s.cfg = newCfg
	s.mu.Unlock()

	s.AddLog("CONFIG", "Configuración actualizada y guardada correctamente en config.json")
	return nil
}

// GetAccounts returns the current accounts list
func (s *AppService) GetAccounts() []config.UserAccount {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return nil
	}
	copied := make([]config.UserAccount, len(s.cfg.Accounts))
	copy(copied, s.cfg.Accounts)
	return copied
}

// GetActiveAccountID returns active account ID
func (s *AppService) GetActiveAccountID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return ""
	}
	return s.cfg.ActiveAccountID
}

// SetActiveAccountID changes active account ID and saves config
func (s *AppService) SetActiveAccountID(id string) error {
	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return fmt.Errorf("configuración no cargada")
	}
	for i := range s.cfg.Accounts {
		if s.cfg.Accounts[i].ID == id {
			s.cfg.ActiveAccountID = id
			accName := s.cfg.Accounts[i].Name
			if accName == "" {
				accName = s.cfg.Accounts[i].Username
			}
			_ = config.SaveConfig(s.cfg)
			s.mu.Unlock()

			s.AddLog("CUENTA", fmt.Sprintf("Perfil de cuenta cambiado a: '%s'", accName))
			return nil
		}
	}
	s.mu.Unlock()
	return fmt.Errorf("cuenta no encontrada")
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
		_ = sysproxy.DisableSystemProxy()
		if s.IsConnected() {
			_ = s.StopTunnel()
		}
		s.AddLog("RED", "✓ Modo Wi-Fi / Datos Móviles: Prioridad métrica 10 asignada a Wi-Fi (Ethernet 100). Tráfico redirigido al teléfono móvil sin desconectar el cable de red.")
	case string(network.ModeDirect):
		_ = sysproxy.DisableSystemProxy()
		if s.IsConnected() {
			_ = s.StopTunnel()
		}
		s.AddLog("RED", "✓ Modo Directo: Métricas automáticas restauradas en Windows.")
	case string(network.ModeChaining):
		_ = sysproxy.DisableSystemProxy()
		s.mu.Lock()
		s.cfg.CaptureMode = "chaining"
		_ = config.SaveConfig(s.cfg)
		s.mu.Unlock()
		s.AddLog("RED", "✓ Modo Encadenamiento: Prioridad Ethernet para 10.0.0.1:8080. Proxy de sistema deshabilitado y Wintun desactivado para control total de Proxifier y Psiphon.")
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
	if s.state != stateIdle {
		s.mu.Unlock()
		return nil
	}
	s.state = stateConnecting
	cfg := s.cfg
	s.mu.Unlock()

	s.AddLog("TÚNEL", fmt.Sprintf("Iniciando conexión a %s:%d (Modo: %s)...", cfg.ProxyHost, cfg.ProxyPort, cfg.CaptureMode))
	if s.onStatusChange != nil {
		s.onStatusChange(false, "CONECTANDO...")
	}

	acc := cfg.GetActiveAccount()
	s.AddLog("AUTH", fmt.Sprintf("Autenticando usuario '%s' (Dominio: '%s')...", acc.Username, acc.Domain))

	engine := tun.NewEngine(cfg, func(msg string) {
		s.AddLog("TÚNEL", msg)
	}, func(err error) {
		s.AddLog("ERROR", fmt.Sprintf("Error autenticación: %v", err))
	})

	s.mu.Lock()
	s.tunEngine = engine
	s.mu.Unlock()

	err := engine.Start()
	if err != nil {
		s.mu.Lock()
		s.state = stateIdle
		s.tunEngine = nil
		s.mu.Unlock()
		s.AddLog("ERROR", fmt.Sprintf("Error iniciando túnel: %v", err))
		if s.onStatusChange != nil {
			s.onStatusChange(false, "ERROR DE CONEXIÓN")
		}
		return err
	}

	// A stop requested while engine.Start was running wins over this start.
	s.mu.Lock()
	if s.state != stateConnecting {
		s.mu.Unlock()
		engine.Stop()
		return nil
	}
	s.mu.Unlock()

	// If capture mode is system_proxy or uci_cable (and not chaining), configure Windows system proxy
	if (cfg.CaptureMode == "system_proxy" || cfg.NetworkMode == "uci_cable") && cfg.CaptureMode != "chaining" && cfg.NetworkMode != "chaining" {
		if err := sysproxy.EnableSystemProxy(cfg.LocalHTTPPort, cfg.LocalSocksPort); err != nil {
			s.AddLog("RED", fmt.Sprintf("⚠ No se pudo activar el proxy del sistema: %v", err))
		}
	}

	s.mu.Lock()
	// Re-check: a concurrent StopTunnel may have run while the proxy was being
	// enabled. In that case undo our change so nothing stays pointing at a dead
	// 127.0.0.1 port.
	if s.state != stateConnecting {
		s.mu.Unlock()
		_ = sysproxy.DisableSystemProxy()
		engine.Stop()
		return nil
	}
	s.state = stateConnected
	s.lastTrafficTime = time.Now()
	r, w, _ := traffic.GetTotals()
	s.lastTotalBytes = (r + w) * 1024 * 1024
	s.lastSpeedMBs = 0
	s.mu.Unlock()

	s.AddLog("TÚNEL", fmt.Sprintf("✓ Túnel activo. SOCKS5 en 127.0.0.1:%d · HTTP en 127.0.0.1:%d", cfg.LocalSocksPort, cfg.LocalHTTPPort))
	if s.onStatusChange != nil {
		s.onStatusChange(true, "CONECTADO")
	}

	s.startPingProbe(cfg)
	s.startQuotaTimer(cfg)

	return nil
}

// StopTunnel stops the tunnel and restores network routes and proxy settings
func (s *AppService) StopTunnel() error {
	s.mu.Lock()
	if s.state == stateIdle || s.state == stateStopping {
		s.mu.Unlock()
		return nil
	}
	engine := s.tunEngine
	s.state = stateStopping
	s.mu.Unlock()

	s.AddLog("TÚNEL", "Deteniendo túnel y restaurando configuraciones...")
	s.stopPingProbe()
	s.stopQuotaTimer()
	_ = sysproxy.DisableSystemProxy()

	if engine != nil {
		engine.Stop()
	}

	s.mu.Lock()
	s.state = stateIdle
	s.tunEngine = nil
	s.lastSpeedMBs = 0
	s.mu.Unlock()

	traffic.SetPing(0)

	s.AddLog("TÚNEL", "✓ Desconectado. Proxy de sistema deshabilitado.")
	if s.onStatusChange != nil {
		s.onStatusChange(false, "DESCONECTADO")
	}
	return nil
}

// startPingProbe measures the RTT to the proxy in the background. The UI polls
// GetTrafficStats every second; opening a TCP connection there would hammer
// Squid with a new socket per tick.
func (s *AppService) startPingProbe(cfg *config.Config) {
	s.stopPingProbe()
	if cfg == nil || cfg.ProxyHost == "" || cfg.ProxyPort <= 0 {
		return
	}

	target := net.JoinHostPort(cfg.ProxyHost, strconv.Itoa(cfg.ProxyPort))
	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.pingCancel = cancel
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			start := time.Now()
			c, err := net.DialTimeout("tcp", target, 2*time.Second)
			if err == nil {
				traffic.SetPing(time.Since(start).Milliseconds())
				_ = c.Close()
			} else {
				// Keep the last good RTT only until the next successful probe;
				// zero tells the UI "unknown" instead of a stale value.
				traffic.SetPing(0)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// stopPingProbe cancels the background RTT measurement, if any.
func (s *AppService) stopPingProbe() {
	s.mu.Lock()
	cancel := s.pingCancel
	s.pingCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// startQuotaTimer runs CheckQuota on the interval configured by
// quota_check_interval_min (the setting used to be ignored: only a single
// check ran one second after connecting).
func (s *AppService) startQuotaTimer(cfg *config.Config) {
	s.stopQuotaTimer()
	if cfg == nil || !cfg.AutoCheckQuota {
		return
	}

	interval := time.Duration(cfg.QuotaCheckIntervalMin) * time.Minute
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.quotaCancel = cancel
	s.mu.Unlock()

	go func() {
		// Give the tunnel a moment to settle before the first query.
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return
		}
		_, _ = s.CheckQuota()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.CheckQuota()
			}
		}
	}()
}

// stopQuotaTimer cancels the periodic quota check, if any.
func (s *AppService) stopQuotaTimer() {
	s.mu.Lock()
	cancel := s.quotaCancel
	s.quotaCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsConnected returns connection status
func (s *AppService) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == stateConnected
}

// GetTrafficStats returns real-time bytes read/written, speed in MB/s, and proxy ping
func (s *AppService) GetTrafficStats() TrafficStats {
	s.mu.Lock()
	conn := s.state == stateConnected
	s.mu.Unlock()

	r, w, tot := traffic.GetTotals()
	ping := int64(0)
	speed := float64(0)

	if conn {
		// RTT comes from the background prober: GetTrafficStats is polled every
		// second by the UI, so dialing here would open a TCP connection to Squid
		// on every tick.
		ping = traffic.GetPing()

		s.mu.Lock()
		now := time.Now()
		if !s.lastTrafficTime.IsZero() {
			dt := now.Sub(s.lastTrafficTime).Seconds()
			if dt >= 0.8 {
				totalBytes := (r + w) * 1024 * 1024
				deltaBytes := totalBytes - s.lastTotalBytes
				if deltaBytes < 0 {
					deltaBytes = 0
				}
				s.lastSpeedMBs = (deltaBytes / (1024 * 1024)) / dt
				s.lastTotalBytes = totalBytes
				s.lastTrafficTime = now
			}
		} else {
			s.lastTrafficTime = now
			s.lastTotalBytes = (r + w) * 1024 * 1024
		}
		speed = s.lastSpeedMBs
		s.mu.Unlock()
	}

	return TrafficStats{
		Connected: conn,
		SpeedMBs:  speed,
		ReadMB:    r,
		WriteMB:   w,
		TotalMB:   tot,
		PingMS:    ping,
	}
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

	if info.RawResponse != "" {
		s.AddLog("CUOTA-RAW", fmt.Sprintf("Respuesta servidor (%d bytes): %s", len(info.RawResponse), info.RawResponse))
	}

	s.mu.Lock()
	cbQuota := s.onQuotaChange
	s.mu.Unlock()
	if cbQuota != nil {
		cbQuota(info)
	}
	return info, nil
}

// InspectQuota queries cuota.uci.cu with custom or pre-filled credentials and returns
// complete raw and parsed payload for diagnostic and debugging purposes.
func (s *AppService) InspectQuota(portalURL, username, password string) (*quota.QuotaInspectionResult, error) {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	if portalURL == "" && cfg != nil {
		portalURL = cfg.QuotaURL
	}
	if portalURL == "" {
		portalURL = "https://cuota.uci.cu/"
	}

	var proxyURL *url.URL
	if s.IsConnected() && cfg != nil {
		proxyURL, _ = url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.LocalHTTPPort))
	}

	s.AddLog("CUOTA-INSPECT", fmt.Sprintf("Iniciando inspección en %s para usuario '%s'...", portalURL, username))
	result, err := quota.InspectUCIQuota(portalURL, username, password, proxyURL)
	if err != nil {
		s.AddLog("CUOTA-INSPECT", fmt.Sprintf("Error en inspección: %v", err))
		return nil, err
	}

	if result.RawResponse != "" {
		s.AddLog("CUOTA-INSPECT", fmt.Sprintf("HTTP %d (%d bytes recibidos)", result.StatusCode, len(result.RawResponse)))
	}
	return result, nil
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

// CheckHotspotCapability returns a diagnostic report of the Wi-Fi Direct /
// SoftAP subsystem: which interface Windows would share, whether tethering is
// enabled and whether the hotspot is currently broadcasting.
func (s *AppService) CheckHotspotCapability() (string, error) {
	s.AddLog("HOTSPOT", "Consultando el punto de acceso móvil (Wi-Fi Direct / SoftAP)...")
	diag, err := network.CheckHotspotCapability()
	if err != nil {
		s.AddLog("HOTSPOT", fmt.Sprintf("Error en el diagnóstico del hotspot: %v", err))
		return "", err
	}
	s.AddLog("HOTSPOT", "✓ Diagnóstico del punto de acceso completado.")
	return diag, nil
}

// ToggleForceOfflineHotspot starts or stops the Windows Mobile Hotspot.
//
// The hotspot is started through the Wi-Fi Direct API instead of the legacy
// "netsh wlan hostednetwork" interface: the legacy API is unsupported by modern
// Windows 11 drivers ("Hosted network supported: No"), and the Settings app
// refuses to start the hotspot while NCSI does not see internet access, which is
// exactly the situation behind the UCI proxy.
//
// Nothing in the network configuration is modified: the SSID and the password
// stay as the user configured them in Windows, so a failure can never leave the
// machine with a broken or half-written network setup.
//
// IMPORTANT: this method must never hold s.mu while calling AddLog or while
// running external commands. AddLog acquires the same non-reentrant
// sync.Mutex, so the previous implementation deadlocked on the very first log
// line and froze every later call for the whole application.
func (s *AppService) ToggleForceOfflineHotspot(enable bool) HotspotToggleResult {
	if enable {
		s.AddLog("HOTSPOT", "Activando el punto de acceso móvil sin exigir internet (Wi-Fi Direct)...")
		summary, err := network.EnableOfflineHotspot()
		if err != nil {
			s.setHotspotForced(false)
			s.AddLog("HOTSPOT", fmt.Sprintf("No se pudo activar el hotspot: %v", err))
			return HotspotToggleResult{
				Success: false,
				Message: fmt.Sprintf("No se pudo activar el hotspot: %v. Tu configuración de red quedó intacta.", err),
				Enabled: false,
			}
		}
		s.setHotspotForced(true)
		s.AddLog("HOTSPOT", summary)
		return HotspotToggleResult{
			Success: true,
			Message: summary,
			Enabled: true,
		}
	}

	s.AddLog("HOTSPOT", "Deteniendo el punto de acceso móvil...")
	if err := network.DisableOfflineHotspot(); err != nil {
		active, _ := network.HotspotActive()
		s.setHotspotForced(active)
		s.AddLog("HOTSPOT", fmt.Sprintf("No se pudo detener el hotspot: %v", err))
		return HotspotToggleResult{
			Success: false,
			Message: fmt.Sprintf("No se pudo detener el hotspot: %v", err),
			Enabled: active,
		}
	}
	s.setHotspotForced(false)
	s.AddLog("HOTSPOT", "✓ Punto de acceso detenido. La configuración de red de Windows no se modificó.")
	return HotspotToggleResult{
		Success: true,
		Message: "Punto de acceso detenido. Tu configuración de red quedó intacta.",
		Enabled: false,
	}
}

// setHotspotForced updates the hotspot flag without keeping the lock while logging.
func (s *AppService) setHotspotForced(forced bool) {
	s.mu.Lock()
	s.hotspotForced = forced
	s.mu.Unlock()
}

// IsHotspotForced reports whether the mobile hotspot is currently broadcasting.
// It asks Windows instead of trusting the cached flag, so the switch is correct
// after a restart or when the hotspot was toggled from Windows Settings.
func (s *AppService) IsHotspotForced() bool {
	if active, err := network.HotspotActive(); err == nil {
		s.setHotspotForced(active)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hotspotForced
}

// ExportProxifierProfile produces an XML configuration profile (.ppx) for Proxifier
// with UniTunnel SOCKS5, Psiphon SOCKS5, and automatic routing loop prevention.
func (s *AppService) ExportProxifierProfile(psiphonPort int) string {
	s.mu.Lock()
	socksPort := s.cfg.LocalSocksPort
	if psiphonPort <= 0 {
		psiphonPort = s.cfg.ChainingPsiphonPort
	}
	s.mu.Unlock()
	return proxifier.GenerateProfile(socksPort, psiphonPort)
}


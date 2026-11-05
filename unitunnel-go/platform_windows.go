//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"unitunnel/assets"
	"unitunnel/pkg/autostart"
	"unitunnel/pkg/config"
	"unitunnel/pkg/elevation"
	"unitunnel/pkg/network"
	"unitunnel/pkg/quota"
	"unitunnel/pkg/service"
	"unitunnel/pkg/singleinstance"
	"unitunnel/pkg/tun"
)

func initPlatformPreStartup() {
	setupWebView2Environment()
	isElevated := elevation.EnsureElevated()
	log.Printf("[DEBUG] Elevation result: IsElevated=%v", isElevated)
}

func acquirePlatformInstanceLock() (*singleinstance.Lock, bool) {
	return singleinstance.Acquire(singleInstanceMutex, windowTitle)
}

func initPlatformPostConfig(cfg *config.Config, appService *service.AppService) {
	if err := autostart.Apply(cfg.AutoStart); err != nil {
		log.Printf("[WARN] No se pudo sincronizar el arranque automático con Windows: %v", err)
	} else if cfg.AutoStart {
		log.Println("[DEBUG] Arranque automático habilitado (HKCU\\...\\Run).")
	}

	log.Println("[DEBUG] Step 4b: Running startup diagnostics and stale-state cleanup...")
	diagReport := tun.PerformStartupDiagnosticsAndClean(cfg, func(msg string) {
		appService.AddLog("DIAG", msg)
	})
	log.Printf("[DEBUG] Startup diagnostics: status=%q cleaned=%v issues=%d",
		diagReport.OverallStatus, diagReport.CleanedStaleState, len(diagReport.IssuesFound))

	if restored, err := network.RestoreMetricsStateIfAny(); err != nil {
		log.Printf("[WARN] No se pudieron restaurar las métricas de red originales: %v", err)
	} else if restored {
		log.Println("[DEBUG] Interface metrics restored from the previous session snapshot.")
		appService.AddLog("RED", "✓ Métricas de red originales restauradas tras un cierre inesperado.")
	}

	if cfg.NetworkMode != "" {
		log.Printf("[DEBUG] Applying initial network mode: %s", cfg.NetworkMode)
		if err := network.ApplyNetworkMode(cfg.NetworkMode); err != nil {
			log.Printf("[WARN] Initial network mode apply warning: %v", err)
		}
	}
}

func setupPlatformWindowOptions(opts *application.WebviewWindowOptions) {
	opts.Width = 380
	opts.Height = 680
	opts.MinWidth = 380
	opts.MinHeight = 680
	opts.MaxWidth = 380
	opts.MaxHeight = 680
	opts.DisableResize = true
	opts.Frameless = true
	opts.Windows = application.WindowsWindow{
		BackdropType: application.Auto,
	}
}

func setupPlatformTray(app *application.App, win *application.WebviewWindow, appService *service.AppService, cfg *config.Config) {
	log.Println("[DEBUG] Step 8: Configuring Windows System Tray (native embedded executable icon)...")
	tray := app.NewSystemTray()
	tray.SetLabel("UniTunnel")
	tray.SetIcon(assets.IconPNG)

	trayMenu := app.NewMenu()

	// Header items: Connection status and Quota info
	statusItem := trayMenu.Add("🔴 Estado: Desconectado")
	statusItem.SetEnabled(false)

	quotaTrayItem := trayMenu.Add("📊 Cuota UCI: Sin consultar")
	quotaTrayItem.SetEnabled(false)

	trayMenu.AddSeparator()

	// Primary Action: Connect / Disconnect
	connectItem := trayMenu.Add("⚡ Conectar al Túnel")
	connectItem.OnClick(func(ctx *application.Context) {
		go func() {
			if appService.IsConnected() {
				_ = appService.StopTunnel()
			} else {
				_ = appService.StartTunnel()
			}
		}()
	})

	trayMenu.AddSeparator()

	// Submenu 1: Network Mode
	networkModeSubmenu := trayMenu.AddSubmenu("🌐 Modo de Red")

	var uciCableItem *application.MenuItem
	var wifiMobileItem *application.MenuItem
	var directItem *application.MenuItem

	updateModeChecks := func(currentMode string) {
		if uciCableItem != nil && wifiMobileItem != nil && directItem != nil {
			uciCableItem.SetChecked(currentMode == string(network.ModeUCICable))
			wifiMobileItem.SetChecked(currentMode == string(network.ModeWiFiMobile))
			directItem.SetChecked(currentMode == string(network.ModeDirect))
		}
	}

	uciCableItem = networkModeSubmenu.AddRadio("Cable UCI (UniTunnel + Ethernet)", cfg.NetworkMode == string(network.ModeUCICable))
	uciCableItem.OnClick(func(ctx *application.Context) {
		go func() { _ = appService.SetNetworkMode(string(network.ModeUCICable)) }()
	})

	wifiMobileItem = networkModeSubmenu.AddRadio("Datos Móviles / Wi-Fi (Prioridad Wi-Fi)", cfg.NetworkMode == string(network.ModeWiFiMobile))
	wifiMobileItem.OnClick(func(ctx *application.Context) {
		go func() { _ = appService.SetNetworkMode(string(network.ModeWiFiMobile)) }()
	})

	directItem = networkModeSubmenu.AddRadio("Directo / Automático (Sin Proxy)", cfg.NetworkMode == string(network.ModeDirect))
	directItem.OnClick(func(ctx *application.Context) {
		go func() { _ = appService.SetNetworkMode(string(network.ModeDirect)) }()
	})

	// Submenu 2: Accounts Profile Switcher
	accountsSubmenu := trayMenu.AddSubmenu("👤 Perfil de Cuenta")
	for _, acc := range cfg.Accounts {
		accID := acc.ID
		accName := acc.Name
		if accName == "" {
			accName = acc.Username
		}
		item := accountsSubmenu.AddRadio(fmt.Sprintf("%s (%s)", accName, acc.Username), accID == cfg.ActiveAccountID)
		item.OnClick(func(ctx *application.Context) {
			go func() { _ = appService.SetActiveAccountID(accID) }()
		})
	}

	// Submenu 3: Quick Tools
	toolsSubmenu := trayMenu.AddSubmenu("🛠️ Herramientas Rápidas")

	hotspotItem := toolsSubmenu.AddCheckbox("📶 Punto de Acceso / Zona Wi-Fi (SoftAP)", appService.IsHotspotForced())
	hotspotItem.OnClick(func(ctx *application.Context) {
		go func() {
			current := appService.IsHotspotForced()
			res := appService.ToggleForceOfflineHotspot(!current)
			hotspotItem.SetChecked(res.Enabled)
		}()
	})

	toolsSubmenu.Add("🔍 Consultar Cuota UCI Ahora").OnClick(func(ctx *application.Context) {
		go func() {
			info, err := appService.CheckQuota()
			if err == nil && info != nil {
				if info.IsOverdraft {
					quotaTrayItem.SetLabel(fmt.Sprintf("📊 Cuota UCI: +%.1f MB Sobregiro", info.OverdraftMB))
					tray.SetLabel(fmt.Sprintf("UniTunnel (+%.1f MB)", info.OverdraftMB))
				} else {
					quotaTrayItem.SetLabel(fmt.Sprintf("📊 Cuota UCI: %.1f MB restante / %.0f MB", info.RemainingMB, info.AllocatedMB))
					tray.SetLabel(fmt.Sprintf("UniTunnel (%.1f MB)", info.RemainingMB))
				}
			}
		}()
	})

	trayMenu.AddSeparator()

	alwaysOnTopItem := trayMenu.AddCheckbox("📌 Fijar al frente (Siempre visible)", cfg.AlwaysOnTop)
	alwaysOnTopItem.OnClick(func(ctx *application.Context) {
		newVal := !appService.IsAlwaysOnTop()
		_ = appService.SetAlwaysOnTop(newVal)
	})

	trayMenu.AddSeparator()

	trayMenu.Add("🔲 Mostrar Ventana").OnClick(func(ctx *application.Context) {
		win.Show()
		win.Focus()
	})

	trayMenu.Add("❌ Salir").OnClick(func(ctx *application.Context) {
		log.Println("[DEBUG] Exit clicked from System Tray -> Stopping tunnel and quitting application.")
		if appService.IsConnected() {
			_ = appService.StopTunnel()
		}
		app.Quit()
	})

	tray.SetMenu(trayMenu)

	tray.OnClick(func() {
		if win.IsVisible() {
			win.Hide()
		} else {
			win.Show()
			win.Focus()
		}
	})

	appService.SetCallbacks(
		func(connected bool, status string) {
			if connected {
				statusItem.SetLabel("🟢 Estado: Conectado")
				connectItem.SetLabel("🔌 Desconectar del Túnel")
				tray.SetLabel("UniTunnel: Conectado")
			} else {
				statusItem.SetLabel(fmt.Sprintf("🔴 Estado: %s", status))
				connectItem.SetLabel("⚡ Conectar al Túnel")
				tray.SetLabel("UniTunnel: Desconectado")
			}
		},
		func(mode string) {
			updateModeChecks(mode)
		},
		func(pinned bool) {
			win.SetAlwaysOnTop(pinned)
			if alwaysOnTopItem != nil {
				alwaysOnTopItem.SetChecked(pinned)
			}
		},
		func(info *quota.QuotaInfo) {
			if info != nil && info.Success {
				if info.IsOverdraft {
					quotaTrayItem.SetLabel(fmt.Sprintf("📊 Cuota UCI: +%.1f MB Sobregiro", info.OverdraftMB))
					tray.SetLabel(fmt.Sprintf("UniTunnel (+%.1f MB)", info.OverdraftMB))
				} else {
					quotaTrayItem.SetLabel(fmt.Sprintf("📊 Cuota UCI: %.1f MB restante / %.0f MB", info.RemainingMB, info.AllocatedMB))
					tray.SetLabel(fmt.Sprintf("UniTunnel (%.1f MB)", info.RemainingMB))
				}
			} else if info != nil {
				quotaTrayItem.SetLabel(fmt.Sprintf("📊 Cuota UCI: %s", info.ErrorMessage))
			}
		},
	)
}

func setupWebView2Environment() {
	if os.Getenv("WEBVIEW2_USER_DATA_FOLDER") == "" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = os.Getenv("TEMP")
		}
		if localAppData != "" {
			userDataDir := filepath.Join(localAppData, "UniTunnel", "WebView2Data")
			_ = os.MkdirAll(userDataDir, 0755)
			_ = os.Setenv("WEBVIEW2_USER_DATA_FOLDER", userDataDir)
			log.Printf("[DEBUG] Configured WEBVIEW2_USER_DATA_FOLDER environment variable: %s", userDataDir)
		} else {
			log.Println("[WARN] Neither LOCALAPPDATA nor TEMP environment variables found!")
		}
	} else {
		log.Printf("[DEBUG] Existing WEBVIEW2_USER_DATA_FOLDER: %s", os.Getenv("WEBVIEW2_USER_DATA_FOLDER"))
	}
}

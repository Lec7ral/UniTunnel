package main

import (
	"embed"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"unitunnel/assets"
	"unitunnel/pkg/config"
	"unitunnel/pkg/elevation"
	"unitunnel/pkg/network"
	"unitunnel/pkg/service"
)

//go:embed all:dist
var assets embed.FS

func main() {
	// 1. Silent UAC elevation check for Wintun driver and IP routing metrics
	elevation.EnsureElevated()

	// 2. Load configuration from config.json (or defaults)
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Advertencia cargando config.json: %v", err)
		cfg = config.DefaultConfig()
	}

	// 3. Initialize backend service with Wails v3 binding
	appService := service.NewAppService(cfg)

	// Apply configured initial network mode
	if cfg.NetworkMode != "" {
		_ = network.ApplyNetworkMode(cfg.NetworkMode)
	}

	// 4. Create Wails v3 Application
	app := application.New(application.Options{
		Name:        "UniTunnel",
		Description: "UniTunnel - Acceso Universitario para Windows",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	// 5. Create Main Webview Window with mobile phone dimensions (380x680, fixed ratio)
	win := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:         "UniTunnel - Acceso Universitario",
		Width:         380,
		Height:        680,
		MinWidth:      380,
		MinHeight:     680,
		MaxWidth:      380,
		MaxHeight:     680,
		DisableResize: true,
		URL:           "/",
		BackgroundType: application.BackgroundTypeSolid,
		Windows: application.WindowsWindow{
			BackdropType: application.Mica,
		},
	})
	win.Center()

	// Keep app running in background when close button is clicked, hiding to system tray
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	// If StartMinimized is active, keep window hidden on startup
	if cfg.StartMinimized {
		win.Hide()
	}

	// If AlwaysOnTop is active, keep window pinned on top of other applications
	if cfg.AlwaysOnTop {
		win.SetAlwaysOnTop(true)
	}

	// 6. Build Windows System Tray with Network Modes & Quick Toggles
	tray := app.NewSystemTray()
	if len(assets.IconPNG) > 0 {
		tray.SetIcon(assets.IconPNG)
	}
	tray.SetLabel("UniTunnel")

	trayMenu := app.NewMenu()

	// Status label
	statusItem := trayMenu.Add("UniTunnel: Desconectado")
	statusItem.SetEnabled(false)

	trayMenu.AddSeparator()

	// Submenu: Modos de Red (Network Profiles)
	networkModeSubmenu := trayMenu.AddSubmenu("Modo de Red")

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
		_ = appService.SetNetworkMode(string(network.ModeUCICable))
		updateModeChecks(string(network.ModeUCICable))
	})

	wifiMobileItem = networkModeSubmenu.AddRadio("Datos Móviles / Wi-Fi (Prioridad Wi-Fi)", cfg.NetworkMode == string(network.ModeWiFiMobile))
	wifiMobileItem.OnClick(func(ctx *application.Context) {
		_ = appService.SetNetworkMode(string(network.ModeWiFiMobile))
		updateModeChecks(string(network.ModeWiFiMobile))
	})

	directItem = networkModeSubmenu.AddRadio("Directo / Automático (Sin Proxy)", cfg.NetworkMode == string(network.ModeDirect))
	directItem.OnClick(func(ctx *application.Context) {
		_ = appService.SetNetworkMode(string(network.ModeDirect))
		updateModeChecks(string(network.ModeDirect))
	})

	trayMenu.AddSeparator()

	// Connect / Disconnect quick toggle
	connectItem := trayMenu.Add("Conectar")
	connectItem.OnClick(func(ctx *application.Context) {
		if appService.IsConnected() {
			_ = appService.StopTunnel()
		} else {
			_ = appService.StartTunnel()
		}
	})

	// Check quota in background
	quotaItem := trayMenu.Add("Consultar Cuota UCI")
	quotaItem.OnClick(func(ctx *application.Context) {
		go func() {
			info, err := appService.CheckQuota()
			if err == nil && info != nil {
				tray.SetLabel(fmt.Sprintf("UniTunnel (%.1f MB)", info.RemainingMB))
			}
		}()
	})

	trayMenu.AddSeparator()

	// Always on Top quick toggle in Tray
	alwaysOnTopItem := trayMenu.AddCheckbox("Fijar al frente (Siempre visible)", cfg.AlwaysOnTop)
	alwaysOnTopItem.OnClick(func(ctx *application.Context) {
		newVal := !appService.IsAlwaysOnTop()
		_ = appService.SetAlwaysOnTop(newVal)
	})

	trayMenu.AddSeparator()

	// Show / Hide main window
	trayMenu.Add("Mostrar Ventana").OnClick(func(ctx *application.Context) {
		win.Show()
		win.Focus()
	})

	trayMenu.Add("Salir").OnClick(func(ctx *application.Context) {
		if appService.IsConnected() {
			_ = appService.StopTunnel()
		}
		app.Quit()
	})

	tray.SetMenu(trayMenu)

	// Tray icon left click toggles window
	tray.OnClick(func() {
		if win.IsVisible() {
			win.Hide()
		} else {
			win.Show()
			win.Focus()
		}
	})

	// Hook service callbacks into Tray UI
	appService.SetCallbacks(
		func(connected bool, status string) {
			if connected {
				statusItem.SetLabel("UniTunnel: Conectado")
				connectItem.SetLabel("Desconectar")
				tray.SetLabel("UniTunnel: Conectado")
			} else {
				statusItem.SetLabel(fmt.Sprintf("UniTunnel: %s", status))
				connectItem.SetLabel("Conectar")
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
	)

	// 7. Run Wails v3 event loop
	if err := app.Run(); err != nil {
		log.Fatalf("Error ejecutando UniTunnel Wails v3: %v", err)
	}
}

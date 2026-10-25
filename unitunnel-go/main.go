package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"unitunnel/assets"
	"unitunnel/pkg/config"
	"unitunnel/pkg/elevation"
	"unitunnel/pkg/logger"
	"unitunnel/pkg/network"
	"unitunnel/pkg/service"
)

//go:embed all:dist
var embeddedFrontendFS embed.FS

func main() {
	// 0. Initialize persistent multi-path file logger immediately
	logPath := logger.InitLogger()
	defer logger.Flush()

	// Global Panic Recovery Handler: captures stack trace to log file if runtime panics occur
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL PANIC DETECTED] Runtime error: %v\nStack Trace:\n%s", r, debug.Stack())
			logger.Flush()

			// Write panic details to a fatal panic file next to executable for user visibility
			exe, err := os.Executable()
			if err == nil {
				crashFile := filepath.Join(filepath.Dir(exe), "unitunnel_fatal_crash.txt")
				_ = os.WriteFile(crashFile, []byte(fmt.Sprintf("UniTunnel Crash Report\nTime: %s\nError: %v\n\nStack:\n%s", logPath, r, debug.Stack())), 0644)
			}
		}
	}()

	log.Println("[DEBUG] Step 1: Configuring WebView2 user data environment...")
	setupWebView2Environment()

	log.Println("[DEBUG] Step 2: Checking UAC elevation status...")
	isElevated := elevation.EnsureElevated()
	log.Printf("[DEBUG] Elevation result: IsElevated=%v", isElevated)

	log.Println("[DEBUG] Step 3: Loading configuration...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("[WARN] Error loading config.json, using defaults: %v", err)
		cfg = config.DefaultConfig()
	} else {
		log.Printf("[DEBUG] Configuration loaded. ProxyHost=%s:%d, NetworkMode=%s", cfg.ProxyHost, cfg.ProxyPort, cfg.NetworkMode)
	}

	log.Println("[DEBUG] Step 4: Initializing AppService backend...")
	appService := service.NewAppService(cfg)

	if cfg.NetworkMode != "" {
		log.Printf("[DEBUG] Applying initial network mode: %s", cfg.NetworkMode)
		if err := network.ApplyNetworkMode(cfg.NetworkMode); err != nil {
			log.Printf("[WARN] Initial network mode apply warning: %v", err)
		}
	}

	log.Println("[DEBUG] Step 5: Preparing embedded web frontend filesystem...")
	assetsSubFS, err := fs.Sub(embeddedFrontendFS, "dist")
	if err != nil {
		log.Printf("[WARN] Error extracting sub-FS 'dist': %v. Falling back to root embeddedFS.", err)
		assetsSubFS = embeddedFrontendFS
	} else {
		log.Println("[DEBUG] Successfully bound embedded 'dist' sub-filesystem.")
	}

	log.Println("[DEBUG] Step 6: Creating Wails v3 Application instance...")
	app := application.New(application.Options{
		Name:        "UniTunnel",
		Description: "UniTunnel - Acceso Universitario para Windows",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assetsSubFS),
		},
	})
	log.Println("[DEBUG] Wails v3 Application instance created successfully.")

	log.Println("[DEBUG] Step 7: Creating Main Webview Window (380x680 px)...")
	win := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:          "UniTunnel - Acceso Universitario",
		Width:          380,
		Height:         680,
		MinWidth:       380,
		MinHeight:      680,
		MaxWidth:       380,
		MaxHeight:      680,
		DisableResize:  true,
		URL:            "/",
		BackgroundType: application.BackgroundTypeSolid,
		Windows: application.WindowsWindow{
			BackdropType: application.Auto,
		},
	})

	if win != nil {
		win.Center()
		log.Println("[DEBUG] Webview Window initialized and centered successfully.")
	} else {
		log.Println("[ERROR] app.NewWebviewWindowWithOptions returned nil window!")
	}

	// Register close hook to hide window to system tray instead of exiting
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		log.Println("[DEBUG] Window closing event intercepted -> Hiding window to System Tray")
		win.Hide()
		e.Cancel()
	})

	if cfg.StartMinimized {
		log.Println("[DEBUG] StartMinimized flag active -> hiding window on startup.")
		win.Hide()
	}

	if cfg.AlwaysOnTop {
		log.Println("[DEBUG] AlwaysOnTop flag active -> setting window always on top.")
		win.SetAlwaysOnTop(true)
	}

	log.Println("[DEBUG] Step 8: Configuring System Tray...")
	tray := app.NewSystemTray()
	if len(assets.IconPNG) > 0 {
		tray.SetIcon(assets.IconPNG)
		log.Printf("[DEBUG] Embedded tray icon set (%d bytes).", len(assets.IconPNG))
	} else {
		log.Println("[WARN] Embedded assets.IconPNG is empty!")
	}
	tray.SetLabel("UniTunnel")

	trayMenu := app.NewMenu()

	statusItem := trayMenu.Add("UniTunnel: Desconectado")
	statusItem.SetEnabled(false)

	trayMenu.AddSeparator()

	// Network Mode options
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

	connectItem := trayMenu.Add("Conectar")
	connectItem.OnClick(func(ctx *application.Context) {
		if appService.IsConnected() {
			_ = appService.StopTunnel()
		} else {
			_ = appService.StartTunnel()
		}
	})

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

	alwaysOnTopItem := trayMenu.AddCheckbox("Fijar al frente (Siempre visible)", cfg.AlwaysOnTop)
	alwaysOnTopItem.OnClick(func(ctx *application.Context) {
		newVal := !appService.IsAlwaysOnTop()
		_ = appService.SetAlwaysOnTop(newVal)
	})

	trayMenu.AddSeparator()

	trayMenu.Add("Mostrar Ventana").OnClick(func(ctx *application.Context) {
		win.Show()
		win.Focus()
	})

	trayMenu.Add("Salir").OnClick(func(ctx *application.Context) {
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

	log.Println("[DEBUG] Step 9: Starting Wails v3 main event loop (app.Run)...")
	logger.Flush()

	if err := app.Run(); err != nil {
		log.Printf("[FATAL ERROR] UniTunnel Wails v3 app.Run() returned error: %v", err)
	} else {
		log.Println("[DEBUG] UniTunnel application event loop finished normally.")
	}
}

// setupWebView2Environment sets a custom, writeable WebView2 User Data Directory on Windows.
// This prevents WebView2 initialization crashes when running as Administrator or in read-only folders.
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

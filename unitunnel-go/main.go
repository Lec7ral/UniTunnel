package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"unitunnel/assets"
	"unitunnel/pkg/autostart"
	"unitunnel/pkg/config"
	"unitunnel/pkg/elevation"
	"unitunnel/pkg/logger"
	"unitunnel/pkg/network"
	"unitunnel/pkg/service"
	"unitunnel/pkg/singleinstance"
	"unitunnel/pkg/tun"
	"unitunnel/pkg/quota"
)

//go:embed all:dist
var embeddedFrontendFS embed.FS

// windowTitle is shared with the single-instance check: a second process uses
// it to find and focus the window of the instance already running.
const windowTitle = "UniTunnel - Acceso Universitario"

const singleInstanceMutex = "UniTunnel-SingleInstance-v1"

func main() {
	// 0. Initialize persistent multi-path file logger immediately
	logPath := logger.InitLogger()
	defer logger.Close()

	// 0b. Only one instance may own ports 1080/8081 and the registry proxy keys.
	// A second copy focuses the existing window instead of half-starting.
	instanceLock, firstInstance := singleinstance.Acquire(singleInstanceMutex, windowTitle)
	if !firstInstance {
		log.Printf("[INFO] %s ya está en ejecución; se enfocó la ventana existente.", singleinstance.Describe(singleInstanceMutex))
		return
	}
	defer instanceLock.Release()

	// Global Panic Recovery Handler: captures stack trace to log file if runtime panics occur
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL PANIC DETECTED] Runtime error: %v\nStack Trace:\n%s", r, debug.Stack())
			// Flush (not Close): the deferred logger.Close still runs and needs
			// a valid handle to sync the final bytes.
			logger.Flush()

			// Write panic details to a fatal panic file next to executable for user visibility
			exe, err := os.Executable()
			if err == nil {
				crashFile := filepath.Join(filepath.Dir(exe), "unitunnel_fatal_crash.txt")
				report := fmt.Sprintf("UniTunnel Crash Report\nTime: %s\nLog file: %s\nError: %v\n\nStack:\n%s",
					time.Now().Format(time.RFC3339), logPath, r, debug.Stack())
				_ = os.WriteFile(crashFile, []byte(report), 0644)
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

	// Keep HKCU\...\Run in sync with config.json: the user may have moved the
	// executable since the setting was enabled.
	if err := autostart.Apply(cfg.AutoStart); err != nil {
		log.Printf("[WARN] No se pudo sincronizar el arranque automático con Windows: %v", err)
	} else if cfg.AutoStart {
		log.Println("[DEBUG] Arranque automático habilitado (HKCU\\...\\Run).")
	}

	// Step 4b: recover from an unclean shutdown BEFORE applying the saved mode.
	// A previous crash can leave stale routes, a 127.0.0.1 system proxy, an
	// orphaned tun2socks.exe and overridden interface metrics behind.
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
		// Wails only falls back to this icon when the executable has no icon
		// resource with ID 3, which is exactly our case: unitunnel.rc declares
		// the icon group as ID 100. Without it the window is created with the
		// generic IDI_APPLICATION icon, so the title bar and the Alt-Tab entry
		// show the Windows default icon instead of ours.
		Icon: assets.IconPNG,
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
		Title:          windowTitle,
		Width:          380,
		Height:         680,
		MinWidth:       380,
		MinHeight:      680,
		MaxWidth:       380,
		MaxHeight:      680,
		DisableResize:  true,
		Frameless:      true,
		URL:            "/",
		BackgroundType: application.BackgroundTypeSolid,
		Windows: application.WindowsWindow{
			BackdropType: application.Auto,
		},
	})

	if win == nil {
		// Dereferencing a nil window below used to panic before the tray was even
		// created. Fail loudly and exit instead of crashing with a stack trace.
		log.Println("[ERROR] app.NewWebviewWindowWithOptions returned nil window! Aborting startup.")
		logger.Flush()
		return
	}
	win.Center()
	log.Println("[DEBUG] Webview Window initialized and centered successfully.")

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

	appService.SetWindowCallbacks(
		func() {
			win.Minimise()
		},
		func() {
			win.Hide()
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

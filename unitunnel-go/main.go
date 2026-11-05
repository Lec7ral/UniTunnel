//go:build !android

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
	"unitunnel/pkg/config"
	"unitunnel/pkg/logger"
	"unitunnel/pkg/service"
	"unitunnel/pkg/singleinstance"
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

	// 0b. Single-instance acquisition (Windows mutex or Android activity lifecycle)
	instanceLock, firstInstance := acquirePlatformInstanceLock()
	if !firstInstance {
		log.Printf("[INFO] %s ya está en ejecución; se enfocó la ventana existente.", singleinstance.Describe(singleInstanceMutex))
		return
	}
	if instanceLock != nil {
		defer instanceLock.Release()
	}

	// Global Panic Recovery Handler: captures stack trace to log file if runtime panics occur
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL PANIC DETECTED] Runtime error: %v\nStack Trace:\n%s", r, debug.Stack())
			logger.Flush()

			exe, err := os.Executable()
			if err == nil {
				crashFile := filepath.Join(filepath.Dir(exe), "unitunnel_fatal_crash.txt")
				report := fmt.Sprintf("UniTunnel Crash Report\nTime: %s\nLog file: %s\nError: %v\n\nStack:\n%s",
					time.Now().Format(time.RFC3339), logPath, r, debug.Stack())
				_ = os.WriteFile(crashFile, []byte(report), 0644)
			}
		}
	}()

	log.Println("[DEBUG] Step 1-2: Initializing platform-specific environment...")
	initPlatformPreStartup()

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

	// Platform-specific post-config initialization (Windows autostart/cleanup or Android mobile setup)
	initPlatformPostConfig(cfg, appService)

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

	log.Println("[DEBUG] Step 7: Creating Main Webview Window...")
	winOpts := application.WebviewWindowOptions{
		Title:          windowTitle,
		URL:            "/",
		BackgroundType: application.BackgroundTypeSolid,
	}
	setupPlatformWindowOptions(&winOpts)
	win := app.NewWebviewWindowWithOptions(winOpts)

	if win == nil {
		log.Println("[ERROR] app.NewWebviewWindowWithOptions returned nil window! Aborting startup.")
		logger.Flush()
		return
	}
	win.Center()
	log.Println("[DEBUG] Webview Window initialized and centered successfully.")

	// Register close hook to hide window to system tray or background
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		log.Println("[DEBUG] Window closing event intercepted -> Hiding window")
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

	// Platform-specific system tray setup (Windows tray menu or mobile notification binding)
	setupPlatformTray(app, win, appService, cfg)

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

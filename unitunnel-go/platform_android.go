//go:build android

package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"unitunnel/pkg/config"
	"unitunnel/pkg/service"
	"unitunnel/pkg/singleinstance"
)

type dummyLock struct{}

func (d dummyLock) Release() {}

func initPlatformPreStartup() {
	log.Println("[ANDROID] Step 1-2: Initializing UniTunnel Android mobile runtime")
}

func acquirePlatformInstanceLock() (singleinstance.Lock, bool) {
	// Android handles single-instance lifecycle via Activity launchMode (singleTop / singleTask)
	return dummyLock{}, true
}

func initPlatformPostConfig(cfg *config.Config, appService *service.AppService) {
	log.Printf("[ANDROID] Mobile configuration active. ActiveAccount=%s, Proxy=%s:%d",
		cfg.ActiveAccountID, cfg.ProxyHost, cfg.ProxyPort)
}

func setupPlatformWindowOptions(opts *application.WebviewWindowOptions) {
	// Mobile viewport fills Android screen container
	opts.DisableResize = false
	opts.Frameless = true
}

func setupPlatformTray(app *application.App, win *application.WebviewWindow, appService *service.AppService, cfg *config.Config) {
	// Android uses ForegroundService notification rather than a desktop system tray
	log.Println("[ANDROID] Desktop system tray omitted on Android (managed by UniTunnelVpnService notification)")
}

//go:build !windows && !android

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
	log.Println("[POSIX] Generic platform pre-startup")
}

func acquirePlatformInstanceLock() (singleinstance.Lock, bool) {
	return dummyLock{}, true
}

func initPlatformPostConfig(cfg *config.Config, appService *service.AppService) {
	log.Println("[POSIX] Generic platform post-config")
}

func setupPlatformWindowOptions(opts *application.WebviewWindowOptions) {
	opts.Width = 380
	opts.Height = 680
	opts.Frameless = true
}

func setupPlatformTray(app *application.App, win *application.WebviewWindow, appService *service.AppService, cfg *config.Config) {
	log.Println("[POSIX] System tray omitted on non-Windows/non-Android platform")
}

//go:build android

package main

import (
	"log"

	"unitunnel/pkg/config"
	"unitunnel/pkg/service"
	"unitunnel/pkg/singleinstance"
)

func initPlatformPreStartup() {
	log.Println("[ANDROID] Step 1-2: Initializing UniTunnel Android mobile runtime")
}

func acquirePlatformInstanceLock() (*singleinstance.Lock, bool) {
	// Android handles single-instance lifecycle via Activity launchMode (singleTop / singleTask)
	return nil, true
}

func initPlatformPostConfig(cfg *config.Config, appService *service.AppService) {
	log.Printf("[ANDROID] Mobile configuration active. ActiveAccount=%s, Proxy=%s:%d",
		cfg.ActiveAccountID, cfg.ProxyHost, cfg.ProxyPort)
}

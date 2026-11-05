//go:build android

package main

import (
	"log"

	"unitunnel/pkg/config"
	"unitunnel/pkg/logger"
	"unitunnel/pkg/service"
)

var globalAppService *service.AppService

func main() {
	// Required main entry point for C-shared buildmode (-buildmode=c-shared)
	log.Println("[ANDROID-NATIVE] UniTunnel Go Core shared library loaded.")
}

func init() {
	log.Println("[ANDROID-NATIVE] Initializing UniTunnel Go Core runtime for Android...")
	logPath := logger.InitLogger()
	log.Printf("[ANDROID-NATIVE] Logger initialized at: %s", logPath)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("[ANDROID-NATIVE] Defaulting config: %v", err)
		cfg = config.DefaultConfig()
	}

	globalAppService = service.NewAppService(cfg)
	log.Println("[ANDROID-NATIVE] AppService initialized successfully.")
}

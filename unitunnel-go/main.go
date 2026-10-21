package main

import (
	"log"

	"unitunnel/pkg/config"
	"unitunnel/pkg/elevation"
	"unitunnel/ui"
)

func main() {
	// 1. Ensure administrator privilege on Windows immediately and silently
	elevation.EnsureElevated()

	// 2. Load existing or default configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Advertencia cargando config.json: %v", err)
		cfg = config.DefaultConfig()
	}

	// 3. Initialize Fyne application
	gui := ui.NewGUI(cfg)
	gui.Setup()
	gui.ShowAndRun()
}

package main

import (
	"log"

	"unitunnel/pkg/config"
	"unitunnel/ui"
)

func main() {
	// Load existing or default configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Advertencia cargando config.json: %v", err)
		cfg = config.DefaultConfig()
	}

	// Initialize Fyne application
	gui := ui.NewGUI(cfg)
	gui.Setup()
	gui.ShowAndRun()
}

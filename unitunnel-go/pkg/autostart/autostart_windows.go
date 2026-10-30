//go:build windows

// Package autostart keeps UniTunnel registered in the per-user Run key so the
// "Iniciar con Windows" setting actually does something.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "UniTunnel"
)

// Apply enables or disables launch-at-logon for the current executable.
//
// It always rewrites the value when enabling, so moving the .exe does not leave
// a Run entry pointing at a path that no longer exists.
func Apply(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("no se pudo abrir la clave Run del registro: %w", err)
	}
	defer k.Close()

	if !enabled {
		if err := k.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("no se pudo eliminar el arranque automático: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo determinar la ruta del ejecutable: %w", err)
	}
	exe = filepath.Clean(exe)

	// Quote the path: "C:\Program Files\..." contains spaces.
	if err := k.SetStringValue(valueName, `"`+exe+`"`); err != nil {
		return fmt.Errorf("no se pudo registrar el arranque automático: %w", err)
	}
	return nil
}

// Enabled reports whether the Run entry exists.
func Enabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(valueName)
	return err == nil
}

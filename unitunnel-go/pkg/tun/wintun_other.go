//go:build !windows

package tun

import (
	"unitunnel/pkg/config"
)

type WintunDevice struct {
	Name string
}

func OpenWintun(cfg *config.Config, onLog func(string)) (*WintunDevice, error) {
	return &WintunDevice{Name: "stub"}, nil
}

func (w *WintunDevice) Close() error {
	return nil
}

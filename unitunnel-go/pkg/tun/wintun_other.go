//go:build !windows

package tun

import (
	"unitunnel/pkg/config"
)

type WintunDevice struct {
	Name string
}

func OpenWintun(cfg *config.Config) (*WintunDevice, error) {
	return &WintunDevice{Name: "stub"}, nil
}

func (w *WintunDevice) Close() error {
	return nil
}

//go:build windows

package sysproxy

import (
	"fmt"
	"syscall"
	"golang.org/x/sys/windows/registry"
)

var (
	wininet                 = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW  = wininet.NewProc("InternetSetOptionW")
)

const (
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

// EnableSystemProxy sets Windows system proxy to 127.0.0.1:port
func EnableSystemProxy(port int) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("error al abrir registro de Windows: %w", err)
	}
	defer k.Close()

	// Enable proxy: ProxyEnable = 1
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}

	// Set proxy server: ProxyServer = "http=127.0.0.1:8081;https=127.0.0.1:8081;socks=127.0.0.1:1080"
	proxyVal := fmt.Sprintf("socks=127.0.0.1:%d", port)
	if err := k.SetStringValue("ProxyServer", proxyVal); err != nil {
		return err
	}

	// Bypass local addresses
	_ = k.SetStringValue("ProxyOverride", "localhost;127.*;10.*;192.168.*;<local>")

	// Refresh system settings
	refreshWinINet()
	return nil
}

// DisableSystemProxy turns off Windows system proxy
func DisableSystemProxy() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	_ = k.SetDWordValue("ProxyEnable", 0)
	refreshWinINet()
	return nil
}

func refreshWinINet() {
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_SETTINGS_CHANGED, 0, 0)
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_REFRESH, 0, 0)
}

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

// EnableSystemProxy sets Windows system proxy to local HTTP (8081) and SOCKS5 (1080)
func EnableSystemProxy(httpPort, socksPort int) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("error al abrir registro de Windows: %w", err)
	}
	defer k.Close()

	// Enable proxy: ProxyEnable = 1
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}

	// Set composite proxy server: HTTP, HTTPS and SOCKS
	var proxyVal string
	if httpPort > 0 && socksPort > 0 {
		proxyVal = fmt.Sprintf("http=127.0.0.1:%d;https=127.0.0.1:%d;socks=127.0.0.1:%d", httpPort, httpPort, socksPort)
	} else if httpPort > 0 {
		proxyVal = fmt.Sprintf("127.0.0.1:%d", httpPort)
	} else {
		proxyVal = fmt.Sprintf("socks=127.0.0.1:%d", socksPort)
	}

	if err := k.SetStringValue("ProxyServer", proxyVal); err != nil {
		return err
	}

	// Bypass local addresses
	_ = k.SetStringValue("ProxyOverride", "localhost;127.*;10.*;192.168.*;172.16.*;<local>")

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

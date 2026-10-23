//go:build windows

package sysproxy

import (
	"fmt"
	"os/exec"
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

	// Enable UWP AppContainer Loopback Exemption for Microsoft Store
	EnableUWPStoreExemption()

	// Automatically set console environment variables (CMD, PowerShell, Git, curl, npm, pip)
	SetConsoleEnvironmentProxies(httpPort, socksPort)
	return nil
}

// EnableUWPStoreExemption exempts Microsoft Store and common Windows UWP apps from AppContainer loopback restrictions
func EnableUWPStoreExemption() {
	cmd := exec.Command("CheckNetIsolation.exe", "LoopbackExempt", "-a", "-n=Microsoft.WindowsStore_8wekyb3d8bbwe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = cmd.Run()
}

// SetConsoleEnvironmentProxies sets HTTP_PROXY and ALL_PROXY in Windows User Environment
func SetConsoleEnvironmentProxies(httpPort, socksPort int) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	httpURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)

	_ = k.SetStringValue("HTTP_PROXY", httpURL)
	_ = k.SetStringValue("HTTPS_PROXY", httpURL)
	_ = k.SetStringValue("ALL_PROXY", socksURL)
	_ = k.SetStringValue("http_proxy", httpURL)
	_ = k.SetStringValue("https_proxy", httpURL)
	_ = k.SetStringValue("all_proxy", socksURL)
}

// RemoveConsoleEnvironmentProxies cleans up console proxy variables from User Environment
func RemoveConsoleEnvironmentProxies() {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	_ = k.DeleteValue("HTTP_PROXY")
	_ = k.DeleteValue("HTTPS_PROXY")
	_ = k.DeleteValue("ALL_PROXY")
	_ = k.DeleteValue("http_proxy")
	_ = k.DeleteValue("https_proxy")
	_ = k.DeleteValue("all_proxy")
}

// DisableSystemProxy turns off Windows system proxy and clears console environment
func DisableSystemProxy() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	_ = k.SetDWordValue("ProxyEnable", 0)
	refreshWinINet()

	// Clear console environment variables
	RemoveConsoleEnvironmentProxies()
	return nil
}

func refreshWinINet() {
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_SETTINGS_CHANGED, 0, 0)
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_REFRESH, 0, 0)
}

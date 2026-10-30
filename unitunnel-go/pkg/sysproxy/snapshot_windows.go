//go:build windows

package sysproxy

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/registry"

	"unitunnel/pkg/appstate"
)

const (
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	environmentKey      = `Environment`
	proxyStateFile      = "proxy_state.json"
)

// consoleProxyVars are the user environment variables UniTunnel overwrites when
// the system proxy is enabled. They must be restored too, or every new console
// keeps pointing at a dead 127.0.0.1:8081 after disconnecting.
var consoleProxyVars = []string{
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "all_proxy",
}

// registryValue records whether a value existed at all, so a value that was
// absent before we touched the registry can be deleted instead of zeroed.
type registryValue struct {
	Present bool   `json:"present"`
	Kind    string `json:"kind"` // "dword" or "string"
	Number  uint32 `json:"number,omitempty"`
	Text    string `json:"text,omitempty"`
}

// proxyState is the exact pre-existing proxy configuration, captured once
// before UniTunnel modifies anything.
type proxyState struct {
	CapturedAt    string                   `json:"captured_at"`
	ProxyEnable   registryValue            `json:"proxy_enable"`
	ProxyServer   registryValue            `json:"proxy_server"`
	ProxyOverride registryValue            `json:"proxy_override"`
	AutoConfigURL registryValue            `json:"auto_config_url"`
	Env           map[string]registryValue `json:"env"`
}

func readStringValue(k registry.Key, name string) registryValue {
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return registryValue{Present: false, Kind: "string"}
	}
	return registryValue{Present: true, Kind: "string", Text: v}
}

func readDWordValue(k registry.Key, name string) registryValue {
	v, _, err := k.GetIntegerValue(name)
	if err != nil {
		return registryValue{Present: false, Kind: "dword"}
	}
	return registryValue{Present: true, Kind: "dword", Number: uint32(v)}
}

// captureProxyState reads the current proxy + console-env configuration.
func captureProxyState() (*proxyState, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("error al leer la configuración de proxy de Windows: %w", err)
	}
	defer k.Close()

	state := &proxyState{
		CapturedAt:    time.Now().Format(time.RFC3339),
		ProxyEnable:   readDWordValue(k, "ProxyEnable"),
		ProxyServer:   readStringValue(k, "ProxyServer"),
		ProxyOverride: readStringValue(k, "ProxyOverride"),
		AutoConfigURL: readStringValue(k, "AutoConfigURL"),
		Env:           make(map[string]registryValue, len(consoleProxyVars)),
	}

	if envKey, err := registry.OpenKey(registry.CURRENT_USER, environmentKey, registry.QUERY_VALUE); err == nil {
		defer envKey.Close()
		for _, name := range consoleProxyVars {
			state.Env[name] = readStringValue(envKey, name)
		}
	}
	return state, nil
}

// persistProxyStateIfMissing saves the pre-existing configuration once. Later
// connects must not overwrite it, otherwise the "original" state would become
// UniTunnel's own 127.0.0.1 proxy.
func persistProxyStateIfMissing() error {
	exists, err := appstate.Exists(proxyStateFile)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	state, err := captureProxyState()
	if err != nil {
		return err
	}
	return appstate.WriteJSON(proxyStateFile, state)
}

func loadProxyState() (*proxyState, error) {
	var state proxyState
	if err := appstate.ReadJSON(proxyStateFile, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func applyDWord(k registry.Key, name string, v registryValue) error {
	if !v.Present {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetDWordValue(name, v.Number)
}

func applyString(k registry.Key, name string, v registryValue) error {
	if !v.Present {
		if err := k.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(name, v.Text)
}

// restore puts back every value captured by captureProxyState.
func (s *proxyState) restore() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("error al abrir el registro para restaurar el proxy: %w", err)
	}
	defer k.Close()

	if err := applyDWord(k, "ProxyEnable", s.ProxyEnable); err != nil {
		return err
	}
	if err := applyString(k, "ProxyServer", s.ProxyServer); err != nil {
		return err
	}
	if err := applyString(k, "ProxyOverride", s.ProxyOverride); err != nil {
		return err
	}
	if err := applyString(k, "AutoConfigURL", s.AutoConfigURL); err != nil {
		return err
	}

	if len(s.Env) > 0 {
		if envKey, err := registry.OpenKey(registry.CURRENT_USER, environmentKey, registry.SET_VALUE|registry.QUERY_VALUE); err == nil {
			for name, v := range s.Env {
				_ = applyString(envKey, name, v)
			}
			envKey.Close()
		}
	}

	refreshWinINet()
	return nil
}

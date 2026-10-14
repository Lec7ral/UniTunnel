package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"unitunnel/pkg/auth"
	"unitunnel/pkg/config"
	"unitunnel/pkg/sysproxy"
	"unitunnel/pkg/tun"
)

type GUI struct {
	app        fyne.App
	window     fyne.Window
	cfg        *config.Config
	engine     *tun.Engine
	logBinding *widget.Entry
	statusLbl  *widget.Label
	toggleBtn  *widget.Button
}

func NewGUI(cfg *config.Config) *GUI {
	myApp := app.NewWithID("com.uninet.unitunnel")
	myApp.Settings().SetTheme(theme.DarkTheme())
	w := myApp.NewWindow("UniTunnel - Proxy & TUN Manager")
	w.Resize(fyne.NewSize(620, 680))

	return &GUI{
		app:    myApp,
		window: w,
		cfg:    cfg,
	}
}

func (g *GUI) AppendLog(msg string) {
	ts := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s\n", ts, msg)
	if g.logBinding != nil {
		g.logBinding.SetText(g.logBinding.Text + entry)
		g.logBinding.CursorRow = strings.Count(g.logBinding.Text, "\n")
	}
}

func (g *GUI) Setup() {
	// Status Header
	g.statusLbl = widget.NewLabel("ESTADO: DESCONECTADO")
	g.statusLbl.TextStyle = fyne.TextStyle{Bold: true}

	// Inputs
	proxyHostEntry := widget.NewEntry()
	proxyHostEntry.SetText(g.cfg.ProxyHost)

	proxyPortEntry := widget.NewEntry()
	proxyPortEntry.SetText(strconv.Itoa(g.cfg.ProxyPort))

	userEntry := widget.NewEntry()
	userEntry.SetText(g.cfg.Username)

	domainEntry := widget.NewEntry()
	domainEntry.SetPlaceHolder("Opcional (ej: FACULTAD)")
	domainEntry.SetText(g.cfg.Domain)

	passEntry := widget.NewPasswordEntry()
	passEntry.SetText(g.cfg.Password)

	// Auth Scheme Selector
	authSelect := widget.NewSelect([]string{"Auto-detectar", "NTLM", "Basic", "Sin autenticación"}, func(s string) {
		switch s {
		case "Auto-detectar":
			g.cfg.AuthType = "auto"
		case "NTLM":
			g.cfg.AuthType = "ntlm"
		case "Basic":
			g.cfg.AuthType = "basic"
		case "Sin autenticación":
			g.cfg.AuthType = "none"
		}
	})
	authSelect.SetSelected("Auto-detectar")

	// Capture Mode Radio
	captureRadio := widget.NewRadioGroup([]string{
		"Modo TUN Virtual (Wintun) - 100% Apps (Recomendado)",
		"Modo Proxy de Sistema (Solo navegadores y apps HTTP)",
	}, func(selected string) {
		if strings.HasPrefix(selected, "Modo TUN") {
			g.cfg.CaptureMode = "tun"
		} else {
			g.cfg.CaptureMode = "system_proxy"
		}
	})
	if g.cfg.CaptureMode == "tun" {
		captureRadio.SetSelected("Modo TUN Virtual (Wintun) - 100% Apps (Recomendado)")
	} else {
		captureRadio.SetSelected("Modo Proxy de Sistema (Solo navegadores y apps HTTP)")
	}

	// Auto-Probe Button
	probeBtn := widget.NewButtonWithIcon("Diagnosticar Proxy (10.0.0.1)", theme.SearchIcon(), func() {
		host := proxyHostEntry.Text
		port, _ := strconv.Atoi(proxyPortEntry.Text)
		addr := fmt.Sprintf("%s:%d", host, port)

		g.AppendLog(fmt.Sprintf("Iniciando sonda de diagnóstico hacia %s...", addr))
		go func() {
			res, err := auth.ProbeProxy(addr, 5*time.Second)
			if err != nil {
				g.AppendLog(fmt.Sprintf("Error al sondear proxy: %v", err))
				dialog.ShowError(err, g.window)
				return
			}

			info := fmt.Sprintf("Proxy respondio correctamente.\nEsquema recomendado: %s\nServidor: %s\nCabeceras detectadas:\n%s",
				res.PrimaryType, res.ProxyServer, strings.Join(res.RawHeaders, "\n"))
			g.AppendLog(fmt.Sprintf("Diagnóstico completado: Esquema=%s", res.PrimaryType))

			if res.PrimaryType == auth.AuthTypeNTLM {
				authSelect.SetSelected("NTLM")
			} else if res.PrimaryType == auth.AuthTypeBasic {
				authSelect.SetSelected("Basic")
			}

			dialog.ShowInformation("Diagnóstico de Proxy", info, g.window)
		}()
	})

	// Toggle Connect Button
	g.toggleBtn = widget.NewButtonWithIcon("CONECTAR TÚNEL", theme.MediaPlayIcon(), func() {
		g.handleToggle(proxyHostEntry.Text, proxyPortEntry.Text, userEntry.Text, domainEntry.Text, passEntry.Text)
	})
	g.toggleBtn.Importance = widget.HighImportance

	// Log Entry Box
	g.logBinding = widget.NewMultiLineEntry()
	g.logBinding.SetMinRowsVisible(8)
	g.logBinding.TextStyle = fyne.TextStyle{Monospace: true}

	// Layout arrangement
	proxyForm := widget.NewForm(
		widget.NewFormItem("Servidor Proxy:", proxyHostEntry),
		widget.NewFormItem("Puerto:", proxyPortEntry),
		widget.NewFormItem("Autenticación:", authSelect),
		widget.NewFormItem("Usuario:", userEntry),
		widget.NewFormItem("Dominio (NTLM):", domainEntry),
		widget.NewFormItem("Contraseña:", passEntry),
	)

	content := container.NewVBox(
		container.NewHBox(g.statusLbl),
		widget.NewSeparator(),
		widget.NewCard("Configuración del Proxy Universitario", "", container.NewVBox(
			proxyForm,
			container.NewHBox(probeBtn),
		)),
		widget.NewCard("Modo de Captura de Tráfico", "", container.NewVBox(
			captureRadio,
		)),
		g.toggleBtn,
		widget.NewSeparator(),
		widget.NewLabel("Registro de Conexión en Vivo:"),
		g.logBinding,
	)

	scroll := container.NewVScroll(content)
	g.window.SetContent(scroll)
	g.AppendLog("UniTunnel iniciado. Listo para conectar.")
}

func (g *GUI) handleToggle(host, portStr, user, domain, pass string) {
	if g.engine != nil && g.engine.IsRunning() {
		// Stop tunnel
		g.engine.Stop()
		if g.cfg.CaptureMode == "system_proxy" {
			_ = sysproxy.DisableSystemProxy()
			g.AppendLog("✓ Proxy de Windows restaurado.")
		}
		g.statusLbl.SetText("ESTADO: DESCONECTADO")
		g.toggleBtn.SetText("CONECTAR TÚNEL")
		g.toggleBtn.SetIcon(theme.MediaPlayIcon())
		return
	}

	// Update config from inputs
	g.cfg.ProxyHost = host
	p, err := strconv.Atoi(portStr)
	if err == nil {
		g.cfg.ProxyPort = p
	}
	g.cfg.Username = user
	g.cfg.Domain = domain
	g.cfg.Password = pass
	_ = config.SaveConfig(g.cfg)

	g.statusLbl.SetText("ESTADO: CONECTANDO...")
	g.AppendLog(fmt.Sprintf("Iniciando túnel hacia %s:%d (Modo: %s)...", g.cfg.ProxyHost, g.cfg.ProxyPort, g.cfg.CaptureMode))

	g.engine = tun.NewEngine(g.cfg, func(msg string) {
		g.AppendLog(msg)
	})

	if err := g.engine.Start(); err != nil {
		g.AppendLog(fmt.Sprintf("Error al iniciar: %v", err))
		dialog.ShowError(err, g.window)
		g.statusLbl.SetText("ESTADO: ERROR DE CONEXIÓN")
		return
	}

	// If system proxy mode is chosen, apply to Windows
	if g.cfg.CaptureMode == "system_proxy" {
		if err := sysproxy.EnableSystemProxy(g.cfg.LocalSocksPort); err != nil {
			g.AppendLog(fmt.Sprintf("Advertencia al aplicar proxy de Windows: %v", err))
		} else {
			g.AppendLog(fmt.Sprintf("✓ Proxy de Windows asignado a 127.0.0.1:%d", g.cfg.LocalSocksPort))
		}
	}

	g.statusLbl.SetText("ESTADO: CONECTADO (TÚNEL ACTIVO)")
	g.toggleBtn.SetText("DESCONECTAR TÚNEL")
	g.toggleBtn.SetIcon(theme.MediaStopIcon())
}

func (g *GUI) ShowAndRun() {
	g.window.ShowAndRun()
}

package ui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"unitunnel/assets"
	"unitunnel/pkg/auth"
	"unitunnel/pkg/config"
	"unitunnel/pkg/sysproxy"
	"unitunnel/pkg/traffic"
	"unitunnel/pkg/tun"
)

// GUI represents the native Fyne desktop application.
type GUI struct {
	app              fyne.App
	window           fyne.Window
	cfg              *config.Config
	engine           *tun.Engine
	logText          string
	logWidget        *widget.Entry
	statusText       *canvas.Text
	connectBtn       *widget.Button
	accountSelect    *widget.Select
	accountDetails   *widget.Label
	trayConnect      *fyne.MenuItem
	trayMenu         *fyne.Menu
	pingCancel       context.CancelFunc
	chart            *RealtimeChart
	speedMonitorStop chan struct{}

	// Inline collapsible drawer panels
	panelHolder      *fyne.Container
	settingsPanel    *fyne.Container
	logsPanel        *fyne.Container
	settingsVisible  bool
	logsVisible      bool
	proxyTargetLabel *widget.Label

	// Settings fields
	proxyHostEntry    *widget.Entry
	proxyPortEntry    *widget.Entry
	authTypeSelect    *widget.Select
	captureModeSelect *widget.Select
	vpsCheck          *widget.Check
	vpsHostEntry      *widget.Entry
}

// NewGUI initializes the Fyne application and creates the vertically proportioned window.
func NewGUI(cfg *config.Config) *GUI {
	myApp := app.NewWithID("com.uninet.unitunnel")
	myApp.Settings().SetTheme(theme.DarkTheme())

	w := myApp.NewWindow("UniTunnel Connect")

	// Set custom icon embedded directly into the Go executable binary
	w.SetIcon(assets.AppIconResource)
	myApp.SetIcon(assets.AppIconResource)

	// Modern vertical dimensions (365x640) matching OpenVPN Connect & Cloudflare WARP
	w.Resize(fyne.NewSize(365, 640))
	w.SetFixedSize(true)

	return &GUI{
		app:    myApp,
		window: w,
		cfg:    cfg,
	}
}

// AppendLog adds a timestamped message to the live log buffer.
func (g *GUI) AppendLog(msg string) {
	ts := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s\n", ts, msg)
	g.logText += entry
	if g.logWidget != nil {
		g.logWidget.SetText(g.logText)
		g.logWidget.CursorRow = strings.Count(g.logText, "\n")
	}
}

// Setup builds the user interface with inline drawers and bottom-pinned chart.
func (g *GUI) Setup() {
	g.setupSystemTray()

	// 1. Top Header Bar:
	// [Settings (Gear)] <------ "UniTunnel" Header ------> [Logs (Doc)]
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		g.toggleSettings()
	})
	logsBtn := widget.NewButtonWithIcon("", theme.DocumentIcon(), func() {
		g.toggleLogs()
	})

	appTitle := canvas.NewText("UniTunnel", color.NRGBA{R: 248, G: 250, B: 252, A: 255})
	appTitle.TextSize = 16
	appTitle.TextStyle = fyne.TextStyle{Bold: true}
	appTitle.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("TUNEL DE ALTA VELOCIDAD", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	subtitle.TextSize = 9
	subtitle.TextStyle = fyne.TextStyle{Bold: true}
	subtitle.Alignment = fyne.TextAlignCenter

	titleBox := container.NewVBox(
		container.NewCenter(appTitle),
		container.NewCenter(subtitle),
	)

	topBar := container.NewBorder(nil, nil, settingsBtn, logsBtn, titleBox)

	// 2. Prepare Inline Collapsible Panels
	g.setupSettingsPanel()
	g.setupLogsPanel()
	g.panelHolder = container.NewVBox(g.settingsPanel, g.logsPanel)
	g.panelHolder.Hide()

	// 3. Central OpenVPN-Style Connection Section
	g.statusText = canvas.NewText("DESCONECTADO", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	g.statusText.TextSize = 13
	g.statusText.TextStyle = fyne.TextStyle{Bold: true}
	g.statusText.Alignment = fyne.TextAlignCenter

	g.connectBtn = widget.NewButtonWithIcon("CONECTAR", theme.MediaPlayIcon(), func() {
		g.toggleConnection()
	})
	g.connectBtn.Importance = widget.HighImportance

	g.proxyTargetLabel = widget.NewLabel(g.getProxySummaryText())
	g.proxyTargetLabel.Alignment = fyne.TextAlignCenter
	g.proxyTargetLabel.TextStyle = fyne.TextStyle{Monospace: true}

	connectCard := container.NewVBox(
		container.NewCenter(g.statusText),
		container.NewPadded(g.connectBtn),
		container.NewCenter(g.proxyTargetLabel),
	)

	// 4. Account Profile Selector Section
	accountCard := g.setupAccountSection()

	// Scrollable middle body (Header -> Panels -> Connect Card -> Account Card)
	middleContent := container.NewVBox(
		container.NewPadded(connectCard),
		widget.NewSeparator(),
		container.NewPadded(accountCard),
	)

	// 5. Bottom Section: Real-time traffic & ping chart (Pinned at bottom)
	g.chart = NewRealtimeChart(330, 48)

	footerLabel := canvas.NewText("Wintun L3 · Proceso Oculto Admin · Cero Consolas", color.NRGBA{R: 100, G: 116, B: 139, A: 255})
	footerLabel.TextSize = 10
	footerLabel.Alignment = fyne.TextAlignCenter

	bottomSection := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(g.chart.Container),
		container.NewCenter(footerLabel),
	)

	// Top container combines TopBar + Inline Drawers
	topSection := container.NewVBox(
		topBar,
		widget.NewSeparator(),
		g.panelHolder,
	)

	// Assemble final window layout using Border layout to lock top & bottom
	mainLayout := container.NewBorder(
		topSection,
		bottomSection,
		nil,
		nil,
		container.NewVScroll(middleContent),
	)

	g.window.SetContent(mainLayout)

	// Intercept window close to stay resident in System Tray
	g.window.SetCloseIntercept(func() {
		g.window.Hide()
	})

	g.AppendLog("UniTunnel v2.4 iniciado con privilegios de Administrador.")
	g.AppendLog(fmt.Sprintf("Proxy configurado: %s:%d (%s)", g.cfg.ProxyHost, g.cfg.ProxyPort, g.cfg.AuthType))
}

func (g *GUI) getProxySummaryText() string {
	mode := "Wintun"
	if g.cfg.CaptureMode == "system_proxy" {
		mode = "ProxyWin"
	}
	return fmt.Sprintf("Proxy: %s:%d  ·  %s", g.cfg.ProxyHost, g.cfg.ProxyPort, mode)
}

// toggleSettings unfolds or collapses the Settings drawer panel inline.
func (g *GUI) toggleSettings() {
	if g.settingsVisible {
		g.settingsVisible = false
		g.settingsPanel.Hide()
		g.panelHolder.Hide()
		return
	}

	// Open Settings, close Logs
	g.logsVisible = false
	g.logsPanel.Hide()

	// Sync inputs with current config
	g.proxyHostEntry.SetText(g.cfg.ProxyHost)
	g.proxyPortEntry.SetText(strconv.Itoa(g.cfg.ProxyPort))
	g.authTypeSelect.SetSelected(g.cfg.AuthType)
	g.captureModeSelect.SetSelected(g.cfg.CaptureMode)
	g.vpsCheck.SetChecked(g.cfg.VPSEnabled)
	g.vpsHostEntry.SetText(g.cfg.VPSServer)

	g.settingsVisible = true
	g.settingsPanel.Show()
	g.panelHolder.Show()
}

// toggleLogs unfolds or collapses the Logs drawer panel inline.
func (g *GUI) toggleLogs() {
	if g.logsVisible {
		g.logsVisible = false
		g.logsPanel.Hide()
		g.panelHolder.Hide()
		return
	}

	// Open Logs, close Settings
	g.settingsVisible = false
	g.settingsPanel.Hide()

	if g.logWidget != nil {
		g.logWidget.SetText(g.logText)
		g.logWidget.CursorRow = strings.Count(g.logText, "\n")
	}

	g.logsVisible = true
	g.logsPanel.Show()
	g.panelHolder.Show()
}

// setupSettingsPanel builds the inline expandable settings view.
func (g *GUI) setupSettingsPanel() {
	g.proxyHostEntry = widget.NewEntry()
	g.proxyHostEntry.SetPlaceHolder("Ej: 10.0.0.1")

	g.proxyPortEntry = widget.NewEntry()
	g.proxyPortEntry.SetPlaceHolder("8080")

	g.authTypeSelect = widget.NewSelect([]string{"auto", "ntlm", "basic", "none"}, nil)
	g.captureModeSelect = widget.NewSelect([]string{"tun", "system_proxy"}, nil)

	g.vpsCheck = widget.NewCheck("Habilitar VPS de salida", func(b bool) {
		g.cfg.VPSEnabled = b
	})
	g.vpsHostEntry = widget.NewEntry()
	g.vpsHostEntry.SetPlaceHolder("vps.midominio.com:443")

	probeBtn := widget.NewButtonWithIcon("Sondear Proxy", theme.SearchIcon(), func() {
		addr := fmt.Sprintf("%s:%s", g.proxyHostEntry.Text, g.proxyPortEntry.Text)
		g.AppendLog(fmt.Sprintf("Sondeando proxy %s...", addr))
		go func() {
			p, _ := strconv.Atoi(g.proxyPortEntry.Text)
			res, err := auth.ProbeProxy(fmt.Sprintf("%s:%d", g.proxyHostEntry.Text, p), 5*time.Second)
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			msg := fmt.Sprintf("Esquema detectado: %s\nServidor: %s\nCabeceras: %s",
				res.PrimaryType, res.ProxyServer, strings.Join(res.RawHeaders, "\n"))
			dialog.ShowInformation("Resultado de Sonda", msg, g.window)
			if res.PrimaryType == auth.AuthTypeNTLM {
				g.authTypeSelect.SetSelected("ntlm")
			} else if res.PrimaryType == auth.AuthTypeBasic {
				g.authTypeSelect.SetSelected("basic")
			}
		}()
	})

	saveBtn := widget.NewButtonWithIcon("Guardar", theme.DocumentSaveIcon(), func() {
		g.cfg.ProxyHost = g.proxyHostEntry.Text
		if p, err := strconv.Atoi(g.proxyPortEntry.Text); err == nil {
			g.cfg.ProxyPort = p
		}
		g.cfg.AuthType = g.authTypeSelect.Selected
		g.cfg.CaptureMode = g.captureModeSelect.Selected
		g.cfg.VPSServer = g.vpsHostEntry.Text

		_ = config.SaveConfig(g.cfg)
		g.proxyTargetLabel.SetText(g.getProxySummaryText())
		g.AppendLog("✓ Ajustes guardados correctamente en config.json.")
		g.toggleSettings() // collapse drawer
	})
	saveBtn.Importance = widget.HighImportance

	closeBtn := widget.NewButtonWithIcon("Cerrar", theme.CancelIcon(), func() {
		g.toggleSettings()
	})

	header := container.NewBorder(
		nil,
		nil,
		widget.NewLabelWithStyle("⚙ Ajustes de Conexión", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		closeBtn,
	)

	form := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Servidor Proxy:", g.proxyHostEntry),
			widget.NewFormItem("Puerto:", g.proxyPortEntry),
			widget.NewFormItem("Autenticación:", g.authTypeSelect),
			widget.NewFormItem("Modo Captura:", g.captureModeSelect),
		),
		container.NewHBox(probeBtn),
		widget.NewSeparator(),
		g.vpsCheck,
		widget.NewForm(widget.NewFormItem("VPS:", g.vpsHostEntry)),
		widget.NewSeparator(),
		container.NewHBox(saveBtn, closeBtn),
	)

	card := widget.NewCard("", "", container.NewVBox(header, form))
	g.settingsPanel = container.NewVBox(card, widget.NewSeparator())
	g.settingsPanel.Hide()
}

// setupLogsPanel builds the inline live console log view.
func (g *GUI) setupLogsPanel() {
	g.logWidget = widget.NewMultiLineEntry()
	g.logWidget.TextStyle = fyne.TextStyle{Monospace: true}
	g.logWidget.Wrapping = fyne.TextWrapWord
	g.logWidget.SetText(g.logText)

	copyBtn := widget.NewButtonWithIcon("Copiar", theme.ContentCopyIcon(), func() {
		g.window.Clipboard().SetContent(g.logText)
		g.AppendLog("Logs copiados al portapapeles.")
	})

	clearBtn := widget.NewButtonWithIcon("Limpiar", theme.ContentClearIcon(), func() {
		g.logText = ""
		g.logWidget.SetText("")
	})

	closeBtn := widget.NewButtonWithIcon("Cerrar", theme.CancelIcon(), func() {
		g.toggleLogs()
	})

	header := container.NewBorder(
		nil,
		nil,
		widget.NewLabelWithStyle("📋 Registro de Conexión", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		closeBtn,
	)

	actions := container.NewHBox(copyBtn, clearBtn)

	// Set fixed height scroll for the log viewer
	logScroll := container.NewVScroll(g.logWidget)
	logScroll.SetMinSize(fyne.NewSize(330, 160))

	card := widget.NewCard("", "", container.NewVBox(
		header,
		logScroll,
		actions,
	))

	g.logsPanel = container.NewVBox(card, widget.NewSeparator())
	g.logsPanel.Hide()
}

func (g *GUI) setupAccountSection() *fyne.Container {
	var accountNames []string
	for _, acc := range g.cfg.Accounts {
		accountNames = append(accountNames, acc.Name)
	}

	active := g.cfg.GetActiveAccount()
	activeName := ""
	if active != nil {
		activeName = active.Name
	}

	g.accountDetails = widget.NewLabel("")
	g.updateAccountDetailsText(active)

	g.accountSelect = widget.NewSelect(accountNames, func(selected string) {
		for _, acc := range g.cfg.Accounts {
			if acc.Name == selected {
				g.cfg.SetActiveAccount(acc.ID)
				_ = config.SaveConfig(g.cfg)
				g.updateAccountDetailsText(&acc)
				g.AppendLog(fmt.Sprintf("Cuenta cambiada a: %s (%s)", acc.Name, acc.Username))
				break
			}
		}
	})

	if activeName != "" {
		g.accountSelect.SetSelected(activeName)
	}

	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		g.showAddAccountDialog()
	})

	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		g.showDeleteAccountDialog()
	})

	header := widget.NewLabelWithStyle("PERFIL DE USUARIO", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	selectRow := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(addBtn, delBtn),
		g.accountSelect,
	)

	card := widget.NewCard("", "", container.NewVBox(
		header,
		selectRow,
		container.NewCenter(g.accountDetails),
	))

	return container.NewVBox(card)
}

func (g *GUI) updateAccountDetailsText(acc *config.UserAccount) {
	if acc == nil || acc.Username == "" {
		g.accountDetails.SetText("(Sin credenciales configuradas)")
	} else {
		dom := ""
		if acc.Domain != "" {
			dom = acc.Domain + `\`
		}
		g.accountDetails.SetText(fmt.Sprintf("%s%s · Contraseña guardada", dom, acc.Username))
	}
}

func (g *GUI) refreshAccountList() {
	var names []string
	for _, acc := range g.cfg.Accounts {
		names = append(names, acc.Name)
	}
	g.accountSelect.Options = names
	active := g.cfg.GetActiveAccount()
	if active != nil {
		g.accountSelect.SetSelected(active.Name)
		g.updateAccountDetailsText(active)
	}
}

func (g *GUI) showAddAccountDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Ej: Cuenta Personal")

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("Usuario de la universidad")

	domainEntry := widget.NewEntry()
	domainEntry.SetPlaceHolder("Opcional (Ej: FACULTAD)")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Contraseña")

	form := widget.NewForm(
		widget.NewFormItem("Nombre del Perfil:", nameEntry),
		widget.NewFormItem("Usuario:", userEntry),
		widget.NewFormItem("Dominio (NTLM):", domainEntry),
		widget.NewFormItem("Contraseña:", passEntry),
	)

	d := dialog.NewCustomConfirm("Añadir Cuenta Universitaria", "Guardar", "Cancelar", form, func(confirmed bool) {
		if !confirmed || userEntry.Text == "" {
			return
		}
		acc := g.cfg.AddAccount(nameEntry.Text, userEntry.Text, domainEntry.Text, passEntry.Text)
		_ = config.SaveConfig(g.cfg)
		g.refreshAccountList()
		g.AppendLog(fmt.Sprintf("✓ Cuenta '%s' agregada y guardada.", acc.Name))
	}, g.window)

	d.Resize(fyne.NewSize(340, 260))
	d.Show()
}

func (g *GUI) showDeleteAccountDialog() {
	active := g.cfg.GetActiveAccount()
	if active == nil {
		return
	}
	if len(g.cfg.Accounts) <= 1 {
		dialog.ShowInformation("Aviso", "No puedes eliminar la única cuenta existente.", g.window)
		return
	}

	msg := fmt.Sprintf("¿Deseas eliminar el perfil '%s' (%s)?", active.Name, active.Username)
	dialog.ShowConfirm("Eliminar Cuenta", msg, func(confirmed bool) {
		if confirmed {
			g.cfg.DeleteAccount(active.ID)
			_ = config.SaveConfig(g.cfg)
			g.refreshAccountList()
			g.AppendLog(fmt.Sprintf("✓ Cuenta '%s' eliminada.", active.Name))
		}
	}, g.window)
}

func (g *GUI) toggleConnection() {
	if g.engine != nil && g.engine.IsRunning() {
		// Stop connection
		g.engine.Stop()
		if g.cfg.CaptureMode == "system_proxy" {
			_ = sysproxy.DisableSystemProxy()
		}
		if g.speedMonitorStop != nil {
			close(g.speedMonitorStop)
			g.speedMonitorStop = nil
		}
		if g.chart != nil {
			g.chart.AddData(0, 0)
		}
		if g.pingCancel != nil {
			g.pingCancel()
			g.pingCancel = nil
		}

		g.statusText.Text = "DESCONECTADO"
		g.statusText.Color = color.NRGBA{R: 148, G: 163, B: 184, A: 255}
		g.statusText.Refresh()

		g.connectBtn.SetText("CONECTAR")
		g.connectBtn.SetIcon(theme.MediaPlayIcon())
		g.connectBtn.Importance = widget.HighImportance

		g.updateTrayState("disconnected")
		return
	}

	// Verify active account has credentials
	acc := g.cfg.GetActiveAccount()
	if acc == nil || acc.Username == "" {
		dialog.ShowInformation("Faltan Credenciales", "Por favor ingresa usuario y contraseña en el selector de cuentas.", g.window)
		return
	}

	g.statusText.Text = "CONECTANDO..."
	g.statusText.Color = color.NRGBA{R: 245, G: 158, B: 11, A: 255} // Amber
	g.statusText.Refresh()

	g.connectBtn.SetText("CONECTANDO...")
	g.connectBtn.Importance = widget.WarningImportance
	g.updateTrayState("connecting")

	g.AppendLog(fmt.Sprintf("Iniciando túnel hacia %s:%d con usuario '%s'...", g.cfg.ProxyHost, g.cfg.ProxyPort, acc.Username))

	g.engine = tun.NewEngine(g.cfg, func(msg string) {
		g.AppendLog(msg)
	})

	if err := g.engine.Start(); err != nil {
		g.statusText.Text = "ERROR"
		g.statusText.Color = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
		g.statusText.Refresh()
		g.AppendLog(fmt.Sprintf("Fallo al conectar: %v", err))
		g.connectBtn.SetText("CONECTAR")
		g.connectBtn.SetIcon(theme.MediaPlayIcon())
		g.connectBtn.Importance = widget.HighImportance
		g.updateTrayState("disconnected")
		dialog.ShowError(err, g.window)
		return
	}

	if g.cfg.CaptureMode == "system_proxy" {
		_ = sysproxy.EnableSystemProxy(g.cfg.LocalSocksPort)
		g.AppendLog("✓ Proxy del sistema Windows configurado en 127.0.0.1:1080.")
	}

	g.statusText.Text = "CONECTADO · VERIFICANDO TRÁFICO..."
	g.statusText.Color = color.NRGBA{R: 52, G: 211, B: 153, A: 255} // Emerald Green
	g.statusText.Refresh()

	g.connectBtn.SetText("DESCONECTAR")
	g.connectBtn.SetIcon(theme.MediaStopIcon())
	g.connectBtn.Importance = widget.DangerImportance

	g.updateTrayState("connected")

	// Start real-time speed monitor feeding the line chart at the bottom
	g.speedMonitorStop = traffic.StartSpeedMonitor(1*time.Second, func(speedMBs float64, pingMS int64) {
		if g.chart != nil {
			g.chart.AddData(speedMBs, pingMS)
		}
	})

	// Start periodic ping health-check to verify real traffic flow through Wintun
	ctx, cancel := context.WithCancel(context.Background())
	g.pingCancel = cancel

	go tun.StartPeriodicPing(ctx, 5*time.Second, func(res tun.ProbeResult) {
		if res.Success {
			ms := res.RTT.Milliseconds()
			traffic.SetPing(ms)
			g.statusText.Text = fmt.Sprintf("CONECTADO · %dms", ms)
			g.statusText.Color = color.NRGBA{R: 52, G: 211, B: 153, A: 255}
			g.statusText.Refresh()
			g.AppendLog(fmt.Sprintf("✓ Ping google.com exitoso (%dms) - Tráfico confirmado fluyendo.", ms))
		} else {
			traffic.SetPing(0)
			g.statusText.Text = "CONECTADO · ESPERANDO TRÁFICO..."
			g.statusText.Color = color.NRGBA{R: 245, G: 158, B: 11, A: 255}
			g.statusText.Refresh()
			g.AppendLog(fmt.Sprintf("⚠ Sonda a google.com en espera: %v", res.Err))
		}
	})
}

// updateTrayState dynamically updates the system tray menu item and refreshes the OS tray.
func (g *GUI) updateTrayState(status string) {
	if g.trayConnect == nil {
		return
	}
	switch status {
	case "connected":
		g.trayConnect.Label = "Desconectar"
	case "connecting":
		g.trayConnect.Label = "Conectando..."
	default:
		g.trayConnect.Label = "Conectar"
	}

	if g.trayMenu != nil {
		g.trayMenu.Refresh()
		if desk, ok := g.app.(desktop.App); ok {
			desk.SetSystemTrayMenu(g.trayMenu)
		}
	}
}

func (g *GUI) setupSystemTray() {
	if desk, ok := g.app.(desktop.App); ok {
		// Set custom tray icon in notification area
		desk.SetSystemTrayIcon(assets.AppIconResource)

		g.trayConnect = fyne.NewMenuItem("Conectar", func() {
			g.toggleConnection()
		})
		g.trayMenu = fyne.NewMenu("UniTunnel",
			fyne.NewMenuItem("Abrir UniTunnel", func() {
				g.window.Show()
			}),
			g.trayConnect,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Salir", func() {
				if g.engine != nil && g.engine.IsRunning() {
					g.engine.Stop()
				}
				g.app.Quit()
			}),
		)
		desk.SetSystemTrayMenu(g.trayMenu)
	}
}

// ShowAndRun presents the window or initializes directly in the tray if configured.
func (g *GUI) ShowAndRun() {
	if !g.cfg.StartMinimized {
		g.window.Show()
	}
	g.app.Run()
}

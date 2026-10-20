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

	"unitunnel/pkg/auth"
	"unitunnel/pkg/config"
	"unitunnel/pkg/sysproxy"
	"unitunnel/pkg/traffic"
	"unitunnel/pkg/tun"
)

type GUI struct {
	app              fyne.App
	window           fyne.Window
	cfg              *config.Config
	engine           *tun.Engine
	logText          string
	logWidget        *widget.Entry
	statusText       *canvas.Text
	connectBtn       *widget.Button
	circleBg         *canvas.Circle
	accountSelect    *widget.Select
	accountDetails   *widget.Label
	trayConnect      *fyne.MenuItem
	pingCancel       context.CancelFunc
	chart            *RealtimeChart
	speedMonitorStop chan struct{}
}

func NewGUI(cfg *config.Config) *GUI {
	myApp := app.NewWithID("com.uninet.unitunnel")
	myApp.Settings().SetTheme(theme.DarkTheme())

	w := myApp.NewWindow("UniTunnel Connect")

	// Set custom icon if present
	if iconData, err := os.ReadFile("assets/icon.png"); err == nil {
		res := fyne.NewStaticResource("icon.png", iconData)
		w.SetIcon(res)
		myApp.SetIcon(res)
	}

	// Fixed compact landscape phone-like dimensions (500x420 with chart)
	w.Resize(fyne.NewSize(500, 420))
	w.SetFixedSize(true)

	return &GUI{
		app:    myApp,
		window: w,
		cfg:    cfg,
	}
}

func (g *GUI) AppendLog(msg string) {
	ts := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s\n", ts, msg)
	g.logText += entry
	if g.logWidget != nil {
		g.logWidget.SetText(g.logText)
		g.logWidget.CursorRow = strings.Count(g.logText, "\n")
	}
}

func (g *GUI) Setup() {
	// Configure system tray
	g.setupSystemTray()

	// Top Bar: [Settings Gear (Left)] ----- [Title] ----- [Logs Document (Right)]
	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		g.showSettingsDialog()
	})

	logsBtn := widget.NewButtonWithIcon("", theme.DocumentIcon(), func() {
		g.showLogsDialog()
	})

	titleLbl := canvas.NewText("UniTunnel Connect", color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	titleLbl.TextSize = 15
	titleLbl.TextStyle = fyne.TextStyle{Bold: true}
	titleLbl.Alignment = fyne.TextAlignCenter

	topBar := container.NewBorder(nil, nil, settingsBtn, logsBtn, container.NewCenter(titleLbl))

	// Central Circular Connect Button (OpenVPN Connect style)
	g.circleBg = canvas.NewCircle(color.NRGBA{R: 30, G: 41, B: 59, A: 255}) // Slate-800 default
	g.circleBg.Resize(fyne.NewSize(120, 120))
	g.circleBg.StrokeColor = color.NRGBA{R: 71, G: 85, B: 105, A: 255}
	g.circleBg.StrokeWidth = 3

	g.statusText = canvas.NewText("DESCONECTADO", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	g.statusText.TextSize = 12
	g.statusText.TextStyle = fyne.TextStyle{Bold: true}
	g.statusText.Alignment = fyne.TextAlignCenter

	g.connectBtn = widget.NewButtonWithIcon("CONECTAR", theme.MediaPlayIcon(), func() {
		g.toggleConnection()
	})
	g.connectBtn.Importance = widget.HighImportance

	roundContainer := container.NewVBox(
		container.NewCenter(g.statusText),
		container.NewCenter(g.connectBtn),
	)

	// Account Selector Section (OpenVPN Connect profile switcher)
	accSection := g.setupAccountSection()

	// Bottom info label
	infoLabel := canvas.NewText("Modo Wintun · Cero Consolas Visibles", color.NRGBA{R: 100, G: 116, B: 139, A: 255})
	infoLabel.TextSize = 10
	infoLabel.Alignment = fyne.TextAlignCenter

	// Realtime Bandwidth & Latency Line Chart
	g.chart = NewRealtimeChart(460, 45)

	// Assemble layout
	mainLayout := container.NewVBox(
		topBar,
		widget.NewSeparator(),
		container.NewPadded(roundContainer),
		g.chart.Container,
		widget.NewSeparator(),
		accSection,
		container.NewCenter(infoLabel),
	)

	g.window.SetContent(mainLayout)

	// Intercept window close to stay in System Tray (OpenVPN Connect behavior)
	g.window.SetCloseIntercept(func() {
		g.window.Hide()
	})

	g.AppendLog("UniTunnel inicializado con privilegios de Administrador.")
	g.AppendLog(fmt.Sprintf("Proxy configurado: %s:%d", g.cfg.ProxyHost, g.cfg.ProxyPort))
}

func (g *GUI) setupSystemTray() {
	if desk, ok := g.app.(desktop.App); ok {
		g.trayConnect = fyne.NewMenuItem("Conectar", func() {
			g.toggleConnection()
		})
		trayMenu := fyne.NewMenu("UniTunnel",
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
		desk.SetSystemTrayMenu(trayMenu)
	}
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

	// Add Account (+) Button
	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		g.showAddAccountDialog()
	})

	// Delete Account (-) Button
	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		g.showDeleteAccountDialog()
	})

	accRow := container.NewBorder(
		nil,
		nil,
		widget.NewLabelWithStyle("Usuario:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(addBtn, delBtn),
		g.accountSelect,
	)

	return container.NewVBox(
		accRow,
		container.NewCenter(g.accountDetails),
	)
}

func (g *GUI) updateAccountDetailsText(acc *config.UserAccount) {
	if acc == nil || acc.Username == "" {
		g.accountDetails.SetText("(Sin credenciales configuradas - abre Ajustes)")
	} else {
		dom := ""
		if acc.Domain != "" {
			dom = acc.Domain + "\\"
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
	nameEntry.SetPlaceHolder("Ej: Cuenta Personal / yoanbramon")

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

	d.Resize(fyne.NewSize(420, 260))
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

	msg := fmt.Sprintf("¿Estás seguro de que deseas eliminar el perfil '%s' (%s)?", active.Name, active.Username)
	dialog.ShowConfirm("Eliminar Cuenta", msg, func(confirmed bool) {
		if confirmed {
			g.cfg.DeleteAccount(active.ID)
			_ = config.SaveConfig(g.cfg)
			g.refreshAccountList()
			g.AppendLog(fmt.Sprintf("✓ Cuenta '%s' eliminada.", active.Name))
		}
	}, g.window)
}

func (g *GUI) showSettingsDialog() {
	proxyHost := widget.NewEntry()
	proxyHost.SetText(g.cfg.ProxyHost)

	proxyPort := widget.NewEntry()
	proxyPort.SetText(strconv.Itoa(g.cfg.ProxyPort))

	authTypeSelect := widget.NewSelect([]string{"auto", "ntlm", "basic", "none"}, nil)
	authTypeSelect.SetSelected(g.cfg.AuthType)

	captureModeSelect := widget.NewSelect([]string{"tun", "system_proxy"}, nil)
	captureModeSelect.SetSelected(g.cfg.CaptureMode)

	// Probe button inside settings
	probeBtn := widget.NewButtonWithIcon("Comprobar Proxy (10.0.0.1)", theme.SearchIcon(), func() {
		addr := fmt.Sprintf("%s:%s", proxyHost.Text, proxyPort.Text)
		g.AppendLog(fmt.Sprintf("Sondeando proxy %s...", addr))
		go func() {
			p, _ := strconv.Atoi(proxyPort.Text)
			res, err := auth.ProbeProxy(fmt.Sprintf("%s:%d", proxyHost.Text, p), 5*time.Second)
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			msg := fmt.Sprintf("Esquema detectado: %s\nServidor: %s\nCabeceras: %s",
				res.PrimaryType, res.ProxyServer, strings.Join(res.RawHeaders, "\n"))
			dialog.ShowInformation("Resultado de Sonda", msg, g.window)
			if res.PrimaryType == auth.AuthTypeNTLM {
				authTypeSelect.SetSelected("ntlm")
			} else if res.PrimaryType == auth.AuthTypeBasic {
				authTypeSelect.SetSelected("basic")
			}
		}()
	})

	vpsCheck := widget.NewCheck("Habilitar VPS de salida", func(b bool) {
		g.cfg.VPSEnabled = b
	})
	vpsCheck.SetChecked(g.cfg.VPSEnabled)

	vpsHost := widget.NewEntry()
	vpsHost.SetText(g.cfg.VPSServer)

	form := container.NewVBox(
		widget.NewCard("Proxy Universitario", "", container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("Servidor Proxy:", proxyHost),
				widget.NewFormItem("Puerto:", proxyPort),
				widget.NewFormItem("Autenticación:", authTypeSelect),
				widget.NewFormItem("Modo de Captura:", captureModeSelect),
			),
			container.NewHBox(probeBtn),
		)),
		widget.NewCard("Salida VPS Opcional", "", container.NewVBox(
			vpsCheck,
			widget.NewForm(widget.NewFormItem("Servidor VPS:", vpsHost)),
		)),
	)

	d := dialog.NewCustomConfirm("Ajustes de Conexión", "Guardar", "Cerrar", container.NewVScroll(form), func(confirmed bool) {
		if confirmed {
			g.cfg.ProxyHost = proxyHost.Text
			if p, err := strconv.Atoi(proxyPort.Text); err == nil {
				g.cfg.ProxyPort = p
			}
			g.cfg.AuthType = authTypeSelect.Selected
			g.cfg.CaptureMode = captureModeSelect.Selected
			g.cfg.VPSServer = vpsHost.Text
			_ = config.SaveConfig(g.cfg)
			g.AppendLog("✓ Ajustes guardados correctamente en config.json.")
		}
	}, g.window)

	d.Resize(fyne.NewSize(460, 320))
	d.Show()
}

func (g *GUI) showLogsDialog() {
	if g.logWidget == nil {
		g.logWidget = widget.NewMultiLineEntry()
		g.logWidget.TextStyle = fyne.TextStyle{Monospace: true}
		g.logWidget.SetText(g.logText)
	}

	clearBtn := widget.NewButtonWithIcon("Limpiar", theme.ContentClearIcon(), func() {
		g.logText = ""
		g.logWidget.SetText("")
	})

	copyBtn := widget.NewButtonWithIcon("Copiar", theme.ContentCopyIcon(), func() {
		g.window.Clipboard().SetContent(g.logText)
		g.AppendLog("Logs copiados al portapapeles.")
	})

	content := container.NewBorder(
		nil,
		container.NewHBox(clearBtn, copyBtn),
		nil,
		nil,
		g.logWidget,
	)

	d := dialog.NewCustom("Registro de Conexión (Logs)", "Cerrar", content, g.window)
	d.Resize(fyne.NewSize(460, 300))
	d.Show()
}

func (g *GUI) toggleConnection() {
	if g.engine != nil && g.engine.IsRunning() {
		// Stop
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

		if g.trayConnect != nil {
			g.trayConnect.Label = "Conectar"
		}
		return
	}

	// Verify credentials
	acc := g.cfg.GetActiveAccount()
	if acc == nil || acc.Username == "" {
		dialog.ShowInformation("Faltan Credenciales", "Por favor ingresa usuario y contraseña en el selector de cuenta o ajustes.", g.window)
		return
	}

	g.statusText.Text = "CONECTANDO..."
	g.statusText.Color = color.NRGBA{R: 245, G: 158, B: 11, A: 255} // Amber
	g.statusText.Refresh()
	g.AppendLog(fmt.Sprintf("Iniciando túnel hacia %s:%d con usuario '%s'...", g.cfg.ProxyHost, g.cfg.ProxyPort, acc.Username))

	g.engine = tun.NewEngine(g.cfg, func(msg string) {
		g.AppendLog(msg)
	})

	if err := g.engine.Start(); err != nil {
		g.statusText.Text = "ERROR"
		g.statusText.Color = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
		g.statusText.Refresh()
		g.AppendLog(fmt.Sprintf("Fallo al conectar: %v", err))
		dialog.ShowError(err, g.window)
		return
	}

	if g.cfg.CaptureMode == "system_proxy" {
		_ = sysproxy.EnableSystemProxy(g.cfg.LocalSocksPort)
		g.AppendLog("✓ Proxy del sistema Windows configurado en 127.0.0.1:1080.")
	}

	// Connected state
	g.statusText.Text = "CONECTADO · VERIFICANDO TRÁFICO..."
	g.statusText.Color = color.NRGBA{R: 52, G: 211, B: 153, A: 255} // Emerald Green
	g.statusText.Refresh()

	g.connectBtn.SetText("DESCONECTAR")
	g.connectBtn.SetIcon(theme.MediaStopIcon())
	g.connectBtn.Importance = widget.DangerImportance

	if g.trayConnect != nil {
		g.trayConnect.Label = "Desconectar"
	}

	// Start real-time speed monitor feeding the line chart
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

func (g *GUI) ShowAndRun() {
	if g.cfg.StartMinimized {
		// Show window initially or start minimized to tray
		g.window.Show()
	} else {
		g.window.Show()
	}
	g.app.Run()
}

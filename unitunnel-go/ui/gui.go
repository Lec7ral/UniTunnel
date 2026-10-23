package ui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"sync"
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
	"unitunnel/pkg/tunnel"
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
	proxyTargetLabel *widget.Label
	trayConnect      *fyne.MenuItem
	trayMenu         *fyne.Menu
	pingCancel       context.CancelFunc
	chart            *RealtimeChart
	speedMonitorStop chan struct{}
	mu               sync.Mutex

	// Settings fields
	proxyHostEntry    *widget.Entry
	proxyPortEntry    *widget.Entry
	authTypeSelect    *widget.Select
	captureModeSelect *widget.Select
	vpsCheck          *widget.Check
	vpsHostEntry      *widget.Entry
}

// NewGUI initializes the Fyne application and creates the main window.
func NewGUI(cfg *config.Config) *GUI {
	myApp := app.NewWithID("com.uninet.unitunnel")
	myApp.Settings().SetTheme(theme.DarkTheme())

	w := myApp.NewWindow("UniTunnel Connect")

	// Set custom icon embedded directly into the Go executable binary
	w.SetIcon(assets.AppIconResource)
	myApp.SetIcon(assets.AppIconResource)

	// Dimensions with ample horizontal and vertical room for tabs and controls
	w.Resize(fyne.NewSize(420, 580))
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
	g.mu.Lock()
	g.logText += entry
	currentText := g.logText
	g.mu.Unlock()

	if g.logWidget != nil {
		g.logWidget.SetText(currentText)
		g.logWidget.CursorRow = strings.Count(currentText, "\n")
	}
}

// Setup builds the tabbed user interface (Conexión, Ajustes, Registro).
func (g *GUI) Setup() {
	g.setupSystemTray()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Conexión", theme.HomeIcon(), g.buildConnectionTab()),
		container.NewTabItemWithIcon("Ajustes", theme.SettingsIcon(), g.buildSettingsTab()),
		container.NewTabItemWithIcon("Registro", theme.DocumentIcon(), g.buildLogsTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	g.window.SetContent(tabs)

	// Intercept window close to stay resident in System Tray
	g.window.SetCloseIntercept(func() {
		g.window.Hide()
	})

	g.AppendLog("[SISTEMA] UniTunnel iniciado con privilegios de Administrador.")
	g.AppendLog(fmt.Sprintf("[CONFIG] Proxy configurado: %s:%d (%s)", g.cfg.ProxyHost, g.cfg.ProxyPort, g.cfg.AuthType))

	// Run startup diagnostic and auto-clean of any stale routes or processes
	go tun.PerformStartupDiagnosticsAndClean(g.cfg, func(msg string) {
		g.AppendLog(msg)
	})
}

func (g *GUI) getProxySummaryText() string {
	mode := "Wintun (L3)"
	if g.cfg.CaptureMode == "system_proxy" {
		mode = "Proxy Windows"
	}
	return fmt.Sprintf("Proxy: %s:%d  ·  %s", g.cfg.ProxyHost, g.cfg.ProxyPort, mode)
}

// buildConnectionTab builds the main connection view.
func (g *GUI) buildConnectionTab() fyne.CanvasObject {
	appTitle := canvas.NewText("UniTunnel", color.NRGBA{R: 248, G: 250, B: 252, A: 255})
	appTitle.TextSize = 18
	appTitle.TextStyle = fyne.TextStyle{Bold: true}
	appTitle.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Túnel de Red · Autenticación NTLMv2 / AD", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	subtitle.TextSize = 11
	subtitle.Alignment = fyne.TextAlignCenter

	g.statusText = canvas.NewText("DESCONECTADO", color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	g.statusText.TextSize = 14
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
		container.NewCenter(appTitle),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
		container.NewCenter(g.statusText),
		container.NewPadded(g.connectBtn),
		container.NewCenter(g.proxyTargetLabel),
	)

	accountCard := g.setupAccountSection()

	middleContent := container.NewVBox(
		connectCard,
		widget.NewSeparator(),
		accountCard,
	)

	g.chart = NewRealtimeChart(360, 50)

	footerLabel := canvas.NewText("Wintun L3 · Protección Anti-Bloqueo AD · Cero Consolas", color.NRGBA{R: 100, G: 116, B: 139, A: 255})
	footerLabel.TextSize = 10
	footerLabel.Alignment = fyne.TextAlignCenter

	bottomSection := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(g.chart.Container),
		container.NewCenter(footerLabel),
	)

	return container.NewBorder(
		nil,
		bottomSection,
		nil,
		nil,
		container.NewVScroll(container.NewPadded(middleContent)),
	)
}

// buildSettingsTab builds the network settings view.
func (g *GUI) buildSettingsTab() fyne.CanvasObject {
	g.proxyHostEntry = widget.NewEntry()
	g.proxyHostEntry.SetText(g.cfg.ProxyHost)
	g.proxyHostEntry.SetPlaceHolder("Ej: 10.0.0.1 o proxy.institucion.cu")

	g.proxyPortEntry = widget.NewEntry()
	g.proxyPortEntry.SetText(strconv.Itoa(g.cfg.ProxyPort))
	g.proxyPortEntry.SetPlaceHolder("8080")

	g.authTypeSelect = widget.NewSelect([]string{"auto", "ntlm", "basic", "none"}, nil)
	g.authTypeSelect.SetSelected(g.cfg.AuthType)

	g.captureModeSelect = widget.NewSelect([]string{"tun", "system_proxy"}, nil)
	g.captureModeSelect.SetSelected(g.cfg.CaptureMode)

	g.vpsCheck = widget.NewCheck("Habilitar VPS de salida (opcional)", func(b bool) {
		g.cfg.VPSEnabled = b
	})
	g.vpsCheck.SetChecked(g.cfg.VPSEnabled)

	g.vpsHostEntry = widget.NewEntry()
	g.vpsHostEntry.SetText(g.cfg.VPSServer)
	g.vpsHostEntry.SetPlaceHolder("vps.midominio.com:443")

	probeBtn := widget.NewButtonWithIcon("Sondear Proxy (Detectar Esquema y Auditar)", theme.SearchIcon(), func() {
		host := strings.TrimSpace(g.proxyHostEntry.Text)
		portStr := strings.TrimSpace(g.proxyPortEntry.Text)
		addr := fmt.Sprintf("%s:%s", host, portStr)
		g.AppendLog(fmt.Sprintf("[SONDA] Conectando con %s para detectar esquema y verificar credenciales...", addr))

		go func() {
			p, _ := strconv.Atoi(portStr)
			acc := g.cfg.GetActiveAccount()
			u, dom, pass := "", "", ""
			if acc != nil {
				u = acc.Username
				dom = acc.Domain
				pass = acc.Password
			}

			res, err := auth.ProbeProxyWithAuth(fmt.Sprintf("%s:%d", host, p), u, dom, pass, 6*time.Second)
			if err != nil {
				g.AppendLog(fmt.Sprintf("[SONDA ERROR] Falló sonda a %s: %v", addr, err))
				dialog.ShowError(err, g.window)
				return
			}

			// Clean, formatted custom dialog
			titleLbl := widget.NewLabelWithStyle("Diagnóstico Completo del Proxy y Dominio", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			serverLbl := widget.NewLabel(fmt.Sprintf("• Servidor: %s (Latencia: %dms)", res.ProxyServer, res.Latency.Milliseconds()))
			if res.ProxyVia != "" {
				serverLbl.SetText(fmt.Sprintf("• Servidor: %s [Via: %s] (Latencia: %dms)", res.ProxyServer, res.ProxyVia, res.Latency.Milliseconds()))
			}

			authLbl := widget.NewLabelWithStyle(fmt.Sprintf("• Esquema Proxy: %s", strings.ToUpper(string(res.PrimaryType))), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

			var bodyItems []fyne.CanvasObject
			bodyItems = append(bodyItems, titleLbl, widget.NewSeparator(), serverLbl, authLbl)

			// Active Directory / Domain Details
			if res.TargetNetBIOS != "" || res.TargetDNSDomain != "" {
				adTitle := widget.NewLabelWithStyle("Datos de Active Directory / Dominio:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				bodyItems = append(bodyItems, widget.NewSeparator(), adTitle)
				if res.TargetNetBIOS != "" {
					bodyItems = append(bodyItems, widget.NewLabel(fmt.Sprintf("  • Dominio NetBIOS: %s", res.TargetNetBIOS)))
				}
				if res.TargetDNSDomain != "" {
					bodyItems = append(bodyItems, widget.NewLabel(fmt.Sprintf("  • Dominio DNS (FQDN): %s", res.TargetDNSDomain)))
				}
				if res.TargetServerName != "" {
					bodyItems = append(bodyItems, widget.NewLabel(fmt.Sprintf("  • Servidor Dominio: %s", res.TargetServerName)))
				}
			}

			// Credentials audit
			if res.AuthTested {
				bodyItems = append(bodyItems, widget.NewSeparator())
				authHeader := widget.NewLabelWithStyle("Auditoría de Credenciales:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				statusLbl := widget.NewLabel(res.AuthStatusText)
				statusLbl.Wrapping = fyne.TextWrapWord
				bodyItems = append(bodyItems, authHeader, statusLbl)
			}

			diagLbl := widget.NewLabel(fmt.Sprintf("• Diagnóstico: %s", res.Diagnostic))
			diagLbl.Wrapping = fyne.TextWrapWord
			bodyItems = append(bodyItems, widget.NewSeparator(), diagLbl)

			if len(res.RelevantHeaders) > 0 {
				hdrTitle := widget.NewLabelWithStyle("Cabeceras del Servidor:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
				bodyItems = append(bodyItems, widget.NewSeparator(), hdrTitle)
				for _, h := range res.RelevantHeaders {
					l := widget.NewLabel("  " + h)
					l.TextStyle = fyne.TextStyle{Monospace: true}
					l.Wrapping = fyne.TextWrapWord
					bodyItems = append(bodyItems, l)
				}
			}

			scrollContent := container.NewVScroll(container.NewVBox(bodyItems...))
			scrollContent.SetMinSize(fyne.NewSize(380, 280))

			d := dialog.NewCustom("Sonda de Proxy y Dominio", "Aceptar", scrollContent, g.window)
			d.Resize(fyne.NewSize(420, 360))
			d.Show()

			if res.PrimaryType == auth.AuthTypeNTLM {
				g.authTypeSelect.SetSelected("ntlm")
				g.AppendLog(fmt.Sprintf("[SONDA] Autenticación NTLM requerida por %s (seleccionada automáticamente).", res.ProxyServer))
			} else if res.PrimaryType == auth.AuthTypeBasic {
				g.authTypeSelect.SetSelected("basic")
				g.AppendLog(fmt.Sprintf("[SONDA] Autenticación Basic detectada en %s.", res.ProxyServer))
			}
			if res.AuthTested {
				g.AppendLog(fmt.Sprintf("[SONDA AUDITORÍA] %s", res.AuthStatusText))
			}
		}()
	})

	diagBtn := widget.NewButtonWithIcon("Diagnosticar Red y Wintun", theme.InfoIcon(), func() {
		rep := tun.InspectNetworkState(g.cfg)

		titleLbl := widget.NewLabelWithStyle("Diagnóstico de Red y Adaptador Wintun", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

		// Wintun driver & binary
		wintunStatus := "✓ Presente en la carpeta"
		if !rep.WintunDLLFound {
			wintunStatus = "✗ No encontrado (falta wintun.dll)"
		}
		tun2socksStatus := "✓ Presente en la carpeta"
		if !rep.Tun2socksFound {
			tun2socksStatus = "✗ No encontrado (falta tun2socks.exe)"
		}
		driverBox := widget.NewLabel(fmt.Sprintf("• Driver Wintun (wintun.dll): %s\n• Motor L3 (tun2socks.exe): %s", wintunStatus, tun2socksStatus))

		// Virtual adapter
		adapterDesc := "• Adaptador Virtual: No inicializado aún (inicia al pulsar CONECTAR)"
		if rep.AdapterPresent {
			adapterDesc = fmt.Sprintf("• Adaptador Virtual '%s': Activo en Windows\n  - IP Asignada: %s\n  - Puerta de Enlace Virtual: %s",
				rep.AdapterName, rep.AdapterIPv4, rep.AdapterGateway)
		}
		adapterLbl := widget.NewLabel(adapterDesc)

		// Routing & Gateway
		gwStatus := fmt.Sprintf("• Puerta de enlace física de Windows: %s", rep.PhysicalGateway)
		routeDesc := "• Rutas del Túnel (0.0.0.0/1 y 128.0.0.0/1): Inactivas (estado de reposo)"
		if rep.TunnelRoutesActive {
			routeDesc = "• Rutas del Túnel: ✓ ACTIVAS (Todo el tráfico IP dirigido al túnel)"
		} else if rep.StaleRoutesDetected {
			routeDesc = "• Rutas del Túnel: ⚠ Rutas huérfanas detectadas en Windows"
		}
		routingLbl := widget.NewLabel(fmt.Sprintf("%s\n%s", gwStatus, routeDesc))

		// Proxy state
		proxyDesc := "• Proxy del Sistema (WinINet): Inactivo"
		if rep.SystemProxyEnabled {
			proxyDesc = fmt.Sprintf("• Proxy del Sistema (WinINet): ✓ Activo (%s)", rep.SystemProxyServer)
		}
		if rep.ConsoleEnvActive {
			proxyDesc += "\n• Variables de Consola (HTTP_PROXY): ✓ Configuradas automáticamente"
		}
		proxyLbl := widget.NewLabel(proxyDesc)

		overallLbl := widget.NewLabelWithStyle(rep.OverallStatus, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		var diagItems []fyne.CanvasObject
		diagItems = append(diagItems, titleLbl, widget.NewSeparator(),
			driverBox, widget.NewSeparator(),
			adapterLbl, widget.NewSeparator(),
			routingLbl, widget.NewSeparator(),
			proxyLbl, widget.NewSeparator(),
			overallLbl,
		)

		var d dialog.Dialog
		repairBtn := widget.NewButtonWithIcon("Limpiar / Reparar Conflictos de Red", theme.DeleteIcon(), func() {
			_ = tun.RepairNetworkState(g.cfg)
			g.AppendLog("[REPARACIÓN] ✓ Rutas huérfanas, procesos tun2socks y proxies del sistema limpiados con éxito.")
			if d != nil {
				d.Hide()
			}
			dialog.ShowInformation("Red Reparada", "Configuración de red de Windows limpiada y restaurada a estado original.", g.window)
		})
		repairBtn.Importance = widget.WarningImportance

		diagItems = append(diagItems, widget.NewSeparator(), container.NewCenter(repairBtn))

		scroll := container.NewVScroll(container.NewVBox(diagItems...))
		scroll.SetMinSize(fyne.NewSize(420, 360))

		d = dialog.NewCustom("Diagnóstico de Wintun y Red", "Cerrar", scroll, g.window)
		d.Resize(fyne.NewSize(450, 420))
		d.Show()
	})

	auditBtn := widget.NewButtonWithIcon("Auditar Bloqueos 403 (Squid ACL)", theme.WarningIcon(), func() {
		g.AppendLog("[AUDITORÍA] Verificando reglas ACL y puertos bloqueados en el proxy...")
		go func() {
			dialer := tunnel.NewDialer(g.cfg)
			rep := tunnel.RunProxyAudit(g.cfg, dialer)

			var items []fyne.CanvasObject
			titleLbl := widget.NewLabelWithStyle("Auditoría de Bloqueos y Políticas de Squid", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			items = append(items, titleLbl, widget.NewSeparator())

			for _, it := range rep.Items {
				statusPrefix := "✓"
				if it.HTTPCode == 403 {
					statusPrefix = "✗"
				}
				nameLbl := widget.NewLabelWithStyle(fmt.Sprintf("%s %s: %s", statusPrefix, it.Name, it.Status), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
				explLbl := widget.NewLabel(fmt.Sprintf("   %s", it.Explanation))
				explLbl.Wrapping = fyne.TextWrapWord
				items = append(items, nameLbl, explLbl)
			}

			items = append(items, widget.NewSeparator())
			sumTitle := widget.NewLabelWithStyle("Conclusión del Diagnóstico:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			sumLbl := widget.NewLabel(rep.Summary)
			sumLbl.Wrapping = fyne.TextWrapWord

			recTitle := widget.NewLabelWithStyle("Recomendaciones y Alternativas:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			recLbl := widget.NewLabel(rep.Recommendation)
			recLbl.Wrapping = fyne.TextWrapWord

			items = append(items, sumTitle, sumLbl, widget.NewSeparator(), recTitle, recLbl)

			scroll := container.NewVScroll(container.NewVBox(items...))
			scroll.SetMinSize(fyne.NewSize(420, 340))

			d := dialog.NewCustom("Auditoría de Políticas Squid (HTTP 403)", "Entendido", scroll, g.window)
			d.Resize(fyne.NewSize(450, 400))
			d.Show()
		}()
	})

	saveBtn := widget.NewButtonWithIcon("Guardar Ajustes", theme.DocumentSaveIcon(), func() {
		g.cfg.ProxyHost = strings.TrimSpace(g.proxyHostEntry.Text)
		if p, err := strconv.Atoi(strings.TrimSpace(g.proxyPortEntry.Text)); err == nil {
			g.cfg.ProxyPort = p
		}
		g.cfg.AuthType = g.authTypeSelect.Selected
		g.cfg.CaptureMode = g.captureModeSelect.Selected
		g.cfg.VPSServer = strings.TrimSpace(g.vpsHostEntry.Text)

		if err := config.SaveConfig(g.cfg); err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		g.proxyTargetLabel.SetText(g.getProxySummaryText())
		g.AppendLog("[CONFIG] ✓ Ajustes guardados correctamente en config.json.")
		dialog.ShowInformation("Éxito", "Configuración guardada correctamente.", g.window)
	})
	saveBtn.Importance = widget.HighImportance

	form := widget.NewForm(
		widget.NewFormItem("Servidor Proxy:", g.proxyHostEntry),
		widget.NewFormItem("Puerto:", g.proxyPortEntry),
		widget.NewFormItem("Autenticación:", g.authTypeSelect),
		widget.NewFormItem("Modo de Captura:", g.captureModeSelect),
	)

	settingsBox := container.NewVBox(
		widget.NewLabelWithStyle("CONFIGURACIÓN DE RED INSTITUCIONAL", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		form,
		container.NewVBox(
			container.NewGridWithColumns(2, probeBtn, diagBtn),
			auditBtn,
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("SALIDA EXTERNA (OPCIONAL)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.vpsCheck,
		widget.NewForm(widget.NewFormItem("Servidor VPS:", g.vpsHostEntry)),
		widget.NewSeparator(),
		container.NewPadded(saveBtn),
	)

	return container.NewVScroll(container.NewPadded(settingsBox))
}

// buildLogsTab builds the live console log view.
func (g *GUI) buildLogsTab() fyne.CanvasObject {
	g.logWidget = widget.NewMultiLineEntry()
	g.logWidget.TextStyle = fyne.TextStyle{Monospace: true}
	g.logWidget.Wrapping = fyne.TextWrapWord
	g.mu.Lock()
	g.logWidget.SetText(g.logText)
	g.mu.Unlock()

	copyBtn := widget.NewButtonWithIcon("Copiar", theme.ContentCopyIcon(), func() {
		g.mu.Lock()
		txt := g.logText
		g.mu.Unlock()
		g.window.Clipboard().SetContent(txt)
		g.AppendLog("[LOGS] Registro copiado al portapapeles.")
	})

	clearBtn := widget.NewButtonWithIcon("Limpiar", theme.ContentClearIcon(), func() {
		g.mu.Lock()
		g.logText = ""
		g.mu.Unlock()
		g.logWidget.SetText("")
	})

	actions := container.NewHBox(copyBtn, clearBtn)
	header := container.NewBorder(
		nil,
		nil,
		widget.NewLabelWithStyle("REGISTRO DE EVENTOS Y DIAGNÓSTICO", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		actions,
	)

	return container.NewBorder(
		container.NewPadded(header),
		nil,
		nil,
		nil,
		container.NewPadded(g.logWidget),
	)
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
				cleanDom, cleanU := auth.SplitDomainUser(acc.Domain, acc.Username)
				domStr := ""
				if cleanDom != "" {
					domStr = fmt.Sprintf(" (Dominio: %s)", cleanDom)
				}
				g.AppendLog(fmt.Sprintf("[CUENTA] Perfil activo cambiado a: %s (Usuario: %s%s)", acc.Name, cleanU, domStr))
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
		cleanDom, cleanU := auth.SplitDomainUser(acc.Domain, acc.Username)
		domPrefix := ""
		if cleanDom != "" {
			domPrefix = cleanDom + `\`
		}
		g.accountDetails.SetText(fmt.Sprintf("%s%s · Contraseña guardada", domPrefix, cleanU))
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
	userEntry.SetPlaceHolder("Usuario (o DOMINIO\\usuario)")

	domainEntry := widget.NewEntry()
	domainEntry.SetPlaceHolder("Opcional si usas DOMINIO\\usuario")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Contraseña de red institucional")

	form := widget.NewForm(
		widget.NewFormItem("Nombre Perfil:", nameEntry),
		widget.NewFormItem("Usuario:", userEntry),
		widget.NewFormItem("Dominio (NTLM):", domainEntry),
		widget.NewFormItem("Contraseña:", passEntry),
	)

	d := dialog.NewCustomConfirm("Añadir Cuenta Institucional", "Guardar", "Cancelar", form, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(userEntry.Text) == "" {
			return
		}
		acc := g.cfg.AddAccount(
			strings.TrimSpace(nameEntry.Text),
			strings.TrimSpace(userEntry.Text),
			strings.TrimSpace(domainEntry.Text),
			passEntry.Text,
		)
		_ = config.SaveConfig(g.cfg)
		g.refreshAccountList()
		g.AppendLog(fmt.Sprintf("[CUENTA] ✓ Cuenta '%s' agregada y configurada.", acc.Name))
	}, g.window)

	d.Resize(fyne.NewSize(350, 270))
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
			g.AppendLog(fmt.Sprintf("[CUENTA] ✓ Perfil '%s' eliminado.", active.Name))
		}
	}, g.window)
}

func (g *GUI) toggleConnection() {
	if g.engine != nil && g.engine.IsRunning() {
		// Stop connection
		g.engine.Stop()
		_ = sysproxy.DisableSystemProxy()
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
		g.AppendLog("[TÚNEL] Conexión cerrada y rutas restauradas.")
		return
	}

	// Verify active account has credentials
	acc := g.cfg.GetActiveAccount()
	if acc == nil || strings.TrimSpace(acc.Username) == "" {
		dialog.ShowInformation("Credenciales Requeridas",
			"Por favor ingresa tu usuario y contraseña en el selector de cuentas antes de conectar.", g.window)
		return
	}

	cleanDom, cleanU := auth.SplitDomainUser(acc.Domain, acc.Username)
	domStr := ""
	if cleanDom != "" {
		domStr = fmt.Sprintf(" (Dominio: %s)", cleanDom)
	}

	g.statusText.Text = "VERIFICANDO CREDENCIALES..."
	g.statusText.Color = color.NRGBA{R: 245, G: 158, B: 11, A: 255} // Amber
	g.statusText.Refresh()

	g.connectBtn.SetText("CONECTANDO...")
	g.connectBtn.Importance = widget.WarningImportance
	g.updateTrayState("connecting")

	g.AppendLog(fmt.Sprintf("[CONEXIÓN] Iniciando verificación hacia %s:%d con usuario '%s'%s...",
		g.cfg.ProxyHost, g.cfg.ProxyPort, cleanU, domStr))

	// Run pre-flight check and startup in background to prevent UI lockup
	go func() {
		engine := tun.NewEngine(g.cfg, func(msg string) {
			g.AppendLog(msg)
		}, func(authErr error) {
			// Asynchronous auth rejection callback: trip anti-lockout immediately
			g.AppendLog(fmt.Sprintf("✗ [AUTH-RECHAZO] %v", authErr))
			g.AppendLog("🛡 [PROTECCIÓN ACTIVA] UniTunnel detuvo el túnel para proteger tu cuenta de dominio.")
			if g.engine != nil && g.engine.IsRunning() {
				g.engine.Stop()
			}
			_ = sysproxy.DisableSystemProxy()
			g.statusText.Text = "ERROR: SESIÓN CERRADA POR EL PROXY"
			g.statusText.Color = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
			g.statusText.Refresh()
			g.connectBtn.SetText("CONECTAR")
			g.connectBtn.SetIcon(theme.MediaPlayIcon())
			g.connectBtn.Importance = widget.HighImportance
			g.updateTrayState("disconnected")
		})

		g.engine = engine

		err := g.engine.Start()
		if err != nil {
			g.statusText.Text = "ERROR: AUTENTICACIÓN RECHAZADA"
			g.statusText.Color = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
			g.statusText.Refresh()

			g.connectBtn.SetText("CONECTAR")
			g.connectBtn.SetIcon(theme.MediaPlayIcon())
			g.connectBtn.Importance = widget.HighImportance
			g.updateTrayState("disconnected")

			g.AppendLog(fmt.Sprintf("✗ [AUTH-FALLO] %v", err))
			g.AppendLog("ℹ [AYUDA] Verifica tu usuario, contraseña y formato de dominio (ej: UCI o uci.cu).")

			errMsg := fmt.Sprintf("El proxy universitario rechazó las credenciales:\n\n%v\n\n🛡 PROTECCIÓN ANTI-BLOQUEO:\nUniTunnel canceló inmediatamente los reintentos automáticos para evitar que Active Directory bloquee tu cuenta institucional.\n\nPor favor verifica tu usuario, contraseña o especifica el dominio si tu red lo requiere.", err)
			dialog.ShowError(fmt.Errorf("%s", errMsg), g.window)
			return
		}

		// Connected successfully! Enable Windows System Proxy for all web browsers & apps
		httpP := g.cfg.LocalHTTPPort
		if httpP == 0 {
			httpP = 8081
		}
		socksP := g.cfg.LocalSocksPort
		if socksP == 0 {
			socksP = 1080
		}
		if err := sysproxy.EnableSystemProxy(httpP, socksP); err == nil {
			g.AppendLog(fmt.Sprintf("[SISTEMA] ✓ Proxy del sistema Windows activado (HTTP: 127.0.0.1:%d · SOCKS5: 127.0.0.1:%d).", httpP, socksP))
			g.AppendLog("[SISTEMA] ✓ Todo el tráfico de navegadores (Chrome, Edge, Firefox) redirigido automáticamente al túnel.")
		} else {
			g.AppendLog(fmt.Sprintf("[SISTEMA] ℹ Configura tu navegador o Telegram hacia 127.0.0.1:%d (HTTP) o 127.0.0.1:%d (SOCKS5).", httpP, socksP))
		}

		g.statusText.Text = "CONECTADO · VERIFICANDO TRÁFICO..."
		g.statusText.Color = color.NRGBA{R: 52, G: 211, B: 153, A: 255} // Emerald Green
		g.statusText.Refresh()

		g.connectBtn.SetText("DESCONECTAR")
		g.connectBtn.SetIcon(theme.MediaStopIcon())
		g.connectBtn.Importance = widget.DangerImportance

		g.updateTrayState("connected")
		g.AppendLog("✓ [CONECTADO] Túnel activo hacia el proxy universitario.")

		// Start real-time speed monitor feeding the line chart at the bottom
		g.speedMonitorStop = traffic.StartSpeedMonitor(1*time.Second, func(speedMBs float64, pingMS int64) {
			if g.chart != nil {
				g.chart.AddData(speedMBs, pingMS)
			}
		})

		// Start periodic ping health-check to verify real traffic flow through Wintun
		ctx, cancel := context.WithCancel(context.Background())
		g.pingCancel = cancel

		go tun.StartPeriodicPing(ctx, 6*time.Second, func(res tun.ProbeResult) {
			if res.Success {
				ms := res.RTT.Milliseconds()
				traffic.SetPing(ms)
				g.statusText.Text = fmt.Sprintf("CONECTADO · %dms", ms)
				g.statusText.Color = color.NRGBA{R: 52, G: 211, B: 153, A: 255}
				g.statusText.Refresh()
			} else {
				traffic.SetPing(0)
				g.statusText.Text = "CONECTADO · ESPERANDO TRÁFICO..."
				g.statusText.Color = color.NRGBA{R: 245, G: 158, B: 11, A: 255}
				g.statusText.Refresh()
			}
		})
	}()
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

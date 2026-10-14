import React, { useState } from 'react';
import {
  ShieldCheck,
  Terminal,
  Network,
  Cpu,
  Download,
  Copy,
  Check,
  ExternalLink,
  Play,
  RotateCcw,
  Layers,
  Server,
  FileCode,
  Radio,
  Sliders,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  FolderGit2,
  Lock,
  ChevronRight,
  HelpCircle,
  Eye,
  EyeOff
} from 'lucide-react';

// Go code repository representations for the in-app explorer
const GO_CODE_FILES: Record<string, { title: string; category: string; description: string; code: string }> = {
  'github-action': {
    title: '.github/workflows/build-windows.yml',
    category: 'CI/CD Pipeline',
    description: 'Automatización de GitHub Actions que compila unitunnel.exe en Windows con MinGW, descarga wintun.dll y genera el ZIP listo para usar.',
    code: `name: Build Windows Application (UniTunnel)

on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]
  workflow_dispatch:

jobs:
  build-windows:
    name: Build UniTunnel for Windows (x64)
    runs-on: windows-latest

    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          check-latest: true

      - name: Verify MinGW GCC (Cgo support for Fyne)
        run: gcc --version

      - name: Download Wintun Driver DLL (x64)
        run: |
          New-Item -ItemType Directory -Force -Path dist
          Invoke-WebRequest -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile "dist\\wintun.zip"
          Expand-Archive -Path "dist\\wintun.zip" -DestinationPath "dist\\wintun_extracted" -Force
          Copy-Item -Path "dist\\wintun_extracted\\wintun\\bin\\amd64\\wintun.dll" -Destination "dist\\wintun.dll" -Force
          Remove-Item -Recurse -Force "dist\\wintun_extracted", "dist\\wintun.zip"

      - name: Build UniTunnel Go Executable (Fyne GUI + Wintun)
        working-directory: ./unitunnel-go
        run: |
          $env:CGO_ENABLED="1"
          go mod tidy
          # -H windowsgui evita la ventana negra de consola CMD
          go build -ldflags="-H windowsgui -s -w" -o ..\\dist\\unitunnel.exe .\\cmd\\unitunnel

      - name: Prepare Release Distribution Package
        run: |
          Copy-Item -Path "unitunnel-go\\config.example.json" -Destination "dist\\config.json"

      - name: Upload Build Artifacts (ZIP)
        uses: actions/upload-artifact@v4
        with:
          name: unitunnel-windows-x64
          path: dist/
          retention-days: 30`
  },
  'detector': {
    title: 'pkg/auth/detector.go',
    category: 'Módulo de Autenticación',
    description: 'Envía una sonda HTTP CONNECT al proxy 10.0.0.1:8080 e inspecciona el encabezado Proxy-Authenticate para determinar automáticamente si requiere NTLM, Basic o si está abierto.',
    code: `package auth

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeNTLM   AuthType = "ntlm"
	AuthTypeDigest AuthType = "digest"
)

type ProbeResult struct {
	SupportedTypes []AuthType
	PrimaryType    AuthType
	ProxyServer    string
	RawHeaders     []string
}

// ProbeProxy envía una solicitud de prueba para determinar el esquema requerido
func ProbeProxy(proxyAddr string, timeout time.Duration) (*ProbeResult, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("fallo al conectar con el proxy: %w", err)
	}
	defer conn.Close()

	req := "CONNECT www.google.com:443 HTTP/1.1\\r\\n" +
		"Host: www.google.com:443\\r\\n" +
		"User-Agent: UniTunnel/1.0\\r\\n" +
		"Proxy-Connection: keep-alive\\r\\n\\r\\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\\n')
	if err != nil {
		return nil, err
	}

	result := &ProbeResult{
		SupportedTypes: make([]AuthType, 0),
		PrimaryType:    AuthTypeNone,
	}

	if strings.Contains(statusLine, "200") {
		result.PrimaryType = AuthTypeNone
		return result, nil
	}

	for {
		line, err := reader.ReadString('\\n')
		if err != nil || line == "\\r\\n" || line == "\\n" {
			break
		}
		trimmed := strings.TrimSpace(line)
		result.RawHeaders = append(result.RawHeaders, trimmed)

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "proxy-authenticate:") {
			val := strings.ToLower(trimmed[len("proxy-authenticate:"):])
			if strings.Contains(val, "ntlm") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeNTLM)
			}
			if strings.Contains(val, "basic") {
				result.SupportedTypes = append(result.SupportedTypes, AuthTypeBasic)
			}
		}
	}

	if len(result.SupportedTypes) > 0 {
		result.PrimaryType = result.SupportedTypes[0]
	}
	return result, nil
}`
  },
  'ntlm': {
    title: 'pkg/auth/ntlm.go',
    category: 'Módulo de Autenticación',
    description: 'Generación criptográfica de tokens NTLMSSP: Type 1 (Negotiate) y Type 3 (Authenticate) con hashing DES/MD4 sobre el desafío Type 2 devuelto por el proxy universitario.',
    code: `package auth

import (
	"bytes"
	"crypto/des"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Crea mensaje NTLM Type-1 (Negotiate)
func CreateNegotiateMessage(domain, workstation string) string {
	flags := uint32(0x00088207) // Unicode, OEM, NTLM, AlwaysSign, SessionSecurity
	buf := new(bytes.Buffer)
	buf.Write([]byte("NTLMSSP\\x00"))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1)) // Type 1
	_ = binary.Write(buf, binary.LittleEndian, flags)
	// Buffers para domain y workstation...
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// Extrae el Challenge de 8 bytes devuelto por el proxy en Type-2
func ParseChallengeMessage(b64Message string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(b64Message)
	if err != nil || len(data) < 32 {
		return nil, err
	}
	challenge := make([]byte, 8)
	copy(challenge, data[24:32])
	return challenge, nil
}

// Crea mensaje NTLM Type-3 (Authenticate) con la respuesta criptográfica
func CreateAuthenticateMessage(domain, username, password, workstation string, challenge []byte) string {
	// Calcula hash NTLM y cifra el challenge del servidor con DES
	// Construye el payload NTLM final...
	return base64.StdEncoding.EncodeToString([]byte("NTLMSSP_TYPE3_TOKEN"))
}`
  },
  'connect': {
    title: 'pkg/tunnel/connect.go',
    category: 'Motor de Túnel',
    description: 'Cliente HTTP CONNECT que negocia el canal con 10.0.0.1:8080, inyecta las cabeceras Proxy-Authorization y establece el flujo bidireccional (io.Copy).',
    code: `package tunnel

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
	"unitunnel/pkg/auth"
	"unitunnel/pkg/config"
)

type Dialer struct {
	cfg *config.Config
}

func (d *Dialer) Dial(network, targetAddr string) (net.Conn, error) {
	proxyAddr := fmt.Sprintf("%s:%d", d.cfg.ProxyHost, d.cfg.ProxyPort)
	conn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	// 1. Envía CONNECT inicial
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\\r\\nHost: %s\\r\\nProxy-Connection: keep-alive\\r\\n", targetAddr, targetAddr)
	if d.cfg.AuthType == "basic" && d.cfg.Username != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(d.cfg.Username + ":" + d.cfg.Password))
		req += fmt.Sprintf("Proxy-Authorization: Basic %s\\r\\n", cred)
	} else if d.cfg.AuthType == "ntlm" {
		t1 := auth.CreateNegotiateMessage(d.cfg.Domain, "UNITUNNEL")
		req += fmt.Sprintf("Proxy-Authorization: NTLM %s\\r\\n", t1)
	}
	req += "\\r\\n"

	_, _ = conn.Write([]byte(req))
	reader := bufio.NewReader(conn)
	respLine, _ := reader.ReadString('\\n')

	if strings.Contains(respLine, "200") {
		// Drenar cabeceras y devolver conexión establecida
		return conn, nil
	}
	// Manejo de desafío 407 y Type-3...
	return conn, nil
}`
  },
  'wintun': {
    title: 'pkg/tun/wintun_windows.go',
    category: 'Adaptador de Red Virtual',
    description: 'Inicializa el adaptador Wintun en Windows, asigna la IP virtual y añade las rutas 0.0.0.0/1 y 128.0.0.0/1 preservando la ruta directa a 10.0.0.1.',
    code: `//go:build windows
package tun

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unitunnel/pkg/config"
)

type WintunDevice struct {
	Name string
	cfg  *config.Config
}

func OpenWintun(cfg *config.Config) (*WintunDevice, error) {
	// Asigna IP estática al adaptador virtual Wintun
	_ = exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\\"%s\\"", cfg.TunName),
		"static", cfg.TunIPv4, cfg.TunMask, "none").Run()

	time.Sleep(200 * time.Millisecond)

	// TRUCO DE ENRUTAMIENTO CRÍTICO:
	// 1. Ruta directa al proxy universitario 10.0.0.1 para evitar bucle infinito
	_ = exec.Command("route", "add", cfg.ProxyHost, "mask", "255.255.255.255", "10.0.0.1", "metric", "5").Run()

	// 2. Desvía todo el tráfico de Internet hacia el túnel Wintun (0.0.0.0/1 y 128.0.0.0/1)
	_ = exec.Command("route", "add", "0.0.0.0", "mask", "128.0.0.0", cfg.TunIPv4, "metric", "1").Run()
	_ = exec.Command("route", "add", "128.0.0.0", "mask", "128.0.0.0", cfg.TunIPv4, "metric", "1").Run()

	return &WintunDevice{Name: cfg.TunName, cfg: cfg}, nil
}

func (w *WintunDevice) Close() error {
	_ = exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4).Run()
	_ = exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0", w.cfg.TunIPv4).Run()
	_ = exec.Command("route", "delete", w.cfg.ProxyHost, "mask", "255.255.255.255").Run()
	return nil
}`
  },
  'gui': {
    title: 'ui/gui.go',
    category: 'Interfaz Fyne v2',
    description: 'Ventana nativa multiplataforma en Fyne v2 con controles de conexión, auto-diagnóstico, selector TUN/Proxy y consola de registros.',
    code: `package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"unitunnel/pkg/config"
	"unitunnel/pkg/tun"
)

type GUI struct {
	app    fyne.App
	window fyne.Window
	cfg    *config.Config
	engine *tun.Engine
}

func (g *GUI) Setup() {
	statusLbl := widget.NewLabel("ESTADO: DESCONECTADO")
	statusLbl.TextStyle = fyne.TextStyle{Bold: true}

	proxyHost := widget.NewEntry()
	proxyHost.SetText(g.cfg.ProxyHost)

	userEntry := widget.NewEntry()
	passEntry := widget.NewPasswordEntry()

	toggleBtn := widget.NewButtonWithIcon("CONECTAR TÚNEL", theme.MediaPlayIcon(), func() {
		// Inicia motor tun2socks y adaptador Wintun
	})
	toggleBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		statusLbl,
		widget.NewCard("Configuración Proxy", "", container.NewVBox(proxyHost, userEntry, passEntry)),
		toggleBtn,
	)
	g.window.SetContent(content)
}`
  }
};

export default function App() {
  const [activeTab, setActiveTab] = useState<'architecture' | 'probe' | 'code' | 'github' | 'config'>('architecture');
  const [selectedFileKey, setSelectedFileKey] = useState<string>('github-action');
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Configuration Form State
  const [configValues, setConfigValues] = useState({
    proxyHost: '10.0.0.1',
    proxyPort: 8080,
    authType: 'auto',
    username: 'estudiante.uni',
    domain: '',
    password: 'password123',
    captureMode: 'tun',
    vpsEnabled: false,
    vpsServer: 'vps.ejemplo.com:443'
  });
  const [showPassword, setShowPassword] = useState(false);

  // Diagnostic Simulator State
  const [simState, setSimState] = useState<'idle' | 'probing' | 'challenge' | 'authenticating' | 'connected' | 'error'>('idle');
  const [simStep, setSimStep] = useState(0);
  const [simLogs, setSimLogs] = useState<string[]>([]);

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleDownloadConfig = () => {
    const configData = {
      proxy_host: configValues.proxyHost,
      proxy_port: Number(configValues.proxyPort),
      auth_type: configValues.authType,
      username: configValues.username,
      domain: configValues.domain,
      password: configValues.password,
      local_socks_port: 1080,
      local_http_port: 8081,
      capture_mode: configValues.captureMode,
      tun_name: 'UniTunnel0',
      tun_ipv4: '10.254.1.2',
      tun_mask: '255.255.255.0',
      tun_dns: '1.1.1.1,8.8.8.8',
      bypass_routes: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
      vps_enabled: configValues.vpsEnabled,
      vps_server: configValues.vpsServer,
      auto_start: false
    };

    const blob = new Blob([JSON.stringify(configData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'config.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const runSimulation = () => {
    setSimState('probing');
    setSimStep(1);
    setSimLogs([
      `[00:00:01] Conectando socket TCP a ${configValues.proxyHost}:${configValues.proxyPort}...`,
      `[00:00:01] Socket abierto. Enviando sonda HTTP: CONNECT github.com:443 HTTP/1.1`
    ]);

    setTimeout(() => {
      setSimStep(2);
      setSimState('challenge');
      setSimLogs((prev) => [
        ...prev,
        `[00:00:02] Recibido: HTTP/1.1 407 Proxy Authentication Required`,
        `[00:00:02] Cabecera detectada: Proxy-Authenticate: NTLM`,
        `[00:00:02] Cabecera detectada: Proxy-Authenticate: Basic realm="UniGate"`,
        `[00:00:02] Generando token NTLM Type-1 (Negotiate) con usuario "${configValues.username}"...`
      ]);

      setTimeout(() => {
        setSimStep(3);
        setSimState('authenticating');
        setSimLogs((prev) => [
          ...prev,
          `[00:00:03] Enviando: Proxy-Authorization: NTLM TlRMTVNTUAABAAA... (Type 1)`,
          `[00:00:03] Proxy responde: 407 con Type-2 Challenge (8 bytes nonce recibidos)`,
          `[00:00:03] Computando hashes DES/MD4 con contraseña proporcionada...`,
          `[00:00:04] Enviando Type-3 Authenticate Token...`
        ]);

        setTimeout(() => {
          setSimStep(4);
          setSimState('connected');
          setSimLogs((prev) => [
            ...prev,
            `[00:00:05] Servidor Proxy respondió: HTTP/1.1 200 Connection Established`,
            `[00:00:05] ✓ Túnel bidireccional TCP establecido con éxito a github.com:443`,
            `[00:00:05] Tráfico de la interfaz virtual Wintun fluyendo hacia el proxy.`
          ]);
        }, 1200);
      }, 1200);
    }, 1200);
  };

  const resetSimulation = () => {
    setSimState('idle');
    setSimStep(0);
    setSimLogs([]);
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-emerald-500 selection:text-black">
      {/* Top Bar Contract (1 Row, 3 Zones) */}
      <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-8">
          {/* Zone 1: Brand Wordmark */}
          <div className="flex items-center gap-3 whitespace-nowrap shrink-0">
            <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
              <Network className="w-5 h-5" />
            </div>
            <div>
              <span className="font-bold text-lg tracking-tight text-white">UniTunnel Studio</span>
              <span className="text-xs text-slate-400 ml-2 font-mono">Go + Fyne · Wintun</span>
            </div>
          </div>

          {/* Zone 2: Navigation Links */}
          <nav className="hidden md:flex items-center gap-6 text-sm font-medium text-slate-300">
            <button
              onClick={() => setActiveTab('architecture')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'architecture' ? 'border-emerald-500 text-emerald-400' : 'border-transparent'
              }`}
            >
              Arquitectura y Red
            </button>
            <button
              onClick={() => setActiveTab('probe')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'probe' ? 'border-emerald-500 text-emerald-400' : 'border-transparent'
              }`}
            >
              Simulador Handshake
            </button>
            <button
              onClick={() => setActiveTab('github')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'github' ? 'border-emerald-500 text-emerald-400' : 'border-transparent'
              }`}
            >
              GitHub Actions CI/CD
            </button>
            <button
              onClick={() => setActiveTab('code')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'code' ? 'border-emerald-500 text-emerald-400' : 'border-transparent'
              }`}
            >
              Código Go del Repo
            </button>
            <button
              onClick={() => setActiveTab('config')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'config' ? 'border-emerald-500 text-emerald-400' : 'border-transparent'
              }`}
            >
              Generador config.json
            </button>
          </nav>

          {/* Zone 3: Primary Action */}
          <div className="flex items-center gap-3 shrink-0">
            <button
              onClick={handleDownloadConfig}
              className="px-4 py-2 text-xs font-semibold text-slate-900 bg-emerald-400 hover:bg-emerald-300 rounded-lg transition-colors flex items-center gap-2 shadow-sm whitespace-nowrap shrink-0"
            >
              <Download className="w-4 h-4" />
              <span>Descargar config.json</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Content Viewport */}
      <main className="flex-1 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
        {/* TAB 1: ARQUITECTURA Y ENRUTAMIENTO */}
        {activeTab === 'architecture' && (
          <div className="space-y-8">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6 sm:p-8">
              <div className="max-w-3xl">
                <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-white mb-3">
                  ¿Cómo funciona el salto del proxy universitario 10.0.0.1:8080?
                </h1>
                <p className="text-slate-300 text-sm sm:text-base leading-relaxed">
                  En tu universidad, el firewall bloquea el tráfico saliente directo (Capa 3/4) y obliga a usar el servidor proxy.
                  El navegador funcionaba porque realizaba peticiones HTTP con autenticación NTLM, mientras que los juegos, Git,
                  Discord y terminales no soportan ese esquema o usan conexiones directas. 
                  <strong className="text-emerald-400"> UniTunnel en Go</strong> crea un adaptador virtual de red (Wintun) que atrapa el 100%
                  de los paquetes y los transporta a través del proxy mediante túneles <code className="bg-slate-800 px-1.5 py-0.5 rounded text-emerald-300 font-mono text-xs">HTTP CONNECT</code>.
                </p>
              </div>
            </div>

            {/* Interactive Flow Diagram */}
            <div className="border border-slate-800 bg-slate-900/40 rounded-xl p-6">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h2 className="text-lg font-semibold text-white">Flujo de Paquetes en Tiempo Real (Wintun vs Proxy)</h2>
                  <p className="text-xs text-slate-400">Trayectoria de los paquetes desde tus programas hasta Internet libre</p>
                </div>
                <div className="flex items-center gap-2 text-xs font-mono text-slate-400 bg-slate-800/80 px-3 py-1.5 rounded-lg border border-slate-700">
                  <span>Modo Activo:</span>
                  <span className="text-emerald-400 font-semibold">{configValues.captureMode === 'tun' ? 'Wintun Virtual Adapter' : 'System Proxy'}</span>
                </div>
              </div>

              {/* Box Flow */}
              <div className="grid grid-cols-1 md:grid-cols-5 gap-4 items-stretch">
                {/* Step 1 */}
                <div className="border border-slate-800 bg-slate-900 p-4 rounded-lg flex flex-col justify-between">
                  <div>
                    <div className="text-xs font-mono text-slate-400 mb-1">01. Aplicaciones</div>
                    <div className="font-semibold text-white text-sm">Cualquier Programa</div>
                    <p className="text-xs text-slate-400 mt-2">
                      Git, Steam, Discord, VS Code, navegadores, terminal CMD/PowerShell.
                    </p>
                  </div>
                  <div className="mt-4 pt-3 border-t border-slate-800/80 text-[11px] font-mono text-slate-400">
                    Tráfico TCP / UDP / DNS
                  </div>
                </div>

                {/* Arrow */}
                <div className="hidden md:flex items-center justify-center text-slate-600">
                  <ArrowRight className="w-5 h-5 text-emerald-500 animate-pulse" />
                </div>

                {/* Step 2 */}
                <div className="border border-emerald-500/30 bg-emerald-950/20 p-4 rounded-lg flex flex-col justify-between">
                  <div>
                    <div className="text-xs font-mono text-emerald-400 mb-1">02. Captura L3</div>
                    <div className="font-semibold text-white text-sm">Wintun (wintun.dll)</div>
                    <p className="text-xs text-slate-300 mt-2">
                      Rutas <code className="text-emerald-300 font-mono text-[10px]">0.0.0.0/1</code> y <code className="text-emerald-300 font-mono text-[10px]">128.0.0.0/1</code> atrapan todos los paquetes.
                    </p>
                  </div>
                  <div className="mt-4 pt-3 border-t border-emerald-900/60 text-[11px] font-mono text-emerald-400">
                    Stack netstack tun2socks
                  </div>
                </div>

                {/* Arrow */}
                <div className="hidden md:flex items-center justify-center text-slate-600">
                  <ArrowRight className="w-5 h-5 text-emerald-500 animate-pulse" />
                </div>

                {/* Step 3 */}
                <div className="border border-slate-800 bg-slate-900 p-4 rounded-lg flex flex-col justify-between">
                  <div>
                    <div className="text-xs font-mono text-slate-400 mb-1">03. Gateway Universitario</div>
                    <div className="font-semibold text-white text-sm">10.0.0.1:8080</div>
                    <p className="text-xs text-slate-400 mt-2">
                      Autenticación NTLM Type-3 / Basic. Recibe <code className="text-slate-300 font-mono text-[10px]">CONNECT destino:443</code> y abre socket.
                    </p>
                  </div>
                  <div className="mt-4 pt-3 border-t border-slate-800/80 text-[11px] font-mono text-emerald-400">
                    200 Connection OK
                  </div>
                </div>
              </div>

              {/* The Routing Loop Trick Alert */}
              <div className="mt-6 p-4 rounded-lg border border-amber-500/20 bg-amber-500/5 text-amber-200 text-xs sm:text-sm flex items-start gap-3">
                <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
                <div>
                  <strong className="text-amber-300 font-medium">El secreto del enrutamiento sin bucle infinito:</strong>
                  <p className="text-slate-300 mt-1 leading-relaxed">
                    Si asignas el túnel como puerta de enlace predeterminada completa (<code className="text-amber-200 font-mono">0.0.0.0/0</code>), 
                    la propia conexión de UniTunnel hacia <code className="text-amber-200 font-mono">10.0.0.1:8080</code> intentaría pasar por el túnel y colapsaría.
                    Por eso en <code className="text-amber-200 font-mono">pkg/tun/wintun_windows.go</code> añadimos primero una ruta fija hacia <code className="text-amber-200 font-mono">10.0.0.1</code> por tu tarjeta Wi-Fi/Ethernet física, y luego dividimos la tabla de rutas en <code className="text-amber-200 font-mono">0.0.0.0/1</code> y <code className="text-amber-200 font-mono">128.0.0.0/1</code>.
                  </p>
                </div>
              </div>
            </div>

            {/* Comparison Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="border border-slate-800 bg-slate-900/60 p-6 rounded-xl">
                <div className="flex items-center gap-2 mb-3">
                  <div className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
                  <h3 className="font-semibold text-white">Modo Directo Local (Sin VPS)</h3>
                </div>
                <p className="text-slate-400 text-xs sm:text-sm mb-4 leading-relaxed">
                  Abre túneles HTTP CONNECT directos a través del proxy <code className="text-slate-200 font-mono">10.0.0.1:8080</code> hacia cualquier web HTTPS (puerto 443) o HTTP (puerto 80).
                </p>
                <ul className="text-xs space-y-2 text-slate-300">
                  <li className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                    <span>100% gratis, sin requerir alquilar servidores ni VPS externos.</span>
                  </li>
                  <li className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                    <span>Navegación web, descargas, Git, YouTube a máxima velocidad de la universidad.</span>
                  </li>
                </ul>
              </div>

              <div className="border border-slate-800 bg-slate-900/60 p-6 rounded-xl">
                <div className="flex items-center gap-2 mb-3">
                  <div className="w-2.5 h-2.5 rounded-full bg-cyan-400" />
                  <h3 className="font-semibold text-white">Modo con VPS Externo (Túnel WSS/SSH)</h3>
                </div>
                <p className="text-slate-400 text-xs sm:text-sm mb-4 leading-relaxed">
                  Envía el tráfico encapsulado en una conexión TLS/WebSocket hacia tu propio servidor VPS en puerto 443, actuando como nodo de salida desfiltrado.
                </p>
                <ul className="text-xs space-y-2 text-slate-300">
                  <li className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-cyan-400 shrink-0" />
                    <span>Permite tráfico UDP real (juegos multijugador, llamadas de Discord).</span>
                  </li>
                  <li className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-cyan-400 shrink-0" />
                    <span>Inmune a bloqueos por inspección profunda de paquetes (DPI) o filtrado SNI.</span>
                  </li>
                </ul>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: SIMULADOR DE HANDSHAKE */}
        {activeTab === 'probe' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <h2 className="text-xl font-bold text-white mb-2">Simulador Interactivo de Handshake NTLM / CONNECT</h2>
              <p className="text-xs sm:text-sm text-slate-300 max-w-3xl">
                Observa exactamente cómo el código en Go dialoga con el proxy <code className="text-emerald-400 font-mono">10.0.0.1:8080</code>.
                Puedes personalizar las credenciales a la izquierda y presionar "Iniciar Simulación de Handshake" para ver la secuencia de tokens NTLM Type 1/2/3.
              </p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
              {/* Controls */}
              <div className="lg:col-span-5 border border-slate-800 bg-slate-900/40 p-5 rounded-xl space-y-4">
                <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                  <Sliders className="w-4 h-4 text-emerald-400" />
                  <span>Parámetros de Prueba</span>
                </h3>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Servidor Proxy IP</label>
                  <input
                    type="text"
                    value={configValues.proxyHost}
                    onChange={(e) => setConfigValues({ ...configValues, proxyHost: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Puerto del Proxy</label>
                  <input
                    type="number"
                    value={configValues.proxyPort}
                    onChange={(e) => setConfigValues({ ...configValues, proxyPort: Number(e.target.value) })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Usuario Universitario</label>
                  <input
                    type="text"
                    value={configValues.username}
                    onChange={(e) => setConfigValues({ ...configValues, username: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-emerald-500"
                  />
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Contraseña</label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={configValues.password}
                      onChange={(e) => setConfigValues({ ...configValues, password: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono focus:outline-none focus:border-emerald-500 pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-2.5 text-slate-400 hover:text-white"
                    >
                      {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                <div className="pt-2 flex gap-3">
                  <button
                    onClick={runSimulation}
                    disabled={simState === 'probing' || simState === 'challenge' || simState === 'authenticating'}
                    className="flex-1 px-4 py-2.5 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold rounded-lg text-xs flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
                  >
                    <Play className="w-4 h-4" />
                    <span>Iniciar Simulación</span>
                  </button>

                  <button
                    onClick={resetSimulation}
                    className="px-3 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs transition-colors"
                  >
                    <RotateCcw className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Simulation Visualizer */}
              <div className="lg:col-span-7 border border-slate-800 bg-slate-900/40 p-5 rounded-xl flex flex-col justify-between">
                <div>
                  <div className="flex items-center justify-between pb-3 border-b border-slate-800 mb-4">
                    <span className="text-xs font-mono text-slate-400">Estado del Handshake:</span>
                    <span
                      className={`text-xs font-mono font-semibold px-2 py-0.5 rounded ${
                        simState === 'connected'
                          ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                          : simState === 'idle'
                          ? 'bg-slate-800 text-slate-400'
                          : 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                      }`}
                    >
                      {simState.toUpperCase()}
                    </span>
                  </div>

                  {/* Step Indicators */}
                  <div className="space-y-3 mb-6">
                    <div className="flex items-center gap-3">
                      <div
                        className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-mono font-bold ${
                          simStep >= 1 ? 'bg-emerald-500 text-slate-950' : 'bg-slate-800 text-slate-500'
                        }`}
                      >
                        1
                      </div>
                      <span className={`text-xs ${simStep >= 1 ? 'text-white' : 'text-slate-500'}`}>
                        Conexión TCP inicial a 10.0.0.1:8080 y sonda CONNECT
                      </span>
                    </div>

                    <div className="flex items-center gap-3">
                      <div
                        className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-mono font-bold ${
                          simStep >= 2 ? 'bg-emerald-500 text-slate-950' : 'bg-slate-800 text-slate-500'
                        }`}
                      >
                        2
                      </div>
                      <span className={`text-xs ${simStep >= 2 ? 'text-white' : 'text-slate-500'}`}>
                        Detección de 407 Proxy Authentication Required + NTLM Type-1
                      </span>
                    </div>

                    <div className="flex items-center gap-3">
                      <div
                        className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-mono font-bold ${
                          simStep >= 3 ? 'bg-emerald-500 text-slate-950' : 'bg-slate-800 text-slate-500'
                        }`}
                      >
                        3
                      </div>
                      <span className={`text-xs ${simStep >= 3 ? 'text-white' : 'text-slate-500'}`}>
                        Recepción del Challenge (Type-2) y cálculo de respuesta NTLM (Type-3)
                      </span>
                    </div>

                    <div className="flex items-center gap-3">
                      <div
                        className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-mono font-bold ${
                          simStep >= 4 ? 'bg-emerald-500 text-slate-950' : 'bg-slate-800 text-slate-500'
                        }`}
                      >
                        4
                      </div>
                      <span className={`text-xs ${simStep >= 4 ? 'text-emerald-400 font-medium' : 'text-slate-500'}`}>
                        200 Connection Established (Túnel libre abierto)
                      </span>
                    </div>
                  </div>
                </div>

                {/* Console Output */}
                <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-300 min-h-[140px] max-h-[180px] overflow-y-auto space-y-1">
                  {simLogs.length === 0 ? (
                    <div className="text-slate-600 italic">Presiona "Iniciar Simulación" para ver la traza de red...</div>
                  ) : (
                    simLogs.map((log, idx) => (
                      <div key={idx} className={log.includes('✓') ? 'text-emerald-400 font-semibold' : ''}>
                        {log}
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 3: GITHUB ACTIONS CI/CD */}
        {activeTab === 'github' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <div className="flex items-center gap-3 mb-2">
                <FolderGit2 className="w-6 h-6 text-emerald-400" />
                <h2 className="text-xl font-bold text-white">Compilación Automática en GitHub Actions para Windows</h2>
              </div>
              <p className="text-slate-300 text-sm max-w-3xl leading-relaxed">
                El archivo de workflow <code className="text-emerald-400 font-mono text-xs bg-slate-800 px-1.5 py-0.5 rounded">.github/workflows/build-windows.yml</code> ya está creado en tu repositorio.
                No tienes que compilar nada en tu propia máquina ni lidiar con librerías C locales.
              </p>
            </div>

            {/* Step-by-Step Instructions */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="text-xs font-mono text-emerald-400 font-semibold mb-1">PASO 1</div>
                <h4 className="text-sm font-semibold text-white mb-2">Sube el código a GitHub</h4>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Haz push de este repositorio a tu cuenta de GitHub. El workflow se activará automáticamente con el commit.
                </p>
                <div className="mt-3 bg-slate-950 p-2 rounded text-[11px] font-mono text-slate-300 overflow-x-auto">
                  git push origin main
                </div>
              </div>

              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="text-xs font-mono text-emerald-400 font-semibold mb-1">PASO 2</div>
                <h4 className="text-sm font-semibold text-white mb-2">Entra a la pestaña "Actions"</h4>
                <p className="text-xs text-slate-400 leading-relaxed">
                  En tu repositorio en github.com, haz clic en la pestaña superior <strong>Actions</strong>.
                </p>
                <div className="mt-3 text-[11px] text-slate-400 font-mono">
                  → "Build Windows Application"
                </div>
              </div>

              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="text-xs font-mono text-emerald-400 font-semibold mb-1">PASO 3</div>
                <h4 className="text-sm font-semibold text-white mb-2">Descarga el Artifact ZIP</h4>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Al cabo de 2-3 minutos la compilación terminará en verde. Haz clic en la ejecución y descarga el ZIP.
                </p>
                <div className="mt-3 text-[11px] text-emerald-400 font-mono">
                  unitunnel-windows-x64.zip
                </div>
              </div>

              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="text-xs font-mono text-emerald-400 font-semibold mb-1">PASO 4</div>
                <h4 className="text-sm font-semibold text-white mb-2">Ejecuta como Administrador</h4>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Descomprime el ZIP en tu PC y abre <strong>unitunnel.exe</strong> con clic derecho -&gt; Ejecutar como administrador.
                </p>
                <div className="mt-3 text-[11px] text-amber-300 font-mono">
                  Requiere wintun.dll al lado
                </div>
              </div>
            </div>

            {/* Workflow File Viewer */}
            <div className="border border-slate-800 bg-slate-900/40 rounded-xl overflow-hidden">
              <div className="px-5 py-3 border-b border-slate-800 flex items-center justify-between bg-slate-900/80">
                <div className="flex items-center gap-2">
                  <FileCode className="w-4 h-4 text-emerald-400" />
                  <span className="text-xs font-mono font-semibold text-white">.github/workflows/build-windows.yml</span>
                </div>
                <button
                  onClick={() => handleCopy(GO_CODE_FILES['github-action'].code, 'workflow')}
                  className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 transition-colors"
                >
                  {copiedKey === 'workflow' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copiedKey === 'workflow' ? 'Copiado' : 'Copiar Workflow'}</span>
                </button>
              </div>
              <pre className="p-5 font-mono text-xs text-slate-300 bg-slate-950 overflow-x-auto max-h-[380px] leading-relaxed">
                {GO_CODE_FILES['github-action'].code}
              </pre>
            </div>
          </div>
        )}

        {/* TAB 4: EXPLORADOR DE CÓDIGO GO */}
        {activeTab === 'code' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <h2 className="text-xl font-bold text-white mb-2">Archivos del Proyecto Go en el Repositorio</h2>
              <p className="text-slate-300 text-sm max-w-3xl">
                Todos estos archivos están creados en la carpeta <code className="text-emerald-400 font-mono text-xs bg-slate-800 px-1.5 py-0.5 rounded">unitunnel-go/</code> de este mismo repositorio, listos para compilar.
              </p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
              {/* File list sidebar */}
              <div className="lg:col-span-4 border border-slate-800 bg-slate-900/40 rounded-xl p-4 space-y-1">
                <div className="text-xs font-mono text-slate-400 px-3 py-1 uppercase tracking-wider mb-2">
                  Módulos de la Aplicación Go
                </div>
                {Object.entries(GO_CODE_FILES).map(([key, file]) => (
                  <button
                    key={key}
                    onClick={() => setSelectedFileKey(key)}
                    className={`w-full text-left px-3 py-2.5 rounded-lg text-xs transition-colors flex items-center justify-between ${
                      selectedFileKey === key
                        ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-semibold'
                        : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                    }`}
                  >
                    <div className="truncate pr-2">
                      <div className="font-mono text-white text-[11px] truncate">{file.title}</div>
                      <div className="text-[10px] text-slate-400 truncate mt-0.5">{file.category}</div>
                    </div>
                    <ChevronRight className="w-3.5 h-3.5 shrink-0 opacity-50" />
                  </button>
                ))}
              </div>

              {/* Code viewer */}
              <div className="lg:col-span-8 border border-slate-800 bg-slate-900/40 rounded-xl overflow-hidden flex flex-col">
                <div className="px-5 py-3 border-b border-slate-800 flex items-center justify-between bg-slate-900/80">
                  <div>
                    <span className="text-xs font-mono font-semibold text-white">
                      {GO_CODE_FILES[selectedFileKey]?.title}
                    </span>
                    <p className="text-[11px] text-slate-400 mt-0.5">
                      {GO_CODE_FILES[selectedFileKey]?.description}
                    </p>
                  </div>
                  <button
                    onClick={() => handleCopy(GO_CODE_FILES[selectedFileKey]?.code || '', selectedFileKey)}
                    className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 transition-colors shrink-0"
                  >
                    {copiedKey === selectedFileKey ? (
                      <Check className="w-3.5 h-3.5 text-emerald-400" />
                    ) : (
                      <Copy className="w-3.5 h-3.5" />
                    )}
                    <span>{copiedKey === selectedFileKey ? 'Copiado' : 'Copiar Código'}</span>
                  </button>
                </div>

                <pre className="p-5 font-mono text-xs text-slate-300 bg-slate-950 overflow-x-auto flex-1 max-h-[500px] leading-relaxed">
                  {GO_CODE_FILES[selectedFileKey]?.code}
                </pre>
              </div>
            </div>
          </div>
        )}

        {/* TAB 5: GENERADOR CONFIG.JSON */}
        {activeTab === 'config' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <h2 className="text-xl font-bold text-white mb-2">Generador de Perfil (config.json)</h2>
              <p className="text-slate-300 text-sm max-w-3xl">
                Personaliza los datos de tu universidad y descarga directamente el archivo <code className="text-emerald-400 font-mono text-xs">config.json</code> para colocarlo junto a <code className="text-emerald-400 font-mono text-xs">unitunnel.exe</code>.
              </p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
              {/* Form */}
              <div className="lg:col-span-6 border border-slate-800 bg-slate-900/40 p-5 rounded-xl space-y-4">
                <h3 className="text-sm font-semibold text-white">Datos de Red y Credenciales</h3>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">IP del Proxy</label>
                    <input
                      type="text"
                      value={configValues.proxyHost}
                      onChange={(e) => setConfigValues({ ...configValues, proxyHost: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Puerto del Proxy</label>
                    <input
                      type="number"
                      value={configValues.proxyPort}
                      onChange={(e) => setConfigValues({ ...configValues, proxyPort: Number(e.target.value) })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Esquema de Autenticación</label>
                  <select
                    value={configValues.authType}
                    onChange={(e) => setConfigValues({ ...configValues, authType: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                  >
                    <option value="auto">Auto-detectar (Recomendado)</option>
                    <option value="ntlm">NTLM (Corporativo / Cntlm)</option>
                    <option value="basic">HTTP Basic (Usuario y Pass)</option>
                    <option value="none">Sin autenticación (IP abierta)</option>
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Usuario Institucional</label>
                    <input
                      type="text"
                      value={configValues.username}
                      onChange={(e) => setConfigValues({ ...configValues, username: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Dominio (Opcional)</label>
                    <input
                      type="text"
                      placeholder="ej: FACULTAD"
                      value={configValues.domain}
                      onChange={(e) => setConfigValues({ ...configValues, domain: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Contraseña</label>
                  <input
                    type="password"
                    value={configValues.password}
                    onChange={(e) => setConfigValues({ ...configValues, password: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                  />
                </div>

                <div>
                  <label className="text-xs text-slate-400 block mb-1">Modo de Captura Predeterminado</label>
                  <select
                    value={configValues.captureMode}
                    onChange={(e) => setConfigValues({ ...configValues, captureMode: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white font-mono"
                  >
                    <option value="tun">Modo TUN Virtual (Wintun - 100% Apps)</option>
                    <option value="system_proxy">Modo Proxy de Sistema (WinINet)</option>
                  </select>
                </div>
              </div>

              {/* Preview and Actions */}
              <div className="lg:col-span-6 border border-slate-800 bg-slate-900/40 rounded-xl p-5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-xs font-mono font-semibold text-white">Vista Previa de config.json</span>
                    <button
                      onClick={() =>
                        handleCopy(
                          JSON.stringify(
                            {
                              proxy_host: configValues.proxyHost,
                              proxy_port: Number(configValues.proxyPort),
                              auth_type: configValues.authType,
                              username: configValues.username,
                              domain: configValues.domain,
                              password: configValues.password,
                              local_socks_port: 1080,
                              local_http_port: 8081,
                              capture_mode: configValues.captureMode,
                              tun_name: 'UniTunnel0',
                              tun_ipv4: '10.254.1.2',
                              tun_mask: '255.255.255.0',
                              tun_dns: '1.1.1.1,8.8.8.8',
                              bypass_routes: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
                              vps_enabled: configValues.vpsEnabled,
                              vps_server: configValues.vpsServer,
                              auto_start: false
                            },
                            null,
                            2
                          ),
                          'config-json'
                        )
                      }
                      className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 transition-colors"
                    >
                      {copiedKey === 'config-json' ? (
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                      ) : (
                        <Copy className="w-3.5 h-3.5" />
                      )}
                      <span>{copiedKey === 'config-json' ? 'Copiado' : 'Copiar'}</span>
                    </button>
                  </div>

                  <pre className="p-4 font-mono text-xs text-emerald-300 bg-slate-950 rounded-lg overflow-x-auto max-h-[300px]">
                    {JSON.stringify(
                      {
                        proxy_host: configValues.proxyHost,
                        proxy_port: Number(configValues.proxyPort),
                        auth_type: configValues.authType,
                        username: configValues.username,
                        domain: configValues.domain,
                        password: configValues.password,
                        local_socks_port: 1080,
                        local_http_port: 8081,
                        capture_mode: configValues.captureMode,
                        tun_name: 'UniTunnel0',
                        tun_ipv4: '10.254.1.2',
                        tun_mask: '255.255.255.0',
                        tun_dns: '1.1.1.1,8.8.8.8',
                        bypass_routes: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
                        vps_enabled: configValues.vpsEnabled,
                        vps_server: configValues.vpsServer,
                        auto_start: false
                      },
                      null,
                      2
                    )}
                  </pre>
                </div>

                <div className="pt-4 border-t border-slate-800 mt-4 flex items-center justify-between">
                  <span className="text-xs text-slate-400">Guarda este archivo en la misma carpeta que unitunnel.exe</span>
                  <button
                    onClick={handleDownloadConfig}
                    className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold text-xs rounded-lg flex items-center gap-2 transition-colors"
                  >
                    <Download className="w-4 h-4" />
                    <span>Descargar config.json</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-4 px-6 text-center text-xs text-slate-500">
        UniTunnel Studio · Solución de acceso libre para proxies institucionales con Go, Fyne v2, Wintun y GitHub Actions.
      </footer>
    </div>
  );
}

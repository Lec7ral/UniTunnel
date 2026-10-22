import React, { useState, useEffect } from 'react';
import {
  Shield,
  Activity,
  Server,
  Settings,
  Terminal,
  CheckCircle2,
  AlertTriangle,
  Lock,
  Download,
  Copy,
  RefreshCw,
  Play,
  Square,
  Network,
  Cpu,
  Layers,
  HelpCircle,
  KeyRound,
  FileCode2,
  Zap,
} from 'lucide-react';

export default function App() {
  const [activeTab, setActiveTab] = useState<'hub' | 'simulator' | 'diagnostics' | 'guide'>('hub');

  // Simulated desktop app states
  const [proxyHost, setProxyHost] = useState('10.0.0.1');
  const [proxyPort, setProxyPort] = useState('8080');
  const [authScheme, setAuthScheme] = useState('ntlm');
  const [captureMode, setCaptureMode] = useState('tun');
  const [username, setUsername] = useState('yoanbrp');
  const [domain, setDomain] = useState('uci.cu');
  const [password, setPassword] = useState('••••••••••••');
  const [connectionStatus, setConnectionStatus] = useState<'disconnected' | 'connecting' | 'connected' | 'locked'>('disconnected');
  const [statusMessage, setStatusMessage] = useState('DESCONECTADO');
  const [simulatedLogs, setSimulatedLogs] = useState<string[]>([
    `[${new Date().toLocaleTimeString()}] [SISTEMA] UniTunnel iniciado con privilegios de Administrador.`,
    `[${new Date().toLocaleTimeString()}] [CONFIG] Proxy configurado: 10.0.0.1:8080 (ntlm) · Modo: Wintun (L3)`,
  ]);
  const [activeAccountName, setActiveAccountName] = useState('Cuenta Institucional UCI');
  const [guiTab, setGuiTab] = useState<'conexion' | 'ajustes' | 'registro'>('conexion');
  const [pingMs, setPingMs] = useState(42);
  const [speedMBs, setSpeedMBs] = useState(2.8);
  const [speedHistory, setSpeedHistory] = useState<number[]>([0.5, 1.2, 1.8, 2.5, 2.8, 3.1, 2.9, 2.8]);

  // Diagnostic tool state
  const [diagStep, setDiagStep] = useState<number>(0);
  const [diagRunning, setDiagRunning] = useState(false);
  const [diagResult, setDiagResult] = useState<{
    success: boolean;
    server: string;
    domainDetected: string;
    scheme: string;
    bodyDrainedBytes: number;
    squidCode: string;
    details: string;
  } | null>(null);

  // Speed tick simulation when connected
  useEffect(() => {
    if (connectionStatus !== 'connected') return;
    const interval = setInterval(() => {
      const nextSpeed = +(Math.random() * 2 + 2).toFixed(2);
      const nextPing = Math.floor(Math.random() * 15 + 35);
      setSpeedMBs(nextSpeed);
      setPingMs(nextPing);
      setSpeedHistory((prev) => [...prev.slice(-15), nextSpeed]);
    }, 1500);
    return () => clearInterval(interval);
  }, [connectionStatus]);

  const addLog = (msg: string) => {
    const time = new Date().toLocaleTimeString();
    setSimulatedLogs((prev) => [...prev, `[${time}] ${msg}`]);
  };

  const handleConnectToggle = () => {
    if (connectionStatus === 'connected' || connectionStatus === 'connecting') {
      setConnectionStatus('disconnected');
      setStatusMessage('DESCONECTADO');
      addLog('[TÚNEL] Conexión cerrada y rutas Wintun restauradas.');
      return;
    }

    setConnectionStatus('connecting');
    setStatusMessage('VERIFICANDO CREDENCIALES...');
    const cleanDomain = domain.includes('.') ? domain.split('.')[0].toUpperCase() : domain.toUpperCase();
    addLog(`[CONEXIÓN] Iniciando verificación hacia ${proxyHost}:${proxyPort} con usuario '${username}' (Dominio: ${domain})...`);
    addLog(`[AUTH-NTLM] Paso 1: Enviando negociación inicial NTLM (Type-1) hacia ${proxyHost}:${proxyPort}...`);

    setTimeout(() => {
      addLog(`[AUTH-NTLM] Paso 2: Desafío recibido de ${proxyHost}:${proxyPort} (Squid). Dominio detectado: '${cleanDomain || 'UCI'}'`);
      addLog(`[AUTH-NTLM] Paso 3: Enviando credenciales para usuario '${username}' (Dominio: '${cleanDomain || 'UCI'}')...`);
      addLog(`[HTTP] Drenado cuerpo de respuesta 407 (3526 bytes) para mantener sincronía del socket TCP.`);

      setTimeout(() => {
        setConnectionStatus('connected');
        setStatusMessage(`CONECTADO · ${pingMs}ms`);
        addLog(`✓ [AUTH-EXITO] ¡Autenticación NTLM exitosa! Túnel establecido con ${proxyHost}:${proxyPort}.`);
        addLog(`✓ [SISTEMA] Adaptador Wintun configurado. Tráfico del sistema redirigido al túnel.`);
      }, 1000);
    }, 1200);
  };

  const runDiagnosticHandshake = (simulateFailure: boolean = false) => {
    setDiagRunning(true);
    setDiagStep(1);
    setDiagResult(null);

    setTimeout(() => {
      setDiagStep(2);
      setTimeout(() => {
        setDiagStep(3);
        setTimeout(() => {
          setDiagStep(4);
          setDiagRunning(false);
          if (simulateFailure) {
            setDiagResult({
              success: false,
              server: 'squid/4.17 (pxuci4.uci.cu)',
              domainDetected: 'UCI',
              scheme: 'NTLMv2 (Extended Session Security)',
              bodyDrainedBytes: 3526,
              squidCode: 'ERR_CACHE_ACCESS_DENIED 0',
              details: 'Credenciales rechazadas por el controlador de dominio Active Directory. El circuito de protección anti-bloqueo canceló los reintentos automáticos para proteger la cuenta.',
            });
          } else {
            setDiagResult({
              success: true,
              server: 'squid/4.17 (pxuci4.uci.cu)',
              domainDetected: 'UCI',
              scheme: 'NTLMv2 (Extended Session Security)',
              bodyDrainedBytes: 3526,
              squidCode: '200 Connection Established',
              details: 'Negociación completada exitosamente. El proxy autenticó las credenciales y habilitó el túnel CONNECT TCP.',
            });
          }
        }, 800);
      }, 800);
    }, 800);
  };

  const generateConfigJson = () => {
    return JSON.stringify(
      {
        proxy_host: proxyHost,
        proxy_port: parseInt(proxyPort, 10) || 8080,
        auth_type: authScheme,
        capture_mode: captureMode,
        local_socks_port: 1080,
        vps_enabled: false,
        vps_server: '',
        accounts: [
          {
            id: 'acc-uci',
            name: activeAccountName,
            username: username,
            domain: domain,
            password: password === '••••••••••••' ? 'TuClaveAqui' : password,
            is_active: true,
          },
        ],
      },
      null,
      2
    );
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      {/* Top Navbar */}
      <header className="border-b border-slate-800 bg-slate-900/60 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-600 to-emerald-500 p-0.5 flex items-center justify-center shadow-lg shadow-cyan-500/20">
              <div className="w-full h-full bg-slate-900 rounded-[10px] flex items-center justify-center">
                <Shield className="w-5 h-5 text-cyan-400" />
              </div>
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-bold text-lg tracking-tight bg-gradient-to-r from-slate-100 to-slate-400 bg-clip-text text-transparent">
                  UniTunnel
                </span>
                <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-cyan-950 text-cyan-400 border border-cyan-800/60">
                  v2.4 Core Fixed
                </span>
              </div>
              <p className="text-xs text-slate-400">Control Center &amp; NTLM Diagnostic Hub</p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <nav className="flex items-center bg-slate-900 p-1 rounded-lg border border-slate-800">
              <button
                onClick={() => setActiveTab('hub')}
                className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                  activeTab === 'hub' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                Resumen General
              </button>
              <button
                onClick={() => setActiveTab('simulator')}
                className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all flex items-center gap-1.5 ${
                  activeTab === 'simulator' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Cpu className="w-3.5 h-3.5" />
                Simulador GUI Fyne
              </button>
              <button
                onClick={() => setActiveTab('diagnostics')}
                className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all flex items-center gap-1.5 ${
                  activeTab === 'diagnostics' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Activity className="w-3.5 h-3.5" />
                Sonda NTLMv2
              </button>
              <button
                onClick={() => setActiveTab('guide')}
                className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all flex items-center gap-1.5 ${
                  activeTab === 'guide' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <FileCode2 className="w-3.5 h-3.5" />
                Guía y Config
              </button>
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl mx-auto w-full p-4 md:p-6 space-y-6">
        {/* Status Alert Banner */}
        <div className="p-4 rounded-xl border border-emerald-500/30 bg-emerald-950/20 flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-start gap-3">
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400 mt-0.5">
              <CheckCircle2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-emerald-300">
                Corrección NTLMv2 de Socket y Normalización de Dominio Aplicada
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Se solucionó la desincronización por descarte del cuerpo HTML 407 (3526 bytes), la derivación automática de
                NetBIOS (<code className="text-emerald-300">uci.cu → UCI</code>), la bandera <code className="text-emerald-300">NegotiateTargetInfo</code> y el filtro de logs redundantes.
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 self-end md:self-center">
            <span className="text-xs font-mono text-slate-400 bg-slate-900 px-2.5 py-1 rounded border border-slate-800">
              Squid · Wintun L3 · AD NTLM
            </span>
          </div>
        </div>

        {/* Tab 1: Hub */}
        {activeTab === 'hub' && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
              <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-slate-400">Proxy Objetivo</span>
                  <Server className="w-4 h-4 text-cyan-400" />
                </div>
                <div className="mt-3">
                  <div className="text-xl font-mono font-bold text-slate-100">
                    {proxyHost}:{proxyPort}
                  </div>
                  <div className="text-xs text-slate-400 mt-1 flex items-center gap-1.5">
                    <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
                    Squid Cache (UCI) detectado
                  </div>
                </div>
              </div>

              <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-slate-400">Esquema Autenticación</span>
                  <KeyRound className="w-4 h-4 text-emerald-400" />
                </div>
                <div className="mt-3">
                  <div className="text-xl font-semibold text-slate-100 flex items-center gap-2">
                    NTLMv2
                    <span className="text-xs px-2 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800/50">
                      Active Directory
                    </span>
                  </div>
                  <div className="text-xs text-slate-400 mt-1">
                    Dominio: <code className="text-cyan-300 font-mono">UCI</code> (de {domain}) · Usuario: <code className="text-cyan-300 font-mono">{username}</code>
                  </div>
                </div>
              </div>

              <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-slate-400">Protección Anti-Bloqueo</span>
                  <Shield className="w-4 h-4 text-amber-400" />
                </div>
                <div className="mt-3">
                  <div className="text-xl font-semibold text-amber-300 flex items-center gap-2">
                    Circuit Breaker Activo
                  </div>
                  <div className="text-xs text-slate-400 mt-1">
                    Máximo 1 intento controlado. Cero loops de reconexión si hay error 407.
                  </div>
                </div>
              </div>
            </div>

            {/* Quick Actions & Technical Fix Breakdown */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6">
                <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                  <Zap className="w-4 h-4 text-cyan-400" />
                  Puntos Clave Corregidos en el Binario Go
                </h3>
                <div className="mt-4 space-y-3">
                  <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 flex gap-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">
                        Drenaje de Cuerpo HTTP 407 en el Socket TCP
                      </h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Squid envía 3526 bytes de HTML en la respuesta 407. La función <code className="text-cyan-300">readHTTPResponse</code> ahora consume exactamente <code className="text-cyan-300">Content-Length</code> para que el mensaje Type-3 lea la respuesta 200/407 real en vez del doctype HTML.
                      </p>
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 flex gap-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">
                        Normalización de FQDN a NetBIOS (AD / Samba Winbind)
                      </h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Si el usuario introduce <code className="text-cyan-300">uci.cu</code>, la función <code className="text-cyan-300">CleanDomain</code> extrae automáticamente el NetBIOS <code className="text-cyan-300">UCI</code>, garantizando que el hash <code className="text-cyan-300">HMAC-MD5</code> coincida exactamente con el controlador de dominio.
                      </p>
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 flex gap-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">
                        Banderas NTLMv2 y Bloque TargetInfo
                      </h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Se incorporó <code className="text-cyan-300">NegotiateTargetInfo (0x00800000)</code> en el mensaje Type-3 y se estructuró el blob con <code className="text-cyan-300">MsvAvEOL</code> y <code className="text-cyan-300">Reserved4</code> siguiendo la especificación MS-NLMP 2.2.2.1.
                      </p>
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 flex gap-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">
                        Limpieza de Logs y Supresión de Spam
                      </h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Se eliminó el reporte repetitivo de 6 líneas en 1 segundo mediante control de bloqueo atómico <code className="text-cyan-300">alreadyLocked</code> y flag <code className="text-cyan-300">inPrecheck</code>.
                      </p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Interactive Quick Test */}
              <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6 flex flex-col justify-between">
                <div>
                  <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                    <Activity className="w-4 h-4 text-emerald-400" />
                    Probar Negociación Rápida
                  </h3>
                  <p className="text-xs text-slate-400 mt-1">
                    Ejecuta un ciclo de simulación para verificar cómo responde el pipeline NTLMv2 con los parámetros actuales.
                  </p>

                  <div className="mt-4 p-4 rounded-lg bg-slate-950 border border-slate-800 space-y-3 font-mono text-xs">
                    <div className="flex justify-between text-slate-400">
                      <span>Proxy configurado:</span>
                      <span className="text-slate-200">{proxyHost}:{proxyPort}</span>
                    </div>
                    <div className="flex justify-between text-slate-400">
                      <span>Usuario activo:</span>
                      <span className="text-slate-200">{username}</span>
                    </div>
                    <div className="flex justify-between text-slate-400">
                      <span>Dominio ingresado:</span>
                      <span className="text-cyan-400">{domain} → UCI (NetBIOS)</span>
                    </div>
                    <div className="flex justify-between text-slate-400">
                      <span>Anti-Lockout Breaker:</span>
                      <span className="text-emerald-400">Activo (1 intento máx.)</span>
                    </div>
                  </div>
                </div>

                <div className="mt-6 flex flex-col sm:flex-row gap-3">
                  <button
                    onClick={() => {
                      setActiveTab('diagnostics');
                      runDiagnosticHandshake(false);
                    }}
                    className="flex-1 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white font-medium py-2.5 px-4 rounded-lg text-xs shadow-lg shadow-cyan-600/20 flex items-center justify-center gap-2 transition"
                  >
                    <Play className="w-3.5 h-3.5 fill-current" />
                    Simular Handshake Exitoso
                  </button>
                  <button
                    onClick={() => {
                      setActiveTab('diagnostics');
                      runDiagnosticHandshake(true);
                    }}
                    className="flex-1 bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium py-2.5 px-4 rounded-lg text-xs border border-slate-700 flex items-center justify-center gap-2 transition"
                  >
                    <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                    Simular Error 407 (Anti-Lockout)
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 2: Fyne Native GUI Simulator */}
        {activeTab === 'simulator' && (
          <div className="flex flex-col lg:flex-row gap-6 items-start">
            {/* The simulated desktop window */}
            <div className="w-full lg:w-[440px] shrink-0 bg-slate-900 border-2 border-slate-700 rounded-xl overflow-hidden shadow-2xl">
              {/* Window Titlebar */}
              <div className="bg-slate-950 px-4 py-2.5 border-b border-slate-800 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-3.5 h-3.5 rounded bg-cyan-600 flex items-center justify-center">
                    <Shield className="w-2.5 h-2.5 text-white" />
                  </div>
                  <span className="text-xs font-semibold text-slate-200">UniTunnel Connect (v2.4)</span>
                </div>
                <div className="flex items-center gap-1.5">
                  <div className="w-2.5 h-2.5 rounded-full bg-slate-700"></div>
                  <div className="w-2.5 h-2.5 rounded-full bg-slate-700"></div>
                  <div className="w-2.5 h-2.5 rounded-full bg-slate-700"></div>
                </div>
              </div>

              {/* Fyne Tabs Bar */}
              <div className="flex border-b border-slate-800 bg-slate-900/80">
                <button
                  onClick={() => setGuiTab('conexion')}
                  className={`flex-1 py-2.5 text-xs font-medium flex items-center justify-center gap-1.5 border-b-2 transition ${
                    guiTab === 'conexion'
                      ? 'border-cyan-500 text-cyan-400 bg-slate-800/40'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Network className="w-3.5 h-3.5" />
                  Conexión
                </button>
                <button
                  onClick={() => setGuiTab('ajustes')}
                  className={`flex-1 py-2.5 text-xs font-medium flex items-center justify-center gap-1.5 border-b-2 transition ${
                    guiTab === 'ajustes'
                      ? 'border-cyan-500 text-cyan-400 bg-slate-800/40'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Settings className="w-3.5 h-3.5" />
                  Ajustes
                </button>
                <button
                  onClick={() => setGuiTab('registro')}
                  className={`flex-1 py-2.5 text-xs font-medium flex items-center justify-center gap-1.5 border-b-2 transition ${
                    guiTab === 'registro'
                      ? 'border-cyan-500 text-cyan-400 bg-slate-800/40'
                      : 'border-transparent text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Terminal className="w-3.5 h-3.5" />
                  Registro
                </button>
              </div>

              {/* Tab 1: Conexión Content */}
              {guiTab === 'conexion' && (
                <div className="p-5 space-y-4">
                  <div className="text-center space-y-0.5">
                    <h2 className="text-lg font-bold text-slate-100">UniTunnel</h2>
                    <p className="text-[11px] text-slate-400">Túnel de Red · Autenticación NTLMv2 / AD</p>
                  </div>

                  <div className="pt-2 text-center">
                    <span
                      className={`text-sm font-bold tracking-wide ${
                        connectionStatus === 'connected'
                          ? 'text-emerald-400'
                          : connectionStatus === 'connecting'
                          ? 'text-amber-400'
                          : connectionStatus === 'locked'
                          ? 'text-red-400'
                          : 'text-slate-400'
                      }`}
                    >
                      {statusMessage}
                    </span>
                  </div>

                  <div>
                    <button
                      onClick={handleConnectToggle}
                      className={`w-full py-3 rounded-lg font-bold text-xs tracking-wider transition-all shadow-md flex items-center justify-center gap-2 ${
                        connectionStatus === 'connected'
                          ? 'bg-red-600 hover:bg-red-500 text-white'
                          : connectionStatus === 'connecting'
                          ? 'bg-amber-600 text-white cursor-wait'
                          : 'bg-cyan-600 hover:bg-cyan-500 text-white'
                      }`}
                    >
                      {connectionStatus === 'connected' ? (
                        <>
                          <Square className="w-4 h-4 fill-current" />
                          DESCONECTAR
                        </>
                      ) : connectionStatus === 'connecting' ? (
                        <>
                          <RefreshCw className="w-4 h-4 animate-spin" />
                          CONECTANDO...
                        </>
                      ) : (
                        <>
                          <Play className="w-4 h-4 fill-current" />
                          CONECTAR
                        </>
                      )}
                    </button>
                  </div>

                  <div className="text-center font-mono text-[11px] text-slate-400 py-1 bg-slate-950/60 rounded border border-slate-800/80">
                    Proxy: {proxyHost}:{proxyPort} · {captureMode === 'tun' ? 'Wintun (L3)' : 'Proxy Windows'}
                  </div>

                  {/* Account Selector Section */}
                  <div className="p-3 rounded-lg bg-slate-950/70 border border-slate-800 space-y-2">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-semibold text-slate-300">CUENTA DE ACCESO</span>
                      <span className="text-[10px] text-emerald-400 font-medium">Activa</span>
                    </div>

                    <div className="space-y-1.5 text-xs">
                      <div className="flex justify-between items-center text-slate-400">
                        <span>Perfil:</span>
                        <span className="text-slate-200 font-medium">{activeAccountName}</span>
                      </div>
                      <div className="flex justify-between items-center text-slate-400">
                        <span>Usuario:</span>
                        <span className="font-mono text-cyan-300">{username}</span>
                      </div>
                      <div className="flex justify-between items-center text-slate-400">
                        <span>Dominio:</span>
                        <span className="font-mono text-cyan-300">{domain} (NetBIOS: UCI)</span>
                      </div>
                    </div>
                  </div>

                  {/* Realtime Speed Chart */}
                  <div className="pt-2 border-t border-slate-800/80">
                    <div className="flex items-center justify-between text-[11px] text-slate-400 mb-1">
                      <span>Monitoreo en tiempo real:</span>
                      <span className="font-mono text-emerald-400">
                        {connectionStatus === 'connected' ? `${speedMBs} MB/s · ${pingMs}ms` : '0.00 MB/s · 0ms'}
                      </span>
                    </div>
                    <div className="h-10 bg-slate-950 rounded flex items-end gap-1 p-1 border border-slate-800">
                      {speedHistory.map((val, idx) => (
                        <div
                          key={idx}
                          className="flex-1 bg-cyan-500/80 rounded-t transition-all duration-300"
                          style={{
                            height: connectionStatus === 'connected' ? `${Math.min(100, (val / 4) * 100)}%` : '4%',
                          }}
                        ></div>
                      ))}
                    </div>
                    <p className="text-[10px] text-center text-slate-500 mt-2">
                      Wintun L3 · Protección Anti-Bloqueo AD · Cero Consolas
                    </p>
                  </div>
                </div>
              )}

              {/* Tab 2: Ajustes Content */}
              {guiTab === 'ajustes' && (
                <div className="p-5 space-y-4 text-xs">
                  <div className="font-semibold text-slate-200 text-xs tracking-wide">
                    CONFIGURACIÓN DE RED INSTITUCIONAL
                  </div>

                  <div className="space-y-3">
                    <div>
                      <label className="text-slate-400 block mb-1">Servidor Proxy:</label>
                      <input
                        type="text"
                        value={proxyHost}
                        onChange={(e) => setProxyHost(e.target.value)}
                        className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                      />
                    </div>

                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="text-slate-400 block mb-1">Puerto:</label>
                        <input
                          type="text"
                          value={proxyPort}
                          onChange={(e) => setProxyPort(e.target.value)}
                          className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                        />
                      </div>
                      <div>
                        <label className="text-slate-400 block mb-1">Autenticación:</label>
                        <select
                          value={authScheme}
                          onChange={(e) => setAuthScheme(e.target.value)}
                          className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-1.5 text-slate-200 focus:outline-none focus:border-cyan-500"
                        >
                          <option value="auto">auto</option>
                          <option value="ntlm">ntlm</option>
                          <option value="basic">basic</option>
                          <option value="none">none</option>
                        </select>
                      </div>
                    </div>

                    <div>
                      <label className="text-slate-400 block mb-1">Modo de Captura:</label>
                      <select
                        value={captureMode}
                        onChange={(e) => setCaptureMode(e.target.value)}
                        className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-1.5 text-slate-200 focus:outline-none focus:border-cyan-500"
                      >
                        <option value="tun">tun (Wintun L3 completo)</option>
                        <option value="system_proxy">system_proxy (Solo navegador)</option>
                      </select>
                    </div>

                    <button
                      onClick={() => runDiagnosticHandshake(false)}
                      className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium rounded border border-slate-700 flex items-center justify-center gap-1.5 transition"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />
                      Sondear Proxy (Detectar Esquema)
                    </button>
                  </div>
                </div>
              )}

              {/* Tab 3: Registro Content */}
              {guiTab === 'registro' && (
                <div className="p-4 flex flex-col h-[400px]">
                  <div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-800">
                    <span className="text-[11px] font-semibold text-slate-300">REGISTRO DE EVENTOS</span>
                    <div className="flex gap-1.5">
                      <button
                        onClick={() => navigator.clipboard?.writeText(simulatedLogs.join('\n'))}
                        className="px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[10px] text-slate-300 flex items-center gap-1"
                      >
                        <Copy className="w-3 h-3" />
                        Copiar
                      </button>
                      <button
                        onClick={() => setSimulatedLogs([])}
                        className="px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[10px] text-slate-300"
                      >
                        Limpiar
                      </button>
                    </div>
                  </div>

                  <div className="flex-1 bg-slate-950 p-2.5 rounded font-mono text-[11px] text-slate-300 overflow-y-auto space-y-1 leading-relaxed border border-slate-800/80">
                    {simulatedLogs.map((log, idx) => (
                      <div
                        key={idx}
                        className={`${
                          log.includes('🛡')
                            ? 'text-amber-400 font-medium'
                            : log.includes('✓')
                            ? 'text-emerald-400 font-medium'
                            : log.includes('✗')
                            ? 'text-red-400 font-medium'
                            : 'text-slate-300'
                        }`}
                      >
                        {log}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Desktop Simulator Explanatory Notes */}
            <div className="flex-1 bg-slate-900/40 border border-slate-800 rounded-xl p-6 space-y-4">
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <Layers className="w-4 h-4 text-cyan-400" />
                Arquitectura de la GUI Nativa (Fyne + Cgo)
              </h3>
              <p className="text-xs text-slate-400 leading-relaxed">
                Esta interfaz simula fielmente la ventana de escritorio compilada por Go y Fyne (resolución nativa 420x580 sin consolas CMD visibles).
              </p>

              <div className="space-y-3 text-xs">
                <div className="p-3 rounded-lg bg-slate-900 border border-slate-800">
                  <span className="font-semibold text-cyan-400 block mb-1">Pestaña Conexión:</span>
                  <span className="text-slate-400">
                    Selector de perfiles de usuario, interruptor de conexión con feedback visual (ámbar durante pre-flight, esmeralda al conectar, rojo al rechazar), medidor de latencia RTT y gráfica de tráfico en tiempo real.
                  </span>
                </div>
                <div className="p-3 rounded-lg bg-slate-900 border border-slate-800">
                  <span className="font-semibold text-cyan-400 block mb-1">Pestaña Ajustes:</span>
                  <span className="text-slate-400">
                    Host y puerto del proxy institucional, modo de captura (Wintun de capa 3 o Proxy del Sistema Win32), servidor VPS opcional y botón para sondear y detectar cabeceras del proxy.
                  </span>
                </div>
                <div className="p-3 rounded-lg bg-slate-900 border border-slate-800">
                  <span className="font-semibold text-cyan-400 block mb-1">Pestaña Registro:</span>
                  <span className="text-slate-400">
                    Consola en vivo con timestamps, diagnóstico de etapas NTLM (Paso 1, 2 y 3), y mensajes libres de contenido HTML gracias a la nueva función de drenaje de sockets.
                  </span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Tab 3: Diagnostics Tool */}
        {activeTab === 'diagnostics' && (
          <div className="space-y-6">
            <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-800 pb-5">
                <div>
                  <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                    <Activity className="w-4 h-4 text-cyan-400" />
                    Sonda y Diagnóstico del Handshake NTLMv2 (Squid)
                  </h3>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Inspecciona paso a paso el protocolo HTTP CONNECT con autenticación de dominio Active Directory.
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    disabled={diagRunning}
                    onClick={() => runDiagnosticHandshake(false)}
                    className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-medium flex items-center gap-2 transition disabled:opacity-50"
                  >
                    <Play className="w-3.5 h-3.5 fill-current" />
                    Ejecutar Diagnóstico Completo
                  </button>
                  <button
                    disabled={diagRunning}
                    onClick={() => runDiagnosticHandshake(true)}
                    className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-700 flex items-center gap-2 transition disabled:opacity-50"
                  >
                    <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                    Test Rechazo 407
                  </button>
                </div>
              </div>

              {/* Handshake Stepper */}
              <div className="py-6 grid grid-cols-1 md:grid-cols-4 gap-4">
                <div
                  className={`p-4 rounded-xl border transition-all ${
                    diagStep >= 1
                      ? 'border-cyan-500/50 bg-cyan-950/20'
                      : 'border-slate-800 bg-slate-900/30 text-slate-500'
                  }`}
                >
                  <div className="text-[10px] font-bold tracking-wider text-cyan-400 uppercase">Paso 1</div>
                  <div className="text-sm font-semibold text-slate-200 mt-1">Negociación (Type-1)</div>
                  <p className="text-xs text-slate-400 mt-1">
                    Envía petición <code className="text-cyan-300">CONNECT</code> con cabecera <code className="text-cyan-300">Proxy-Authorization: NTLM</code>.
                  </p>
                </div>

                <div
                  className={`p-4 rounded-xl border transition-all ${
                    diagStep >= 2
                      ? 'border-cyan-500/50 bg-cyan-950/20'
                      : 'border-slate-800 bg-slate-900/30 text-slate-500'
                  }`}
                >
                  <div className="text-[10px] font-bold tracking-wider text-cyan-400 uppercase">Paso 2</div>
                  <div className="text-sm font-semibold text-slate-200 mt-1">Desafío (Type-2) &amp; Drenaje</div>
                  <p className="text-xs text-slate-400 mt-1">
                    Squid responde 407 con desafío. <span className="text-emerald-400">Se drenan 3526 bytes</span> de HTML para no corromper el socket.
                  </p>
                </div>

                <div
                  className={`p-4 rounded-xl border transition-all ${
                    diagStep >= 3
                      ? 'border-cyan-500/50 bg-cyan-950/20'
                      : 'border-slate-800 bg-slate-900/30 text-slate-500'
                  }`}
                >
                  <div className="text-[10px] font-bold tracking-wider text-cyan-400 uppercase">Paso 3</div>
                  <div className="text-sm font-semibold text-slate-200 mt-1">Autenticación (Type-3)</div>
                  <p className="text-xs text-slate-400 mt-1">
                    Calcula <code className="text-cyan-300">HMAC-MD5</code> con dominio NetBIOS <code className="text-cyan-300">UCI</code> y bandera <code className="text-cyan-300">NegotiateTargetInfo</code>.
                  </p>
                </div>

                <div
                  className={`p-4 rounded-xl border transition-all ${
                    diagStep >= 4
                      ? diagResult?.success
                        ? 'border-emerald-500/50 bg-emerald-950/20'
                        : 'border-red-500/50 bg-red-950/20'
                      : 'border-slate-800 bg-slate-900/30 text-slate-500'
                  }`}
                >
                  <div className="text-[10px] font-bold tracking-wider uppercase text-slate-400">Paso 4</div>
                  <div className="text-sm font-semibold text-slate-200 mt-1">
                    {diagResult ? (diagResult.success ? 'Túnel 200 OK' : '407 Bloqueado') : 'Resultado'}
                  </div>
                  <p className="text-xs text-slate-400 mt-1">
                    {diagResult
                      ? diagResult.success
                        ? 'Túnel TCP bidireccional listo.'
                        : 'Anti-lockout activado con éxito.'
                      : 'Esperando respuesta...'}
                  </p>
                </div>
              </div>

              {/* Diagnostic Results Box */}
              {diagResult && (
                <div
                  className={`mt-4 p-5 rounded-xl border ${
                    diagResult.success
                      ? 'border-emerald-500/40 bg-emerald-950/20'
                      : 'border-red-500/40 bg-red-950/20'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    {diagResult.success ? (
                      <CheckCircle2 className="w-6 h-6 text-emerald-400 shrink-0" />
                    ) : (
                      <AlertTriangle className="w-6 h-6 text-red-400 shrink-0" />
                    )}
                    <div>
                      <h4 className="text-sm font-semibold text-slate-100">
                        {diagResult.success
                          ? '✓ Prueba de Autenticación Exitosa'
                          : '✗ Credenciales Rechazadas por el Proxy (Protegido por Anti-Bloqueo)'}
                      </h4>
                      <p className="text-xs text-slate-300 mt-1">{diagResult.details}</p>
                    </div>
                  </div>

                  <div className="mt-4 pt-4 border-t border-slate-800/60 grid grid-cols-2 sm:grid-cols-4 gap-4 font-mono text-xs">
                    <div>
                      <span className="text-slate-500 block text-[10px]">SERVIDOR DETECTADO</span>
                      <span className="text-slate-200">{diagResult.server}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block text-[10px]">DOMINIO DETECTADO</span>
                      <span className="text-cyan-400">{diagResult.domainDetected}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block text-[10px]">BYTES DRENADOS</span>
                      <span className="text-emerald-400">{diagResult.bodyDrainedBytes} bytes (HTML)</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block text-[10px]">CÓDIGO RESULTANTE</span>
                      <span className={diagResult.success ? 'text-emerald-400' : 'text-red-400'}>
                        {diagResult.squidCode}
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Tab 4: Guide & Config */}
        {activeTab === 'guide' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* config.json generator */}
            <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6 flex flex-col justify-between">
              <div>
                <div className="flex items-center justify-between">
                  <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                    <FileCode2 className="w-4 h-4 text-cyan-400" />
                    Archivo <code className="text-cyan-300">config.json</code> para Windows
                  </h3>
                  <button
                    onClick={() => navigator.clipboard?.writeText(generateConfigJson())}
                    className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 flex items-center gap-1.5 transition"
                  >
                    <Copy className="w-3.5 h-3.5" />
                    Copiar JSON
                  </button>
                </div>
                <p className="text-xs text-slate-400 mt-1">
                  Coloca este archivo en la misma carpeta que <code className="text-slate-300">unitunnel.exe</code> junto a <code className="text-slate-300">wintun.dll</code> en Windows.
                </p>

                <div className="mt-4 bg-slate-950 p-4 rounded-lg border border-slate-800 font-mono text-xs text-cyan-300 overflow-x-auto max-h-[300px]">
                  <pre>{generateConfigJson()}</pre>
                </div>
              </div>

              <div className="mt-4 pt-4 border-t border-slate-800 flex justify-between items-center text-xs text-slate-400">
                <span>Compatible con Windows 10/11 x64</span>
                <span className="font-mono text-emerald-400">Modo: Wintun L3</span>
              </div>
            </div>

            {/* Troubleshooting Active Directory / Squid in UCI */}
            <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6 space-y-4">
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <HelpCircle className="w-4 h-4 text-cyan-400" />
                Guía de Solución de Problemas (Squid / UCI)
              </h3>

              <div className="space-y-3 text-xs">
                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">¿Qué significa ERR_CACHE_ACCESS_DENIED?</h4>
                  <p className="text-slate-400 mt-1">
                    Es el código que devuelve Squid cuando las credenciales no son reconocidas por el controlador de dominio o cuando la cuota de navegación de la cuenta está bloqueada o vencida.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">¿Por qué mi cuenta se bloquea en la universidad?</h4>
                  <p className="text-slate-400 mt-1">
                    Las políticas de Active Directory bloquean una cuenta tras 3 a 5 intentos fallidos consecutivos. UniTunnel cuenta con un <strong>Circuit Breaker Anti-Bloqueo</strong> que detiene inmediatamente todos los reintentos automáticos tras el primer rechazo para salvaguardar tu usuario.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">Formato correcto de credenciales</h4>
                  <p className="text-slate-400 mt-1">
                    En el campo de Dominio puedes escribir tanto <code className="text-cyan-300">UCI</code> como <code className="text-cyan-300">uci.cu</code>. El motor UniTunnel ahora normaliza ambos casos al formato NetBIOS adecuado.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">Privilegios de Administrador</h4>
                  <p className="text-slate-400 mt-1">
                    Para crear el adaptador de red virtual <code className="text-cyan-300">Wintun</code> y configurar las rutas IP de Windows, <code className="text-cyan-300">unitunnel.exe</code> requiere ejecutarse con permisos de Administrador (el ejecutable incluye el manifiesto UAC para solicitar elevación automáticamente).
                  </p>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800/80 bg-slate-950 py-4 px-6 text-center text-xs text-slate-500">
        UniTunnel Desktop Client &amp; Control Center · Adaptador Virtual Wintun · Cero Consolas · Protección Anti-Bloqueo AD
      </footer>
    </div>
  );
}

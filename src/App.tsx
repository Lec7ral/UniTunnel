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
  Gauge,
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

  // Squid ACL 403 Audit state
  const [auditRunning, setAuditRunning] = useState(false);
  const [auditDone, setAuditDone] = useState(true);

  // Quota monitor state (cuota.uci.cu)
  const [quotaAllocatedMB, setQuotaAllocatedMB] = useState(500);
  const [quotaUsedMB, setQuotaUsedMB] = useState(368.4);
  const [quotaLoading, setQuotaLoading] = useState(false);
  const [quotaLastCheck, setQuotaLastCheck] = useState('Hace 2 min');
  const [quotaPortalUrl, setQuotaPortalUrl] = useState('https://cuota.uci.cu/');

  // Overdraft / Sobregiro calculations (permitted in UCI intranet)
  const isOverdraft = quotaUsedMB > quotaAllocatedMB;
  const overdraftMB = isOverdraft ? +(quotaUsedMB - quotaAllocatedMB).toFixed(1) : 0;
  const quotaRemainingMB = isOverdraft ? -overdraftMB : +(quotaAllocatedMB - quotaUsedMB).toFixed(1);
  const quotaPercentUsed = (quotaUsedMB / quotaAllocatedMB) * 100;
  // Strictly clamped between 0% and 100% so CSS width never overflows container or distorts flex layout
  const clampedProgressPercent = Math.min(100, Math.max(0, quotaPercentUsed));

  const refreshQuota = () => {
    setQuotaLoading(true);
    addLog(`[CUOTA] Consultando saldo en ${quotaPortalUrl} para '${username}'...`);
    setTimeout(() => {
      const simulatedUsed = +(quotaUsedMB + (Math.random() * 4 + 0.5)).toFixed(1);
      setQuotaUsedMB(simulatedUsed);
      setQuotaLoading(false);
      setQuotaLastCheck('Ahora mismo');
      if (simulatedUsed > quotaAllocatedMB) {
        const over = (simulatedUsed - quotaAllocatedMB).toFixed(1);
        const perc = ((simulatedUsed / quotaAllocatedMB) * 100).toFixed(0);
        addLog(`[CUOTA] ℹ Sobregiro detectado (+${over} MB sobre el plan de ${quotaAllocatedMB} MB, ${perc}% consumido - Estado: Sobregiro permitido).`);
      } else {
        const rem = (quotaAllocatedMB - simulatedUsed).toFixed(1);
        const perc = ((simulatedUsed / quotaAllocatedMB) * 100).toFixed(0);
        addLog(`[CUOTA] ✓ Saldo: Restante ${rem} MB de ${quotaAllocatedMB} MB (${perc}% consumido). Estado portal: 200 OK.`);
      }
    }, 850);
  };

  const runAclAudit = () => {
    setAuditRunning(true);
    setTimeout(() => {
      setAuditRunning(false);
      setAuditDone(true);
    }, 1000);
  };

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
        addLog(`✓ [DNS DoH] Servidor RFC 8484 activo en UDP 53. Resolviendo nombres de dominio vía HTTPS.`);
        addLog(`✓ [SISTEMA] Adaptador Wintun configurado con métrica 1. Todo el tráfico y DNS redirigidos.`);
        refreshQuota();
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
        local_http_port: 8081,
        quota_url: quotaPortalUrl,
        auto_check_quota: true,
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
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
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
                    Dominio: <code className="text-cyan-300 font-mono">UCI</code> · Usuario: <code className="text-cyan-300 font-mono">{username}</code>
                  </div>
                </div>
              </div>

              <div className={`bg-slate-900/60 border rounded-xl p-5 transition ${isOverdraft ? 'border-fuchsia-800/60 shadow-sm shadow-fuchsia-950/40' : 'border-slate-800 hover:border-slate-700'}`}>
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-slate-400">Cuota de Navegación</span>
                  <Gauge className={`w-4 h-4 ${isOverdraft ? 'text-fuchsia-400' : 'text-cyan-400'}`} />
                </div>
                <div className="mt-3">
                  <div className="text-xl font-bold text-slate-100 flex items-center justify-between">
                    <div className="flex items-center gap-1.5 min-w-0">
                      {isOverdraft ? (
                        <>
                          <span className="text-fuchsia-400 font-bold truncate">+{overdraftMB.toFixed(1)} MB</span>
                          <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-fuchsia-950 text-fuchsia-300 border border-fuchsia-800/80 shrink-0">
                            Sobregiro
                          </span>
                        </>
                      ) : (
                        <span>{quotaRemainingMB.toFixed(1)} MB</span>
                      )}
                    </div>
                    <span className="text-xs font-mono font-normal text-slate-400 shrink-0">/ {quotaAllocatedMB} MB</span>
                  </div>
                  <div className="mt-2 w-full max-w-full bg-slate-950 rounded-full h-2 overflow-hidden border border-slate-800 relative">
                    <div
                      className={`h-full rounded-full transition-all duration-300 ${
                        isOverdraft
                          ? 'bg-gradient-to-r from-purple-500 via-fuchsia-500 to-pink-500 shadow-sm shadow-fuchsia-500/50'
                          : quotaPercentUsed >= 90
                          ? 'bg-red-500'
                          : quotaPercentUsed >= 70
                          ? 'bg-amber-500'
                          : 'bg-cyan-500'
                      }`}
                      style={{ width: `${clampedProgressPercent}%` }}
                    ></div>
                  </div>
                  <div className="text-[11px] text-slate-400 mt-1 flex justify-between items-center">
                    <span className={isOverdraft ? 'text-fuchsia-300 font-medium' : ''}>
                      {quotaPercentUsed.toFixed(0)}% {isOverdraft ? '(Sobregiro permitido)' : 'consumido'}
                    </span>
                    <span className={`font-mono text-[10px] ${isOverdraft ? 'text-fuchsia-400' : 'text-cyan-400'}`}>
                      {isOverdraft ? `Exceso: +${overdraftMB.toFixed(1)} MB` : 'cuota.uci.cu'}
                    </span>
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
                    Circuit Breaker
                  </div>
                  <div className="text-xs text-slate-400 mt-1">
                    1 intento controlado. Cero loops de reconexión si hay error 407.
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

                  <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 flex gap-3">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" />
                    <div>
                      <h4 className="text-xs font-semibold text-slate-200">
                        Monitor de Cuota de Navegación UCI (cuota.uci.cu)
                      </h4>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Integración del parser de cuota inspirado en <code className="text-cyan-300">UCIProxy</code> (danielrodcaball). Consulta automáticamente el portal interno <code className="text-cyan-300">cuota.uci.cu</code> y muestra el saldo restante en MB para evitar desconexiones inesperadas por cuota agotada.
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

                  {/* Quota de Navegación UCI Section */}
                  <div className={`p-3 rounded-lg bg-slate-950/70 border space-y-2 transition ${isOverdraft ? 'border-fuchsia-900/60 shadow-sm shadow-fuchsia-950/30' : 'border-slate-800'}`}>
                    <div className="flex items-center justify-between text-xs">
                      <div className="flex items-center gap-1.5 font-semibold text-slate-300">
                        <Gauge className={`w-3.5 h-3.5 ${isOverdraft ? 'text-fuchsia-400' : 'text-cyan-400'}`} />
                        <span>CUOTA DE NAVEGACIÓN</span>
                      </div>
                      {isOverdraft ? (
                        <span className="text-[10px] px-1.5 py-0.5 rounded font-bold bg-fuchsia-950 text-fuchsia-300 border border-fuchsia-800/80">
                          Sobregiro (+{(quotaPercentUsed - 100).toFixed(0)}%)
                        </span>
                      ) : (
                        <span
                          className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${
                            quotaRemainingMB < 50
                              ? 'bg-red-950 text-red-400 border border-red-800/60'
                              : quotaRemainingMB < 150
                              ? 'bg-amber-950 text-amber-400 border border-amber-800/60'
                              : 'bg-emerald-950 text-emerald-400 border border-emerald-800/60'
                          }`}
                        >
                          {quotaRemainingMB < 50 ? 'Crítico' : quotaRemainingMB < 150 ? 'Advertencia' : 'Normal'}
                        </span>
                      )}
                    </div>

                    <div className="space-y-1">
                      <div className="w-full max-w-full bg-slate-900 rounded-full h-2.5 overflow-hidden border border-slate-800 relative">
                        <div
                          className={`h-full transition-all duration-500 rounded-full ${
                            isOverdraft
                              ? 'bg-gradient-to-r from-purple-500 via-fuchsia-500 to-pink-500 shadow-sm shadow-fuchsia-500/50'
                              : quotaPercentUsed >= 90
                              ? 'bg-red-500'
                              : quotaPercentUsed >= 70
                              ? 'bg-amber-500'
                              : 'bg-cyan-500'
                          }`}
                          style={{ width: `${clampedProgressPercent}%` }}
                        ></div>
                      </div>
                      <div className="flex justify-between items-center text-[10px] font-mono">
                        {isOverdraft ? (
                          <span className="text-fuchsia-300">
                            Sobregiro: <strong className="text-fuchsia-400">+{overdraftMB.toFixed(1)} MB</strong> (Permitido)
                          </span>
                        ) : (
                          <span className="text-slate-400">
                            Restante: <strong className="text-emerald-400">{quotaRemainingMB.toFixed(1)} MB</strong>
                          </span>
                        )}
                        <span className={isOverdraft ? 'text-fuchsia-400 font-bold' : 'text-slate-400'}>
                          {quotaPercentUsed.toFixed(0)}% de {quotaAllocatedMB} MB
                        </span>
                      </div>
                    </div>

                    <div className="flex items-center justify-between pt-1 border-t border-slate-900 text-[10px]">
                      <div className="flex items-center gap-1.5">
                        <span className="text-slate-500 font-mono">cuota.uci.cu</span>
                        {isOverdraft && (
                          <span className="text-[9px] px-1 rounded bg-fuchsia-950/80 text-fuchsia-400 border border-fuchsia-800/60">
                            Overdraft OK
                          </span>
                        )}
                      </div>
                      <button
                        onClick={refreshQuota}
                        disabled={quotaLoading}
                        className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-cyan-300 flex items-center gap-1 transition disabled:opacity-50"
                      >
                        <RefreshCw className={`w-3 h-3 ${quotaLoading ? 'animate-spin' : ''}`} />
                        {quotaLoading ? 'Consultando...' : 'Consultar Cuota'}
                      </button>
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

                    <div>
                      <label className="text-slate-400 block mb-1">Portal de Cuota (URL):</label>
                      <input
                        type="text"
                        value={quotaPortalUrl}
                        onChange={(e) => setQuotaPortalUrl(e.target.value)}
                        className="w-full bg-slate-950 border border-slate-800 rounded px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                        placeholder="https://cuota.uci.cu/"
                      />
                    </div>

                    <div className="pt-2 border-t border-slate-800/80 space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-slate-400">Plan de Cuota Base:</span>
                        <div className="flex gap-1.5">
                          {[500, 1024, 2048].map((plan) => (
                            <button
                              key={plan}
                              onClick={() => {
                                setQuotaAllocatedMB(plan);
                                if (quotaUsedMB > plan && !isOverdraft) setQuotaUsedMB(+(plan * 0.7).toFixed(1));
                              }}
                              className={`px-2 py-0.5 rounded text-[10px] font-mono transition ${
                                quotaAllocatedMB === plan
                                  ? 'bg-cyan-600 text-white font-bold'
                                  : 'bg-slate-800 text-slate-400 hover:text-slate-200'
                              }`}
                            >
                              {plan >= 1024 ? `${plan / 1024} GB` : `${plan} MB`}
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="flex items-center justify-between pt-1">
                        <span className="text-slate-400">Test de Sobregiro:</span>
                        <div className="flex gap-1.5">
                          <button
                            onClick={() => {
                              setQuotaAllocatedMB(500);
                              setQuotaUsedMB(540);
                              addLog('[TEST SOBREGIRO] Activado: 540 MB / 500 MB (108%). La barra se fija al 100% con gradiente de sobregiro y la UI no se distorsiona.');
                            }}
                            className={`px-2 py-1 rounded text-[10px] font-medium transition flex items-center gap-1 ${
                              isOverdraft
                                ? 'bg-fuchsia-600 text-white font-bold shadow-sm shadow-fuchsia-500/30'
                                : 'bg-slate-800 hover:bg-slate-700 text-fuchsia-300 border border-fuchsia-800/60'
                            }`}
                          >
                            <span>⚡ Simular Sobregiro (108%)</span>
                          </button>
                          <button
                            onClick={() => {
                              setQuotaAllocatedMB(500);
                              setQuotaUsedMB(350);
                              addLog('[TEST SOBREGIRO] Restaurado consumo normal: 350 MB / 500 MB (70%).');
                            }}
                            className="px-2 py-1 rounded text-[10px] bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                          >
                            Normal (70%)
                          </button>
                        </div>
                      </div>
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

            {/* Squid ACL 403 Restrictions Auditor Card */}
            <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6 space-y-6">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-slate-800 pb-5">
                <div>
                  <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                    <Shield className="w-4 h-4 text-amber-400" />
                    Auditor de Bloqueos y Políticas Squid (HTTP 403 Forbidden)
                  </h3>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Demostración práctica de por qué el proxy universitario rechaza las conexiones por IP o puertos no-SSL.
                  </p>
                </div>
                <button
                  disabled={auditRunning}
                  onClick={runAclAudit}
                  className="px-4 py-2 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-xs font-medium flex items-center gap-2 transition disabled:opacity-50"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${auditRunning ? 'animate-spin' : ''}`} />
                  Re-ejecutar Auditoría ACL
                </button>
              </div>

              {/* Audit Results Table */}
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs border border-slate-800 rounded-lg overflow-hidden">
                  <thead className="bg-slate-950 text-slate-400 font-semibold uppercase text-[10px] tracking-wider border-b border-slate-800">
                    <tr>
                      <th className="py-2.5 px-4">Destino de Prueba</th>
                      <th className="py-2.5 px-3">Tipo</th>
                      <th className="py-2.5 px-3">Puerto</th>
                      <th className="py-2.5 px-3">Respuesta Squid</th>
                      <th className="py-2.5 px-4">Regla ACL de la Universidad</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60 font-mono">
                    <tr className="bg-emerald-950/10 hover:bg-emerald-950/20 transition">
                      <td className="py-2.5 px-4 text-slate-200 font-semibold">www.google.com:443</td>
                      <td className="py-2.5 px-3 text-cyan-400">Dominio FQDN</td>
                      <td className="py-2.5 px-3 text-slate-300">443 (HTTPS)</td>
                      <td className="py-2.5 px-3 text-emerald-400 font-bold">200 OK ✓</td>
                      <td className="py-2.5 px-4 text-slate-400 font-sans text-xs">Permitido por política estándar de navegación.</td>
                    </tr>
                    <tr className="bg-red-950/10 hover:bg-red-950/20 transition">
                      <td className="py-2.5 px-4 text-slate-200 font-semibold">142.251.211.238:443</td>
                      <td className="py-2.5 px-3 text-amber-400">IP Numérica (Google)</td>
                      <td className="py-2.5 px-3 text-slate-300">443 (HTTPS)</td>
                      <td className="py-2.5 px-3 text-red-400 font-bold">403 Forbidden ✗</td>
                      <td className="py-2.5 px-4 text-slate-400 font-sans text-xs">
                        Regla <code className="text-amber-300">acl numeric_IPs</code>: Squid prohíbe conectar a IPs crudas para forzar filtrado de dominios.
                      </td>
                    </tr>
                    <tr className="bg-red-950/10 hover:bg-red-950/20 transition">
                      <td className="py-2.5 px-4 text-slate-200 font-semibold">149.154.167.92:443</td>
                      <td className="py-2.5 px-3 text-amber-400">IP Telegram (DC4)</td>
                      <td className="py-2.5 px-3 text-slate-300">443 (HTTPS)</td>
                      <td className="py-2.5 px-3 text-red-400 font-bold">403 Forbidden ✗</td>
                      <td className="py-2.5 px-4 text-slate-400 font-sans text-xs">
                        Bloqueo por IP numérica y lista negra perimetral de rangos Telegram en la universidad.
                      </td>
                    </tr>
                    <tr className="bg-red-950/10 hover:bg-red-950/20 transition">
                      <td className="py-2.5 px-4 text-slate-200 font-semibold">149.154.167.92:80</td>
                      <td className="py-2.5 px-3 text-amber-400">IP Telegram (DC4)</td>
                      <td className="py-2.5 px-3 text-slate-300">80 (HTTP)</td>
                      <td className="py-2.5 px-3 text-red-400 font-bold">403 Forbidden ✗</td>
                      <td className="py-2.5 px-4 text-slate-400 font-sans text-xs">
                        Regla <code className="text-red-300">http_access deny CONNECT !SSL_ports</code>: Squid solo permite CONNECT al puerto 443.
                      </td>
                    </tr>
                    <tr className="bg-red-950/10 hover:bg-red-950/20 transition">
                      <td className="py-2.5 px-4 text-slate-200 font-semibold">85.208.126.162:7551</td>
                      <td className="py-2.5 px-3 text-amber-400">IP MTProto Telegram</td>
                      <td className="py-2.5 px-3 text-slate-300">7551 (Custom)</td>
                      <td className="py-2.5 px-3 text-red-400 font-bold">403 Forbidden ✗</td>
                      <td className="py-2.5 px-4 text-slate-400 font-sans text-xs">
                        Puerto no convencional rechazado de inmediato por el cortafuegos.
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              {/* Solutions & Alternatives Grid */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
                <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2">
                  <div className="flex items-center gap-2 text-cyan-400 font-semibold text-xs">
                    <Zap className="w-4 h-4" />
                    1. Sniffer SNI TLS (Integrado)
                  </div>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    Para todas las conexiones en puerto 443, UniTunnel ahora inspecciona en memoria el paquete <code className="text-cyan-300">ClientHello</code> y extrae el nombre real del dominio (SNI). Así entrega a Squid <code className="text-emerald-400">google.com:443</code> en vez de la IP, sorteando el bloqueo 403.
                  </p>
                </div>

                <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2">
                  <div className="flex items-center gap-2 text-amber-400 font-semibold text-xs">
                    <Settings className="w-4 h-4" />
                    2. Telegram sin Modificaciones Complejas
                  </div>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    En Telegram Desktop: <strong>Ajustes &gt; Avanzado &gt; Tipo de conexión &gt; Usar proxy HTTP personalizado</strong>. Escribe <code className="text-amber-300">127.0.0.1</code> y puerto <code className="text-amber-300">8081</code> (sin usuario ni contraseña). UniTunnel se encarga del resto.
                  </p>
                </div>

                <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2">
                  <div className="flex items-center gap-2 text-emerald-400 font-semibold text-xs">
                    <Server className="w-4 h-4" />
                    3. Alternativa OpenVPN (Modo VPS)
                  </div>
                  <p className="text-xs text-slate-300 leading-relaxed">
                    <strong>¿Por qué OpenVPN salía con todo?</strong> Porque abría <em>un único túnel</em> hacia un VPS externo en el puerto 443. Si posees un VPS (o túnel SSH/VLESS), activa la casilla &quot;Habilitar VPS&quot; en Ajustes de UniTunnel para tener esa misma salida universal sin restricciones.
                  </p>
                </div>
              </div>
            </div>

            {/* UCI Quota Portal Scraper & Regex Parser Card */}
            <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-6 space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-800 pb-4">
                <div className="flex items-center gap-2.5">
                  <div className="p-2 rounded-lg bg-cyan-500/10 text-cyan-400">
                    <Gauge className="w-5 h-5" />
                  </div>
                  <div>
                    <h3 className="text-base font-semibold text-slate-100">
                      Sonda y Parser del Portal de Cuota (<code className="text-cyan-300">cuota.uci.cu</code>)
                    </h3>
                    <p className="text-xs text-slate-400">
                      Módulo <code className="text-cyan-300">pkg/quota</code> para consultar el saldo de datos sin abrir el navegador.
                    </p>
                  </div>
                </div>
                <button
                  onClick={refreshQuota}
                  disabled={quotaLoading}
                  className="px-3.5 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-medium flex items-center gap-1.5 transition disabled:opacity-50 self-start sm:self-center"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${quotaLoading ? 'animate-spin' : ''}`} />
                  {quotaLoading ? 'Consultando...' : 'Testear Consulta cuota.uci.cu'}
                </button>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className={`p-3.5 rounded-lg bg-slate-950 border space-y-2 transition ${isOverdraft ? 'border-fuchsia-800/60 shadow-sm shadow-fuchsia-950/30' : 'border-slate-800'}`}>
                  <div className="flex items-center justify-between text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                    <span>Resumen de Saldo Extraído</span>
                    {isOverdraft && (
                      <span className="px-1.5 py-0.2 rounded bg-fuchsia-950 text-fuchsia-300 border border-fuchsia-800/80 text-[9px] font-bold">
                        SOBREGIRO
                      </span>
                    )}
                  </div>
                  <div className="space-y-1.5 font-mono text-xs">
                    <div className="flex justify-between">
                      <span className="text-slate-400">Cuota Asignada:</span>
                      <span className="text-slate-200 font-semibold">{quotaAllocatedMB.toFixed(2)} MB</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-slate-400">Consumo Actual:</span>
                      <span className={isOverdraft ? 'text-fuchsia-400 font-bold' : 'text-amber-400'}>
                        {quotaUsedMB.toFixed(2)} MB
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-slate-400">Saldo / Sobregiro:</span>
                      {isOverdraft ? (
                        <span className="text-fuchsia-300 font-bold">+{overdraftMB.toFixed(2)} MB (Excedido)</span>
                      ) : (
                        <span className="text-emerald-400 font-bold">{quotaRemainingMB.toFixed(2)} MB</span>
                      )}
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-slate-400">Porcentaje:</span>
                      <span className={isOverdraft ? 'text-fuchsia-300 font-bold' : 'text-cyan-300'}>
                        {quotaPercentUsed.toFixed(1)}% {isOverdraft ? '(Sobregiro OK)' : ''}
                      </span>
                    </div>
                  </div>
                  <div className="pt-2 border-t border-slate-900">
                    <div className="text-[10px] text-slate-400 mb-1">Barra Segura (Clamped 0..100%):</div>
                    <div className="w-full max-w-full bg-slate-900 rounded-full h-2 overflow-hidden border border-slate-800">
                      <div
                        className={`h-full transition-all duration-300 rounded-full ${
                          isOverdraft
                            ? 'bg-gradient-to-r from-purple-500 via-fuchsia-500 to-pink-500'
                            : quotaPercentUsed >= 90
                            ? 'bg-red-500'
                            : quotaPercentUsed >= 70
                            ? 'bg-amber-500'
                            : 'bg-cyan-500'
                        }`}
                        style={{ width: `${clampedProgressPercent}%` }}
                      ></div>
                    </div>
                  </div>
                </div>

                <div className="md:col-span-2 p-3.5 rounded-lg bg-slate-950 border border-slate-800 space-y-2">
                  <div className="flex items-center justify-between text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                    <span>Fragmento HTML Recibido de cuota.uci.cu</span>
                    <span className="text-emerald-400 font-mono">HTTP 200 OK · Basic Auth</span>
                  </div>
                  <pre className="p-2.5 rounded bg-slate-900 border border-slate-800/80 font-mono text-[11px] text-cyan-300 overflow-x-auto leading-relaxed">
{`<div class="quota-container">
  <p><strong>Usuario:</strong> ${username}</p>
  <p><strong>Cuota total asignada:</strong> ${quotaAllocatedMB.toFixed(2)} MB</p>
  <p><strong>Consumo del periodo:</strong> ${quotaUsedMB.toFixed(2)} MB</p>
  <p><strong>Saldo disponible:</strong> ${isOverdraft ? `-${overdraftMB.toFixed(2)} MB (Sobregiro autorizado)` : `${quotaRemainingMB.toFixed(2)} MB`}</p>
</div>`}
                  </pre>
                  <p className="text-[11px] text-slate-400 leading-relaxed">
                    <strong>Blindaje contra distorsiones de interfaz:</strong> Si el usuario consume más del 100% de su saldo (sobregiro permitido en la red UCI), el cálculo de visualización se acota estrictamente a <code className="text-cyan-300">clampedProgressPercent (0..100%)</code> con <code className="text-cyan-300">overflow-hidden</code> para que el ancho CSS nunca exceda el contenedor ni deforme la cuadrícula.
                  </p>
                </div>
              </div>
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
                  <h4 className="font-semibold text-slate-200">¿Por qué funcionaba ping a 8.8.8.8 pero no a google.com?</h4>
                  <p className="text-slate-400 mt-1">
                    Hacer ping a una IP numérica (<code className="text-cyan-300">8.8.8.8</code>) comprueba que el enrutamiento IP y el adaptador L3 funcionan correctamente. Sin embargo, los nombres (<code className="text-cyan-300">google.com</code>) requieren consultas DNS (puerto UDP 53), que los cortafuegos universitarios bloquean y Squid no puede cursar (Squid solo soporta TCP CONNECT). UniTunnel ahora incorpora un <strong>servidor DNS DoH (RFC 8484)</strong> que intercepta UDP 53 y resuelve nombres vía HTTPS a través de Squid.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">¿Por qué el adaptador se identifica como &quot;WireGuard&quot; en Windows?</h4>
                  <p className="text-slate-400 mt-1">
                    El controlador <code className="text-cyan-300">Wintun</code> fue desarrollado por el autor de WireGuard como el driver TUN NDIS 6 moderno de mayor rendimiento para Windows (reemplazo del antiguo y lento <code className="text-slate-400">tap-windows6</code> de OpenVPN). <strong>Esto no cifra tu tráfico</strong> ni cambia el protocolo: solo actúa como una tarjeta de red virtual ultrarrápida.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">¿Por qué con OpenVPN salía todo y cómo lo hace UniTunnel sin VPS?</h4>
                  <p className="text-slate-400 mt-1">
                    OpenVPN se conectaba a un servidor externo (VPS) fuera de la isla, el cual recibía un túnel TCP 443 y adentro resolvía DNS y enrutaba todo. UniTunnel consigue una experiencia idéntica de <strong>Cero Configuración</strong> directamente contra Squid gracias a su trío automático: Adaptador Wintun L3 + Proxy WinINet de sistema + Servidor DNS DoH local.
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">Privilegios de Administrador</h4>
                  <p className="text-slate-400 mt-1">
                    Para crear el adaptador de red virtual <code className="text-cyan-300">Wintun</code> y configurar las rutas IP de Windows, <code className="text-cyan-300">unitunnel.exe</code> requiere ejecutarse con permisos de Administrador (el ejecutable incluye el manifiesto UAC para solicitar elevación automáticamente).
                  </p>
                </div>

                <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                  <h4 className="font-semibold text-slate-200">¿Cómo funciona la consulta de cuota y es legal usarla?</h4>
                  <p className="text-slate-400 mt-1">
                    Es 100% legal y seguro: UniTunnel se comunica con el portal interno <code className="text-cyan-300">https://cuota.uci.cu/</code> mediante HTTP Basic Auth utilizando tus credenciales institucionales (el mismo portal web que visitas desde la intranet). Extrae automáticamente tu saldo asignado, consumido y restante en MB para avisarte con un semáforo visual antes de que se agote la cuota y Squid empiece a bloquear conexiones.
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

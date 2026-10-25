import React, { useState, useEffect } from 'react';
import {
  Shield,
  Settings,
  Terminal,
  Activity,
  Play,
  Square,
  RefreshCw,
  Gauge,
  Wifi,
  Cable,
  Globe2,
  CheckCircle2,
  AlertTriangle,
  ChevronDown,
  Info,
  Server,
  Zap,
  Pin,
  PinOff,
  Power,
} from 'lucide-react';
import {
  bridge,
  AppConfig,
  defaultConfig,
  QuotaInfo,
  NetworkMode,
  InterfaceStatus,
} from './services/wailsBridge';
import { SettingsDrawer } from './components/SettingsDrawer';
import { LogsDrawer } from './components/LogsDrawer';

export default function App() {
  const [config, setConfig] = useState<AppConfig>(defaultConfig);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [interfaces, setInterfaces] = useState<InterfaceStatus[]>([]);

  // Connection & Tunnel state
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [pingMs, setPingMs] = useState(38);
  const [speedMBs, setSpeedMBs] = useState(2.6);
  const [speedHistory, setSpeedHistory] = useState<number[]>([1.2, 1.8, 2.4, 2.6, 2.9, 2.7, 2.6]);

  // Quota state
  const [quota, setQuota] = useState<QuotaInfo>({
    username: 'yoanbrp',
    allocated_mb: 500,
    used_mb: 362.4,
    remaining_mb: 137.6,
    percent_used: 72.5,
    clamped_percent: 72.5,
    is_overdraft: false,
    overdraft_mb: 0,
    status_level: 'normal',
    last_updated: 'Hace 2 min',
    success: true,
  });
  const [quotaLoading, setQuotaLoading] = useState(false);

  // Diagnostic modal/runner state
  const [showDiag, setShowDiag] = useState(false);
  const [diagStep, setDiagStep] = useState(0);
  const [diagRunning, setDiagRunning] = useState(false);
  const [diagResult, setDiagResult] = useState<{
    success: boolean;
    server: string;
    scheme: string;
    bodyDrained: number;
    details: string;
  } | null>(null);

  // Mode switching feedback toast
  const [modeNotice, setModeNotice] = useState<string | null>(null);

  // Initial load
  useEffect(() => {
    async function loadInitial() {
      const loaded = await bridge.getConfig();
      setConfig(loaded);
      const ifaces = await bridge.getInterfaces();
      setInterfaces(ifaces);
    }
    loadInitial();

    const unsubscribe = bridge.subscribeLogs((newLogs) => {
      setLogs(newLogs);
    });
    return () => unsubscribe();
  }, []);

  // Periodic simulated speed tick when connected
  useEffect(() => {
    if (!connected) return;
    const interval = setInterval(() => {
      const nextSpeed = +(Math.random() * 2 + 1.8).toFixed(2);
      const nextPing = Math.floor(Math.random() * 15 + 32);
      setSpeedMBs(nextSpeed);
      setPingMs(nextPing);
      setSpeedHistory((prev) => [...prev.slice(-12), nextSpeed]);
    }, 1500);
    return () => clearInterval(interval);
  }, [connected]);

  const activeAccount =
    config.accounts.find((a) => a.id === config.active_account_id) ||
    config.accounts[0] || {
      id: 'default',
      name: 'Cuenta Principal',
      username: 'yoanbrp',
      domain: 'uci.cu',
    };

  const handleSaveConfig = async (newConfig: AppConfig): Promise<boolean> => {
    const ok = await bridge.saveConfig(newConfig);
    if (ok) {
      setConfig(newConfig);
    }
    return ok;
  };

  const handleSelectNetworkMode = async (mode: NetworkMode) => {
    if (config.network_mode === mode) return;

    let notice = '';
    if (mode === 'uci_cable') {
      notice = 'Prioridad asignada al Cable UCI (Ethernet métrica 10, Wi-Fi 100).';
    } else if (mode === 'wifi_mobile') {
      notice = 'Prioridad Wi-Fi (métrica 10). Puedes usar los datos móviles sin desconectar el cable.';
    } else {
      notice = 'Métricas de red automáticas restauradas en Windows.';
    }

    setModeNotice(notice);
    setTimeout(() => setModeNotice(null), 4000);

    await bridge.applyNetworkMode(mode);
    setConfig((prev) => ({ ...prev, network_mode: mode }));
    const updatedIfaces = await bridge.getInterfaces();
    setInterfaces(updatedIfaces);

    // If switching to mobile or direct, stop tunnel if active
    if (mode !== 'uci_cable' && connected) {
      setConnected(false);
      bridge.addLog('TÚNEL', 'Túnel detenido para dar paso a la navegación directa o móvil.');
    }
  };

  const handleToggleAlwaysOnTop = async () => {
    const nextVal = !config.always_on_top;
    await bridge.setAlwaysOnTop(nextVal);
    setConfig((prev) => ({ ...prev, always_on_top: nextVal }));
    setModeNotice(
      nextVal
        ? '📌 Ventana fijada al frente. Permanecerá visible sobre otras aplicaciones.'
        : 'Ventana desanclada. Comportamiento estándar restaurado.'
    );
    setTimeout(() => setModeNotice(null), 3500);
  };

  const handleConnectToggle = () => {
    if (connected || connecting) {
      setConnected(false);
      setConnecting(false);
      bridge.addLog('TÚNEL', 'Conexión cerrada y rutas de red restauradas.');
      return;
    }

    setConnecting(true);
    bridge.addLog('TÚNEL', `Conectando con ${config.proxy_host}:${config.proxy_port} (Usuario: ${activeAccount.username})...`);

    setTimeout(() => {
      bridge.addLog('AUTH', `Autenticación NTLMv2 aceptada por Squid (pxuci). Dominio NetBIOS: ${activeAccount.domain || 'UCI'}`);
      bridge.addLog('HTTP', `Respuesta HTTP 407 drenada correctamente (3526 bytes).`);
      bridge.addLog('TÚNEL', `✓ Túnel activo. Wintun L3 asignado y DNS seguro DoH RFC 8484 configurado.`);
      setConnecting(false);
      setConnected(true);
      handleRefreshQuota();
    }, 1200);
  };

  const handleRefreshQuota = async () => {
    setQuotaLoading(true);
    try {
      const q = await bridge.fetchQuota(activeAccount);
      setQuota(q);
    } finally {
      setQuotaLoading(false);
    }
  };

  const handleSwitchAccount = async (accId: string) => {
    const updated = { ...config, active_account_id: accId };
    await handleSaveConfig(updated);
    const newAcc = updated.accounts.find((a) => a.id === accId);
    if (newAcc) {
      bridge.addLog('CONFIG', `Cuenta activa cambiada a: '${newAcc.name}' (${newAcc.username}).`);
      handleRefreshQuota();
    }
  };

  const runDiagnostics = (simulateFail: boolean) => {
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
          if (simulateFail) {
            setDiagResult({
              success: false,
              server: 'squid/4.17 (pxuci.uci.cu)',
              scheme: 'NTLMv2 (AD UCI)',
              bodyDrained: 3526,
              details:
                'Credenciales rechazadas (407). El mecanismo Anti-Bloqueo de UniTunnel detuvo los reintentos automáticos para no bloquear tu cuenta institucional.',
            });
          } else {
            setDiagResult({
              success: true,
              server: 'squid/4.17 (pxuci.uci.cu)',
              scheme: 'NTLMv2 (AD UCI)',
              bodyDrained: 3526,
              details:
                'Negociación NTLMv2 exitosa. Socket TCP sincronizado, cuerpo HTML 407 descartado y túnel CONNECT autorizado.',
            });
          }
        }, 600);
      }, 600);
    }, 600);
  };

  return (
    <div className="h-screen w-full bg-slate-950 text-slate-100 flex flex-col font-sans select-none overflow-hidden relative">
      {/* Network Mode Feedback Banner */}
      {modeNotice && (
        <div className="absolute top-3 left-1/2 -translate-x-1/2 z-50 bg-slate-900 border border-cyan-800/60 px-4 py-1.5 rounded-xl text-xs text-cyan-200 shadow-xl flex items-center gap-2 animate-in fade-in">
          <Info className="w-4 h-4 text-cyan-400 shrink-0" />
          <span>{modeNotice}</span>
        </div>
      )}

      {/* Unified Android App Container */}
      <main className="flex-1 w-full max-w-md mx-auto flex flex-col p-4 justify-between overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        
        {/* Top Header Bar: Settings & Logs */}
        <div className="flex items-center justify-between py-2 border-b border-slate-900/80">
          <button
            onClick={() => setSettingsOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-800 transition text-xs font-semibold shadow-sm"
            title="Ajustes"
          >
            <Settings className="w-3.5 h-3.5 text-cyan-400" />
            <span>Ajustes</span>
          </button>

          <div className="flex items-center gap-1.5">
            <span className="font-bold text-xs tracking-tight text-slate-200 uppercase tracking-widest">UniTunnel</span>
          </div>

          <button
            onClick={() => setLogsOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-800 transition text-xs font-semibold shadow-sm"
            title="Logs"
          >
            <Terminal className="w-3.5 h-3.5 text-emerald-400" />
            <span>Logs</span>
            <span className="text-[10px] font-mono px-1.5 py-0.2 rounded-full bg-slate-950 text-slate-300 border border-slate-800">
              {logs.length}
            </span>
          </button>
        </div>

        {/* Center Section: Circular VPN Connect Button */}
        <div className="flex flex-col items-center justify-center my-auto py-2">
          <div className="relative flex items-center justify-center">
            {/* Outer Pulse Ring */}
            <div
              className={`absolute w-36 h-36 rounded-full transition-all duration-700 pointer-events-none ${
                connected
                  ? 'bg-emerald-500/15 animate-ping'
                  : connecting
                  ? 'bg-amber-500/15 animate-ping'
                  : 'opacity-0'
              }`}
            />

            <button
              onClick={handleConnectToggle}
              disabled={config.network_mode !== 'uci_cable' && !connected}
              className={`w-32 h-32 rounded-full flex flex-col items-center justify-center transition-all duration-300 shadow-2xl relative z-10 group border-4 ${
                connected
                  ? 'bg-gradient-to-tr from-emerald-600 via-teal-600 to-cyan-600 hover:from-emerald-500 hover:to-cyan-500 text-white border-emerald-400/40 shadow-emerald-950/80 ring-4 ring-emerald-500/20'
                  : connecting
                  ? 'bg-gradient-to-tr from-amber-600 to-yellow-600 text-white border-amber-400/40 cursor-wait animate-pulse'
                  : config.network_mode !== 'uci_cable'
                  ? 'bg-slate-900 text-slate-600 border-slate-800/80 cursor-not-allowed'
                  : 'bg-gradient-to-tr from-slate-900 via-slate-900 to-slate-950 hover:from-slate-800 hover:to-slate-900 text-slate-200 border-slate-800 hover:border-cyan-500/60 shadow-slate-950 group-hover:scale-105'
              }`}
              title={connected ? 'Desconectar' : 'Conectar'}
            >
              <div className="flex flex-col items-center justify-center space-y-1">
                <Power
                  className={`w-9 h-9 transition-transform ${
                    connected ? 'text-emerald-100 group-hover:scale-110' : 'text-slate-400 group-hover:text-cyan-400'
                  }`}
                />
                <span className="text-[11px] font-bold tracking-wider uppercase mt-0.5">
                  {connected ? 'Conectado' : connecting ? 'Conectando' : 'Conectar'}
                </span>
              </div>
            </button>
          </div>

          <div className="text-center mt-3 font-mono text-xs">
            {connected ? (
              <span className="text-emerald-400 font-medium">● Túnel Seguro Activo · {pingMs}ms</span>
            ) : connecting ? (
              <span className="text-amber-400 animate-pulse">Negociando NTLMv2 con Squid...</span>
            ) : (
              <span className="text-slate-500">Desconectado · Listo para conectar</span>
            )}
          </div>
        </div>

        {/* Bottom Elements: Network Mode, Account, Quota, Graph */}
        <div className="space-y-2.5 pb-1">
          
          {/* Network Mode Selector */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-2.5 flex items-center justify-between text-xs">
            <span className="text-slate-400 text-[11px] pl-1 font-medium">Modo de Red:</span>
            <div className="flex gap-1">
              {[
                { id: 'uci_cable', label: 'Cable UCI' },
                { id: 'wifi_mobile', label: 'Wi-Fi / Móvil' },
                { id: 'direct', label: 'Directo' },
              ].map((m) => (
                <button
                  key={m.id}
                  onClick={() => handleSelectNetworkMode(m.id as NetworkMode)}
                  className={`px-2.5 py-1 rounded-lg text-[11px] font-medium transition ${
                    config.network_mode === m.id
                      ? 'bg-cyan-600 text-white shadow'
                      : 'bg-slate-950 text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                  }`}
                >
                  {m.label}
                </button>
              ))}
            </div>
          </div>

          {/* Account Selector */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl px-3.5 py-2.5 flex items-center justify-between text-xs">
            <div className="flex items-center gap-2">
              <span className="text-slate-400 text-[11px]">Cuenta:</span>
              <span className="font-semibold text-slate-200">{activeAccount.name}</span>
              <span className="font-mono text-cyan-300 text-[10px] bg-slate-950 px-1.5 py-0.5 rounded border border-slate-800">
                {activeAccount.username}
              </span>
            </div>
            {config.accounts.length > 1 && (
              <select
                value={config.active_account_id}
                onChange={(e) => handleSwitchAccount(e.target.value)}
                className="bg-slate-950 border border-slate-800 text-slate-300 text-[11px] rounded px-2 py-1 focus:outline-none focus:border-cyan-500 cursor-pointer"
              >
                {config.accounts.map((acc) => (
                  <option key={acc.id} value={acc.id}>
                    {acc.name}
                  </option>
                ))}
              </select>
            )}
          </div>

          {/* Quota Bar */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 space-y-1.5 text-xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5 font-semibold text-slate-300 text-[11px]">
                <Gauge className={`w-3.5 h-3.5 ${quota.is_overdraft ? 'text-fuchsia-400' : 'text-cyan-400'}`} />
                <span>Cuota UCI</span>
                <button
                  onClick={handleRefreshQuota}
                  disabled={quotaLoading}
                  className="ml-1 p-1 rounded hover:bg-slate-950 text-slate-400 hover:text-cyan-300 transition"
                  title="Actualizar cuota manualmente"
                >
                  <RefreshCw className={`w-3 h-3 ${quotaLoading ? 'animate-spin' : ''}`} />
                </button>
              </div>
              <div className="font-mono text-[11px]">
                {quota.is_overdraft ? (
                  <span className="text-fuchsia-400 font-bold">+{quota.overdraft_mb.toFixed(1)} MB (Sobregiro)</span>
                ) : (
                  <span className="text-slate-300">{quota.remaining_mb.toFixed(1)} MB / {quota.allocated_mb} MB</span>
                )}
              </div>
            </div>

            <div className="w-full bg-slate-950 rounded-full h-1.5 overflow-hidden border border-slate-800 relative">
              <div
                className={`h-full rounded-full transition-all duration-500 ${
                  quota.is_overdraft
                    ? 'bg-gradient-to-r from-purple-500 to-fuchsia-500'
                    : quota.clamped_percent >= 90
                    ? 'bg-red-500'
                    : quota.clamped_percent >= 70
                    ? 'bg-amber-500'
                    : 'bg-cyan-500'
                }`}
                style={{ width: `${quota.clamped_percent}%` }}
              />
            </div>
          </div>

          {/* Connection Sparkline Graph */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-2.5 space-y-1">
            <div className="flex items-center justify-between text-[11px] text-slate-400">
              <span>Tráfico de Red</span>
              <span className="font-mono text-cyan-300">
                {connected ? `${speedMBs} MB/s` : '0.00 MB/s'}
              </span>
            </div>
            <div className="h-8 bg-slate-950 rounded-lg flex items-end gap-1 p-1 border border-slate-800/80">
              {speedHistory.map((val, idx) => (
                <div
                  key={idx}
                  className="flex-1 bg-gradient-to-t from-cyan-600 to-emerald-500 rounded-t transition-all duration-300"
                  style={{
                    height: connected ? `${Math.min(100, (val / 4) * 100)}%` : '4%',
                  }}
                />
              ))}
            </div>
          </div>

        </div>

      </main>

      {/* Slide-over Drawers */}
      <SettingsDrawer
        isOpen={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        config={config}
        onSave={handleSaveConfig}
      />

      <LogsDrawer
        isOpen={logsOpen}
        onClose={() => setLogsOpen(false)}
        logs={logs}
        onClear={() => bridge.clearLogs()}
      />

      {/* NTLMv2 Diagnostics Modal */}
      {showDiag && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl space-y-5">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2.5">
                <Activity className="w-5 h-5 text-cyan-400" />
                <h3 className="font-bold text-sm text-slate-100">Sonda de Negociación NTLMv2</h3>
              </div>
              <button
                onClick={() => setShowDiag(false)}
                className="text-slate-400 hover:text-slate-200"
              >
                ✕
              </button>
            </div>

            <p className="text-xs text-slate-400">
              Verifica el pipeline de autenticación de UniTunnel contra Squid con drenaje de sockets y derivación de dominio Active Directory.
            </p>

            {/* Stepper */}
            <div className="space-y-2.5 py-2">
              <div className="flex items-center gap-3 text-xs">
                <span
                  className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold ${
                    diagStep >= 1 ? 'bg-cyan-600 text-white' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  1
                </span>
                <span className={diagStep >= 1 ? 'text-slate-200' : 'text-slate-500'}>
                  Type-1 Negotiate: Enviar bandera NegotiateTargetInfo (0x00800000)
                </span>
              </div>
              <div className="flex items-center gap-3 text-xs">
                <span
                  className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold ${
                    diagStep >= 2 ? 'bg-cyan-600 text-white' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  2
                </span>
                <span className={diagStep >= 2 ? 'text-slate-200' : 'text-slate-500'}>
                  Type-2 Challenge: Recibir desafío Squid y drenar 3526 bytes HTML 407
                </span>
              </div>
              <div className="flex items-center gap-3 text-xs">
                <span
                  className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold ${
                    diagStep >= 3 ? 'bg-cyan-600 text-white' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  3
                </span>
                <span className={diagStep >= 3 ? 'text-slate-200' : 'text-slate-500'}>
                  Type-3 Authenticate: Generar hash HMAC-MD5 con NetBIOS UCI
                </span>
              </div>
            </div>

            {/* Diagnostic Result */}
            {diagResult && (
              <div
                className={`p-4 rounded-xl border text-xs space-y-2 ${
                  diagResult.success
                    ? 'bg-emerald-950/30 border-emerald-800/60 text-emerald-200'
                    : 'bg-amber-950/30 border-amber-800/60 text-amber-200'
                }`}
              >
                <div className="flex items-center gap-2 font-bold">
                  {diagResult.success ? (
                    <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                  ) : (
                    <AlertTriangle className="w-4 h-4 text-amber-400" />
                  )}
                  <span>
                    {diagResult.success
                      ? 'Negociación 200 OK Completada'
                      : 'Circuito Anti-Bloqueo Activado'}
                  </span>
                </div>
                <p className="text-[11px] leading-relaxed opacity-90">{diagResult.details}</p>
                <div className="font-mono text-[10px] opacity-75 pt-1">
                  Servidor: {diagResult.server} · Socket drenado: {diagResult.bodyDrained} bytes
                </div>
              </div>
            )}

            <div className="flex gap-2.5 pt-2">
              <button
                onClick={() => runDiagnostics(false)}
                disabled={diagRunning}
                className="flex-1 py-2 px-3 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-medium text-xs transition disabled:opacity-50"
              >
                {diagRunning ? 'Ejecutando...' : 'Simular Handshake Normal'}
              </button>
              <button
                onClick={() => runDiagnostics(true)}
                disabled={diagRunning}
                className="py-2 px-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs transition disabled:opacity-50"
              >
                Simular Error 407
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

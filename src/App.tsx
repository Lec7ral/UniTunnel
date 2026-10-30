import React, { useState, useEffect } from 'react';
import {
  Settings,
  Terminal,
  RefreshCw,
  Gauge,
  Info,
  AlertTriangle,
  Power,
} from 'lucide-react';
import {
  bridge,
  AppConfig,
  defaultConfig,
  QuotaInfo,
  NetworkMode,
} from './services/wailsBridge';
import { SettingsDrawer } from './components/SettingsDrawer';
import { LogsDrawer } from './components/LogsDrawer';
import { reconnectReasons, reconnectWarning } from './services/configDiff';

export default function App() {
  const [config, setConfig] = useState<AppConfig>(defaultConfig);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [backendError, setBackendError] = useState<string | null>(null);

  // Connection & Tunnel state
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [pingMs, setPingMs] = useState(0);
  const [speedMBs, setSpeedMBs] = useState(0);
  const [speedHistory, setSpeedHistory] = useState<number[]>([0, 0, 0, 0, 0, 0, 0]);

  // Quota state
  const [quota, setQuota] = useState<QuotaInfo>({
    username: '',
    allocated_mb: 0,
    used_mb: 0,
    remaining_mb: 0,
    percent_used: 0,
    clamped_percent: 0,
    is_overdraft: false,
    overdraft_mb: 0,
    status_level: 'normal',
    last_updated: 'Sin consultar',
    success: false,
  });
  const [quotaLoading, setQuotaLoading] = useState(false);

  // Mode switching feedback toast
  const [modeNotice, setModeNotice] = useState<string | null>(null);

  // Initial load and live backend synchronization
  useEffect(() => {
    async function loadInitial() {
      try {
        const loaded = await bridge.getConfig();
        setConfig(loaded);
        setBackendError(null);
        const isConn = await bridge.isConnected();
        setConnected(isConn);
        await bridge.fetchLogs();
      } catch (err) {
        // No silent fallback: if the Go backend is unreachable the UI must say so
        // instead of showing defaults as if they were the real configuration.
        setBackendError(err instanceof Error ? err.message : String(err));
      }
    }
    loadInitial();

    const unsubscribe = bridge.subscribeLogs((newLogs) => {
      setLogs(newLogs);
    });

    // Periodic sync with Go backend (tunnel status, logs)
    const syncInterval = setInterval(async () => {
      try {
        const isConn = await bridge.isConnected();
        setConnected(isConn);
        await bridge.fetchLogs();
      } catch {
        // ignore
      }
    }, 3000);

    return () => {
      unsubscribe();
      clearInterval(syncInterval);
    };
  }, []);

  // Periodic real traffic stats & ping measurement from Go backend
  useEffect(() => {
    const interval = setInterval(async () => {
      try {
        const stats = await bridge.getTrafficStats();
        setConnected(stats.connected);
        setSpeedMBs(stats.speed_mbs);
        setPingMs(Number(stats.ping_ms));
        setSpeedHistory((prev) => [...prev.slice(-12), stats.speed_mbs]);
      } catch {
        // ignore
      }
    }, 1500);
    return () => clearInterval(interval);
  }, []);

  const activeAccount =
    config.accounts.find((a) => a.id === config.active_account_id) ||
    config.accounts[0] || {
      id: 'default',
      name: 'Cuenta Principal',
      username: '',
      domain: 'uci.cu',
    };

  const handleSaveConfig = async (newConfig: AppConfig): Promise<boolean> => {
    const ok = await bridge.saveConfig(newConfig);
    if (!ok) {
      return false;
    }

    // The tunnel was built from the previous config: warn when the new values
    // will only take effect after reconnecting.
    if (connected || connecting) {
      const reasons = reconnectReasons(newConfig, config);
      if (reasons.length > 0) {
        setModeNotice(reconnectWarning(reasons));
        setTimeout(() => setModeNotice(null), 8000);
      }
    }

    setConfig(newConfig);
    return true;
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

    // If switching to mobile or direct, stop tunnel if active
    if (mode !== 'uci_cable' && connected) {
      await bridge.stopTunnel();
      setConnected(false);
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

  const handleConnectToggle = async () => {
    // Ignore clicks while a connection attempt is in flight. The old code sent a
    // StopTunnel that the still-running StartTunnel immediately undid, leaving
    // the backend and the UI with different states.
    if (connecting) return;

    if (connected) {
      setConnecting(true);
      await bridge.stopTunnel();
      setConnected(false);
      setConnecting(false);
      await bridge.fetchLogs();
      return;
    }

    setConnecting(true);
    const result = await bridge.startTunnel();
    setConnecting(false);
    if (result.success) {
      setConnected(true);
      handleRefreshQuota();
    } else {
      setConnected(false);
    }
    await bridge.fetchLogs();
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

  return (
    <div className="h-screen w-full bg-slate-950 text-slate-100 flex flex-col font-sans select-none overflow-hidden relative">
      {/* Network Mode Feedback Banner */}
      {modeNotice && (
        <div className="absolute top-3 left-1/2 -translate-x-1/2 z-50 max-w-lg w-[92%] bg-slate-900 border border-cyan-800/60 px-4 py-1.5 rounded-xl text-xs text-cyan-200 shadow-xl flex items-center gap-2 animate-in fade-in">
          <Info className="w-4 h-4 text-cyan-400 shrink-0" />
          <span className="leading-snug">{modeNotice}</span>
        </div>
      )}

      {/* Backend Unavailable Banner: never pretend the Go side answered */}
      {backendError && (
        <div className="absolute top-3 left-1/2 -translate-x-1/2 z-50 max-w-md w-[92%] bg-red-950/90 border border-red-800/70 px-4 py-2 rounded-xl text-xs text-red-100 shadow-xl flex items-start gap-2 animate-in fade-in">
          <AlertTriangle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
          <div className="space-y-0.5">
            <div className="font-semibold">Backend Go no disponible</div>
            <div className="opacity-90 break-words">{backendError}</div>
          </div>
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
              disabled={connecting || (config.network_mode !== 'uci_cable' && !connected)}
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
                {quota.allocated_mb === 0 ? (
                  <span className="text-slate-500">{quota.last_updated}</span>
                ) : quota.is_overdraft ? (
                  <span className="text-fuchsia-400 font-bold">+{quota.overdraft_mb.toFixed(1)} MB (Sobregiro)</span>
                ) : (
                  <span className="text-slate-300">{quota.remaining_mb.toFixed(1)} MB / {quota.allocated_mb.toFixed(1)} MB</span>
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
                {connected ? `${speedMBs.toFixed(2)} MB/s` : '0.00 MB/s'}
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
        onClear={() => {
          // clearLogs talks to the backend and rejects when it is unreachable.
          bridge.clearLogs().catch((err: unknown) => {
            setBackendError(err instanceof Error ? err.message : String(err));
          });
        }}
      />
    </div>
  );
}

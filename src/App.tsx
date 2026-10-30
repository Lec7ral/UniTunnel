import React, { useState, useEffect, useRef } from 'react';
import {
  Settings,
  Terminal,
  RefreshCw,
  Gauge,
  Info,
  AlertTriangle,
  Power,
  User,
  ChevronDown,
  Check,
  Code,
  Minus,
  X,
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
import { QuotaInspectorModal } from './components/QuotaInspectorModal';
import { reconnectReasons, reconnectWarning } from './services/configDiff';

function formatSpeed(mbPerSec: number): string {
  if (mbPerSec <= 0) return '0.00 KB/s';
  const kb = mbPerSec * 1024;
  if (kb < 1) {
    return `${(kb * 1024).toFixed(0)} B/s`;
  }
  if (mbPerSec < 1) {
    return `${kb.toFixed(1)} KB/s`;
  }
  return `${mbPerSec.toFixed(2)} MB/s`;
}

export default function App() {
  const [config, setConfig] = useState<AppConfig>(defaultConfig);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsTab, setSettingsTab] = useState<'proxy' | 'accounts' | 'network' | 'system'>('proxy');
  const [accountDropdownOpen, setAccountDropdownOpen] = useState(false);
  const accountDropdownRef = useRef<HTMLDivElement>(null);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [backendError, setBackendError] = useState<string | null>(null);

  // Connection & Tunnel state
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [pingMs, setPingMs] = useState(0);
  const [speedMBs, setSpeedMBs] = useState(0);
  const [speedHistory, setSpeedHistory] = useState<number[]>(Array(16).fill(0));

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
  const [quotaInspectorOpen, setQuotaInspectorOpen] = useState(false);

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
        setSpeedHistory((prev) => [...prev.slice(-15), stats.speed_mbs]);
      } catch {
        // ignore
      }
    }, 1500);
    return () => clearInterval(interval);
  }, []);

  // Close account dropdown on outside click
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (accountDropdownRef.current && !accountDropdownRef.current.contains(event.target as Node)) {
        setAccountDropdownOpen(false);
      }
    }
    if (accountDropdownOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [accountDropdownOpen]);

  const openSettings = (tab: 'proxy' | 'accounts' | 'network' | 'system' = 'proxy') => {
    setSettingsTab(tab);
    setSettingsOpen(true);
  };

  const activeAccount =
    config.accounts.find((a) => a.id === config.active_account_id) ||
    config.accounts[0] || {
      id: 'default',
      name: 'Personal',
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

      {/* Frameless Custom TitleBar (Draggable) */}
      <header
        className="w-full bg-slate-950/95 border-b border-slate-900/90 px-3.5 py-2 flex items-center justify-between select-none z-50 shrink-0"
        style={{
          ['--wails-draggable' as string]: 'drag',
          WebkitAppRegion: 'drag',
        } as React.CSSProperties}
      >
        {/* Left: App Logo, Name and Info Tooltip */}
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse shrink-0" />
          <div className="flex items-center gap-1.5 relative group">
            <span className="font-bold text-xs tracking-widest text-slate-200 uppercase">
              UniTunnel
            </span>
            <div
              className="relative inline-flex items-center"
              style={{
                ['--wails-draggable' as string]: 'no-drag',
                WebkitAppRegion: 'no-drag',
              } as React.CSSProperties}
            >
              <button
                type="button"
                className="p-0.5 rounded-full text-slate-500 hover:text-cyan-400 transition"
                title="Acerca de UniTunnel"
              >
                <Info className="w-3.5 h-3.5" />
              </button>

              {/* Hover Popover Tooltip for App Info, Version & Developer */}
              <div className="absolute left-0 top-full mt-2 w-64 p-3 bg-slate-900/95 backdrop-blur-md border border-cyan-800/60 rounded-xl shadow-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none group-hover:pointer-events-auto z-50 text-[11px] space-y-2 text-slate-300">
                <div className="flex items-center justify-between border-b border-slate-800/80 pb-1.5 font-bold text-slate-100">
                  <span className="text-cyan-400 font-bold">UniTunnel Desktop</span>
                  <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-cyan-950 text-cyan-300 border border-cyan-800/60 font-semibold">
                    v2.4.0
                  </span>
                </div>
                <p className="text-[10px] text-slate-400 leading-snug">
                  Acceso Universitario & Túnel de Red Transparente para la UCI.
                </p>
                <div className="pt-1.5 border-t border-slate-800/80 space-y-1 text-[10px]">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Desarrollador:</span>
                    <span className="text-cyan-300 font-medium">Yoan B. · Red UCI</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Tecnologías:</span>
                    <span className="text-slate-300 font-mono">Go + Wintun + NTLMv2</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500">Plataforma:</span>
                    <span className="text-slate-300">Windows 10 / 11 (x64)</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Right: Custom Window Controls (Minimize & Close) */}
        <div
          className="flex items-center gap-1"
          style={{
            ['--wails-draggable' as string]: 'no-drag',
            WebkitAppRegion: 'no-drag',
          } as React.CSSProperties}
        >
          <button
            type="button"
            onClick={() => bridge.windowMinimize()}
            className="p-1 rounded-md text-slate-400 hover:text-slate-100 hover:bg-slate-800/80 transition"
            title="Minimizar a la barra de tareas"
          >
            <Minus className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            onClick={() => bridge.windowHide()}
            className="p-1 rounded-md text-slate-400 hover:text-red-300 hover:bg-red-950/60 transition"
            title="Ocultar a la bandeja del sistema (System Tray)"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </header>

      {/* Unified Android App Container */}
      <main className="flex-1 w-full max-w-md mx-auto flex flex-col p-4 justify-between overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        
        {/* Top Action Bar: Settings & Logs */}
        <div className="flex items-center justify-between py-2 border-b border-slate-900/80">
          <button
            onClick={() => openSettings('proxy')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-800 transition text-xs font-semibold shadow-sm"
            title="Ajustes"
          >
            <Settings className="w-3.5 h-3.5 text-cyan-400" />
            <span>Ajustes</span>
          </button>

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

          {/* Selector de cuenta simple */}
          <div ref={accountDropdownRef} className="relative">
            <button
              type="button"
              disabled={config.accounts.length <= 1}
              onClick={() => setAccountDropdownOpen(!accountDropdownOpen)}
              className={`w-full bg-slate-900/70 hover:bg-slate-900/90 border transition-all duration-200 rounded-xl p-2.5 flex items-center justify-between shadow-sm select-none text-left ${
                config.accounts.length > 1 ? 'cursor-pointer' : 'cursor-default'
              } ${
                accountDropdownOpen
                  ? 'border-cyan-500/70 ring-1 ring-cyan-500/30 bg-slate-900'
                  : 'border-slate-800/80 hover:border-slate-700'
              }`}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-cyan-950 to-slate-900 border border-cyan-800/60 flex items-center justify-center text-cyan-400 shrink-0 shadow-inner">
                  <User className="w-4 h-4" />
                </div>
                <div className="min-w-0 flex flex-col text-left flex-1 pr-2">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className="text-xs font-bold text-slate-100 truncate">
                      {activeAccount.name?.trim() || activeAccount.username?.trim() || 'Principal'}
                    </span>
                    <span className="text-[9px] px-1.5 py-0.2 rounded bg-emerald-950 text-emerald-400 border border-emerald-800/60 font-mono shrink-0">
                      Activa
                    </span>
                  </div>
                  <div className="font-mono text-slate-400 text-[11px] truncate">
                    {activeAccount.username ? (
                      <span>{activeAccount.username}<span className="text-slate-500">@{activeAccount.domain || 'uci.cu'}</span></span>
                    ) : (
                      <span className="text-slate-500 italic text-[10px]">Sin credenciales</span>
                    )}
                  </div>
                </div>
              </div>

              {config.accounts.length > 1 && (
                <div className="flex items-center gap-1 px-2 py-1 rounded-lg bg-slate-950/80 border border-slate-800 text-[11px] text-slate-300 shrink-0">
                  <span className="font-mono text-[10px] text-slate-400">{config.accounts.length}</span>
                  <ChevronDown className={`w-3.5 h-3.5 text-slate-400 transition-transform duration-200 ${accountDropdownOpen ? 'rotate-180 text-cyan-400' : ''}`} />
                </div>
              )}
            </button>

            {/* Dropdown Menu - exclusivamente para cambiar entre cuentas */}
            {accountDropdownOpen && config.accounts.length > 1 && (
              <div className="absolute left-0 right-0 bottom-full mb-2 z-50 bg-slate-900/95 backdrop-blur-md border border-cyan-500/40 rounded-xl shadow-2xl p-1.5 animate-in fade-in zoom-in-95">
                <div className="px-2 py-1 flex items-center justify-between border-b border-slate-800/80 mb-1">
                  <span className="text-[10px] uppercase tracking-wider font-semibold text-slate-400">Seleccionar Cuenta</span>
                  <span className="text-[10px] font-mono text-cyan-400">{config.accounts.length} disponibles</span>
                </div>
                
                <div className="space-y-1 max-h-48 overflow-y-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
                  {config.accounts.map((acc, idx) => {
                    const isCurrent = acc.id === config.active_account_id;
                    const displayName = acc.name?.trim() || acc.username?.trim() || `Cuenta ${idx + 1}`;
                    return (
                      <button
                        key={acc.id}
                        type="button"
                        onClick={() => {
                          handleSwitchAccount(acc.id);
                          setAccountDropdownOpen(false);
                        }}
                        className={`w-full text-left p-2 rounded-lg flex items-center justify-between transition-all ${
                          isCurrent
                            ? 'bg-cyan-950/60 border border-cyan-600/50 text-cyan-100 shadow-sm'
                            : 'bg-slate-950/60 hover:bg-slate-800/80 border border-slate-800/60 hover:border-slate-700 text-slate-300'
                        }`}
                      >
                        <div className="min-w-0 pr-2">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-xs text-slate-100 truncate">{displayName}</span>
                            {isCurrent && (
                              <span className="text-[9px] px-1 py-0.2 rounded bg-cyan-900 text-cyan-300 font-mono shrink-0">
                                Activa
                              </span>
                            )}
                          </div>
                          <div className="text-[11px] font-mono text-slate-400 truncate">
                            {acc.username ? `${acc.username}@${acc.domain || 'uci.cu'}` : 'Sin usuario configurado'}
                          </div>
                        </div>
                        {isCurrent && (
                          <Check className="w-4 h-4 text-cyan-400 shrink-0" />
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {/* Quota Bar */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-3 space-y-1.5 text-xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5 font-semibold text-slate-300 text-[11px]">
                <Gauge className={`w-3.5 h-3.5 ${quota.is_overdraft ? 'text-fuchsia-400' : 'text-cyan-400'}`} />
                <span>Cuota UCI</span>
                {quota.navigation_level && (
                  <span className="text-[9px] px-1.5 py-0.2 rounded bg-cyan-950/80 text-cyan-300 border border-cyan-800/60 font-mono">
                    {quota.navigation_level}
                  </span>
                )}
                <button
                  onClick={handleRefreshQuota}
                  disabled={quotaLoading}
                  className="ml-1 p-1 rounded hover:bg-slate-950 text-slate-400 hover:text-cyan-300 transition"
                  title="Actualizar cuota manualmente"
                >
                  <RefreshCw className={`w-3 h-3 ${quotaLoading ? 'animate-spin' : ''}`} />
                </button>
                <button
                  onClick={() => setQuotaInspectorOpen(true)}
                  className="p-1 rounded hover:bg-slate-950 text-slate-400 hover:text-cyan-300 transition"
                  title="Inspector de Cuota RAW (Diagnóstico y prueba manual)"
                >
                  <Code className="w-3 h-3 text-cyan-400" />
                </button>
              </div>
              <div className="font-mono text-[11px]">
                {quota.allocated_mb === 0 ? (
                  <span className="text-slate-500">{quota.last_updated}</span>
                ) : quota.is_overdraft ? (
                  <span className="text-fuchsia-400 font-bold">+{quota.overdraft_mb.toFixed(1)} MB (Sobregiro)</span>
                ) : (
                  <span className="text-slate-300">
                    {quota.remaining_mb.toFixed(1)} MB / {quota.allocated_mb.toFixed(0)} MB
                  </span>
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

          {/* Connection Sparkline Graph con Techo Dinámico */}
          {(() => {
            const peakRecentSpeed = Math.max(...speedHistory, 0);
            // Techo dinámico: piso de 0.04 MB/s (~40 KB/s) para que el tráfico bajo (< 1 MB)
            // ocupe barras altas y proporcionales; escala dinámicamente con margen al pico reciente
            const dynamicCeiling = Math.max(0.04, peakRecentSpeed * 1.25);

            return (
              <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-2.5 space-y-1.5">
                <div className="flex items-center justify-between text-[11px] text-slate-400">
                  <div className="flex items-center gap-1.5 font-medium">
                    <span className={`w-1.5 h-1.5 rounded-full transition-colors duration-300 ${connected ? 'bg-emerald-400 animate-pulse' : 'bg-slate-600'}`} />
                    <span>Tráfico de Red</span>
                    {connected && (
                      <span
                        className="text-[9px] font-mono px-1.5 py-0.2 rounded bg-slate-950 text-cyan-400 border border-slate-800"
                        title="Techo dinámico adaptativo según consumo reciente"
                      >
                        Techo: {formatSpeed(dynamicCeiling)}
                      </span>
                    )}
                  </div>
                  <span className="font-mono text-cyan-300 font-semibold transition-all duration-300">
                    {connected ? formatSpeed(speedMBs) : '0.00 KB/s'}
                  </span>
                </div>
                <div className="h-10 bg-slate-950/90 rounded-lg flex items-end gap-1 p-1.5 border border-slate-800/80 overflow-hidden relative">
                  {/* Reference faint baseline / middle line */}
                  <div className="absolute inset-x-0 top-1/2 border-b border-dashed border-slate-800/40 pointer-events-none" />
                  {speedHistory.map((val, idx) => {
                    const heightPct = connected && val > 0
                      ? Math.max(12, Math.min(100, (val / dynamicCeiling) * 100))
                      : (connected ? 6 : 4);
                    const isLatest = idx === speedHistory.length - 1;

                    return (
                      <div
                        key={idx}
                        title={`${formatSpeed(val)} (Escala máx: ${formatSpeed(dynamicCeiling)})`}
                        className={`flex-1 rounded-t relative group ${
                          connected
                            ? isLatest
                              ? 'bg-gradient-to-t from-cyan-500 via-emerald-400 to-emerald-300 shadow-[0_0_8px_rgba(52,211,153,0.35)]'
                              : 'bg-gradient-to-t from-cyan-600/80 to-emerald-500/90 hover:brightness-125'
                            : 'bg-slate-800/40'
                        }`}
                        style={{
                          height: `${heightPct}%`,
                          transition: 'height 450ms cubic-bezier(0.2, 0.8, 0.2, 1), background-color 300ms ease, box-shadow 300ms ease',
                        }}
                      />
                    );
                  })}
                </div>
              </div>
            );
          })()}

        </div>

      </main>

      {/* Slide-over Drawers */}
      <SettingsDrawer
        isOpen={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        config={config}
        onSave={handleSaveConfig}
        initialTab={settingsTab}
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

      <QuotaInspectorModal
        isOpen={quotaInspectorOpen}
        onClose={() => setQuotaInspectorOpen(false)}
        activeAccount={activeAccount}
        defaultPortalUrl={config.quota_url}
      />
    </div>
  );
}

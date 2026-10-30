import React, { useState } from 'react';
import {
  X,
  Save,
  RotateCcw,
  Plus,
  Trash2,
  Eye,
  EyeOff,
  Server,
  Users,
  Shield,
  ShieldCheck,
  UserCheck,
  Sliders,
  Globe,
  Gauge,
  CheckCircle2,
  AlertTriangle,
  HardDrive,
  Wifi,
  RefreshCw,
  Info,
  Tag,
} from 'lucide-react';
import { AppConfig, UserAccount, defaultConfig, bridge } from '../services/wailsBridge';
import { reconnectReasons } from '../services/configDiff';

interface SettingsDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  config: AppConfig;
  onSave: (newConfig: AppConfig) => Promise<boolean>;
  initialTab?: 'proxy' | 'accounts' | 'network' | 'system';
}

export const SettingsDrawer: React.FC<SettingsDrawerProps> = ({
  isOpen,
  onClose,
  config: initialConfig,
  onSave,
  initialTab,
}) => {
  const [cfg, setCfg] = useState<AppConfig>(initialConfig);
  const [activeTab, setActiveTab] = useState<'proxy' | 'accounts' | 'network' | 'system'>(initialTab || 'proxy');
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [showPasswordMap, setShowPasswordMap] = useState<Record<string, boolean>>({});
  const [newBypassInput, setNewBypassInput] = useState('');

  // Hotspot offline bypass & Wi-Fi Direct states
  const [forceHotspotActive, setForceHotspotActive] = useState(false);
  const [forceHotspotLoading, setForceHotspotLoading] = useState(false);
  const [forceHotspotNotice, setForceHotspotNotice] = useState<string | null>(null);
  const [diagLoading, setDiagLoading] = useState(false);
  const [diagResult, setDiagResult] = useState<string | null>(null);

  // Synchronize when opened
  React.useEffect(() => {
    if (isOpen) {
      if (initialTab) {
        setActiveTab(initialTab);
      }
      setCfg(initialConfig);
      setSaveSuccess(false);
      setSaveError(null);
      bridge.isHotspotForced().then(setForceHotspotActive).catch(() => {});
    }
  }, [isOpen, initialConfig, initialTab]);

  const handleToggleForceHotspot = async () => {
    setForceHotspotLoading(true);
    setForceHotspotNotice(null);
    try {
      const nextState = !forceHotspotActive;
      const res = await bridge.toggleForceOfflineHotspot(nextState);
      setForceHotspotActive(res.enabled);
      setForceHotspotNotice(res.message);
    } catch (err) {
      setForceHotspotNotice(`Error al alternar Hotspot: ${err}`);
    } finally {
      setForceHotspotLoading(false);
    }
  };

  const handleCheckHotspotDiag = async () => {
    setDiagLoading(true);
    try {
      const report = await bridge.checkHotspotCapability();
      setDiagResult(report);
    } catch (err) {
      setDiagResult(`Error en diagnóstico: ${err}`);
    } finally {
      setDiagLoading(false);
    }
  };

  if (!isOpen) return null;

  // Settings that only take effect on the next connection (see configDiff.ts).
  const pendingReconnect = reconnectReasons(cfg, initialConfig);

  const handleSave = async () => {
    setSaving(true);
    setSaveError(null);
    const success = await onSave(cfg);
    setSaving(false);
    if (success) {
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 2500);
    } else {
      // The bridge only returns false after a real backend failure; never show
      // "guardado" when nothing was written to config.json.
      setSaveError('El backend no confirmó el guardado. Revisa los logs.');
    }
  };

  const handleResetDefaults = () => {
    if (window.confirm('¿Deseas restablecer todos los valores de configuración a los predeterminados?')) {
      setCfg({ ...defaultConfig, accounts: cfg.accounts });
    }
  };

  const handleAddAccount = () => {
    const newId = `acc_${Date.now()}`;
    const nextNum = cfg.accounts.length + 1;
    const newAcc: UserAccount = {
      id: newId,
      name: `Cuenta ${nextNum}`,
      username: '',
      domain: 'uci.cu',
      password: '',
    };
    setCfg((prev) => ({
      ...prev,
      accounts: [...prev.accounts, newAcc],
      active_account_id: prev.accounts.length === 0 ? newId : prev.active_account_id,
    }));
  };

  const handleUpdateAccount = (id: string, updates: Partial<UserAccount>) => {
    setCfg((prev) => ({
      ...prev,
      accounts: prev.accounts.map((acc) => (acc.id === id ? { ...acc, ...updates } : acc)),
    }));
  };

  const handleDeleteAccount = (id: string) => {
    if (cfg.accounts.length <= 1) {
      alert('Debe existir al menos un perfil de cuenta.');
      return;
    }
    setCfg((prev) => {
      const filtered = prev.accounts.filter((a) => a.id !== id);
      const newActive = prev.active_account_id === id ? filtered[0].id : prev.active_account_id;
      return {
        ...prev,
        accounts: filtered,
        active_account_id: newActive,
      };
    });
  };

  const handleAddBypass = () => {
    const trimmed = newBypassInput.trim();
    if (!trimmed) return;
    if (cfg.bypass_routes.includes(trimmed)) return;
    setCfg((prev) => ({
      ...prev,
      bypass_routes: [...prev.bypass_routes, trimmed],
    }));
    setNewBypassInput('');
  };

  const handleRemoveBypass = (route: string) => {
    setCfg((prev) => ({
      ...prev,
      bypass_routes: prev.bypass_routes.filter((r) => r !== route),
    }));
  };

  const togglePasswordVisibility = (id: string) => {
    setShowPasswordMap((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div className="fixed inset-0 z-50 flex">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-slate-950/70 backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />

      {/* Slide-over panel from LEFT */}
      <div className="relative z-10 w-full max-w-xl bg-slate-900 border-r border-slate-800 shadow-2xl flex flex-col h-full animate-in slide-in-from-left duration-200">
        {/* Header */}
        <div className="p-5 border-b border-slate-800 flex items-center justify-between bg-slate-900/90">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-cyan-600/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
              <Sliders className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-slate-100">Ajustes de UniTunnel</h2>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tab Navigation: Compact 4-column grid with icon above and label below to prevent overflow */}
        <div className="grid grid-cols-4 gap-1 p-1 sm:p-1.5 border-b border-slate-800 bg-slate-950/70 select-none">
          <button
            type="button"
            onClick={() => setActiveTab('proxy')}
            title="Proxy & Cuota"
            className={`flex flex-col items-center justify-center py-2 px-1 rounded-lg transition-all min-w-0 ${
              activeTab === 'proxy'
                ? 'bg-slate-900 text-cyan-400 font-semibold border border-cyan-500/40 shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'
            }`}
          >
            <Server className="w-4 h-4 mb-1 shrink-0" />
            <span className="text-[10px] sm:text-[11px] leading-tight truncate w-full text-center">
              Proxy
            </span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('accounts')}
            title={`Cuentas (${cfg.accounts.length})`}
            className={`flex flex-col items-center justify-center py-2 px-1 rounded-lg transition-all min-w-0 ${
              activeTab === 'accounts'
                ? 'bg-slate-900 text-cyan-400 font-semibold border border-cyan-500/40 shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'
            }`}
          >
            <Users className="w-4 h-4 mb-1 shrink-0" />
            <span className="text-[10px] sm:text-[11px] leading-tight truncate w-full text-center">
              Cuentas ({cfg.accounts.length})
            </span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('network')}
            title="Wintun / L3"
            className={`flex flex-col items-center justify-center py-2 px-1 rounded-lg transition-all min-w-0 ${
              activeTab === 'network'
                ? 'bg-slate-900 text-cyan-400 font-semibold border border-cyan-500/40 shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'
            }`}
          >
            <Shield className="w-4 h-4 mb-1 shrink-0" />
            <span className="text-[10px] sm:text-[11px] leading-tight truncate w-full text-center">
              Wintun / L3
            </span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('system')}
            title="Windows & Hotspot"
            className={`flex flex-col items-center justify-center py-2 px-1 rounded-lg transition-all min-w-0 ${
              activeTab === 'system'
                ? 'bg-slate-900 text-cyan-400 font-semibold border border-cyan-500/40 shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/40 border border-transparent'
            }`}
          >
            <HardDrive className="w-4 h-4 mb-1 shrink-0" />
            <span className="text-[10px] sm:text-[11px] leading-tight truncate w-full text-center">
              Windows
            </span>
          </button>
        </div>

        {/* Scrollable Content */}
        <div className="flex-1 overflow-y-auto p-5 space-y-6 text-xs [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
          {/* TAB 1: PROXY & CUOTA */}
          {activeTab === 'proxy' && (
            <div className="space-y-4">
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                  <Server className="w-4 h-4 text-cyan-400" />
                  Servidor Proxy Institucional (Squid)
                </h3>

                <div className="grid grid-cols-3 gap-3">
                  <div className="col-span-2">
                    <label className="text-slate-400 block mb-1">Host / IP del Proxy:</label>
                    <input
                      type="text"
                      value={cfg.proxy_host}
                      onChange={(e) => setCfg({ ...cfg, proxy_host: e.target.value })}
                      placeholder="10.0.0.1"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">Puerto:</label>
                    <input
                      type="number"
                      value={cfg.proxy_port}
                      onChange={(e) => setCfg({ ...cfg, proxy_port: parseInt(e.target.value, 10) || 8080 })}
                      placeholder="8080"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-slate-400 block mb-1">Tipo de Autenticación:</label>
                  <select
                    value={cfg.auth_type}
                    onChange={(e) => setCfg({ ...cfg, auth_type: e.target.value })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-cyan-500"
                  >
                    <option value="auto">auto (Detección automática de cabeceras 407)</option>
                    <option value="ntlm">ntlm (NTLMv2 con Active Directory / NetBIOS UCI)</option>
                    <option value="basic">basic (Autenticación HTTP básica)</option>
                    <option value="none">none (Sin autenticación)</option>
                  </select>
                </div>
              </div>

              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                  <Gauge className="w-4 h-4 text-cyan-400" />
                  Portal de Consulta de Cuota (cuota.uci.cu)
                </h3>

                <div>
                  <label className="text-slate-400 block mb-1">URL del Portal de Cuota:</label>
                  <input
                    type="text"
                    value={cfg.quota_url}
                    onChange={(e) => setCfg({ ...cfg, quota_url: e.target.value })}
                    placeholder="https://cuota.uci.cu/"
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                </div>

                <div className="flex items-center justify-between pt-1">
                  <div>
                    <span className="font-medium text-slate-200 block">Consultar cuota automáticamente</span>
                    <span className="text-[11px] text-slate-400">Verifica el saldo al conectar y periódicamente</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={cfg.auto_check_quota}
                    onChange={(e) => setCfg({ ...cfg, auto_check_quota: e.target.checked })}
                    className="w-4 h-4 rounded text-cyan-600 focus:ring-0 bg-slate-900 border-slate-700"
                  />
                </div>

                {cfg.auto_check_quota && (
                  <div>
                    <label className="text-slate-400 block mb-1">Frecuencia de chequeo (minutos):</label>
                    <input
                      type="number"
                      value={cfg.quota_check_interval_min || 15}
                      onChange={(e) => setCfg({ ...cfg, quota_check_interval_min: parseInt(e.target.value, 10) || 15 })}
                      min="5"
                      max="120"
                      className="w-32 bg-slate-900 border border-slate-800 rounded-lg px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                )}
              </div>

              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                  <Globe className="w-4 h-4 text-cyan-400" />
                  Puertos Locales en 127.0.0.1
                </h3>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-slate-400 block mb-1">Puerto SOCKS5 Local:</label>
                    <input
                      type="number"
                      value={cfg.local_socks_port}
                      onChange={(e) => setCfg({ ...cfg, local_socks_port: parseInt(e.target.value, 10) || 1080 })}
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">Puerto HTTP Proxy Local:</label>
                    <input
                      type="number"
                      value={cfg.local_http_port}
                      onChange={(e) => setCfg({ ...cfg, local_http_port: parseInt(e.target.value, 10) || 8081 })}
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* TAB 2: ACCOUNTS */}
          {activeTab === 'accounts' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold text-slate-200">Perfiles de Cuenta y Credenciales</h3>
                  <p className="text-[11px] text-slate-400">Asigna etiquetas identificadoras (ej. Personal, Docencia) para alternar fácilmente</p>
                </div>
                <button
                  onClick={handleAddAccount}
                  className="px-2.5 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white font-medium flex items-center gap-1 transition shadow-sm"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Nueva Cuenta
                </button>
              </div>

              <div className="space-y-3">
                {cfg.accounts.map((acc, index) => {
                  const isActive = acc.id === cfg.active_account_id;
                  const showPass = !!showPasswordMap[acc.id];
                  const displayName = acc.name?.trim() || acc.username?.trim() || `Cuenta ${index + 1}`;
                  return (
                    <div
                      key={acc.id}
                      className={`p-3.5 rounded-xl border transition ${
                        isActive
                          ? 'bg-slate-950 border-cyan-500/60 shadow-sm shadow-cyan-950/80 ring-1 ring-cyan-500/20'
                          : 'bg-slate-950/40 border-slate-800 hover:border-slate-700'
                      }`}
                    >
                      {/* Account Card Header */}
                      <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800/80">
                        <div className="flex items-center gap-2">
                          <input
                            type="radio"
                            id={`account-radio-${acc.id}`}
                            name="active_account"
                            checked={isActive}
                            onChange={() => setCfg({ ...cfg, active_account_id: acc.id })}
                            className="text-cyan-500 focus:ring-0 w-3.5 h-3.5 cursor-pointer"
                          />
                          <label
                            htmlFor={`account-radio-${acc.id}`}
                            className="font-semibold text-slate-100 text-xs cursor-pointer flex items-center gap-1.5"
                          >
                            <span>{displayName}</span>
                            {isActive ? (
                              <span className="text-[10px] px-1.5 py-0.2 rounded bg-cyan-950 text-cyan-300 border border-cyan-800/60 font-semibold font-mono">
                                Activa
                              </span>
                            ) : (
                              <span className="text-[10px] text-slate-500 hover:text-cyan-400 font-normal transition">
                                (Hacer activa)
                              </span>
                            )}
                          </label>
                        </div>

                        {cfg.accounts.length > 1 && (
                          <button
                            type="button"
                            onClick={() => handleDeleteAccount(acc.id)}
                            className="text-slate-500 hover:text-red-400 p-1 rounded hover:bg-slate-900 transition"
                            title="Eliminar este perfil de cuenta"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>

                      {/* Explicit Account Name / Alias Field */}
                      <div className="mb-3">
                        <label className="text-[11px] text-slate-300 block mb-1 font-medium flex items-center justify-between">
                          <span className="flex items-center gap-1.5 text-cyan-400 font-semibold">
                            <Tag className="w-3.5 h-3.5" />
                            Nombre personalizado (Alias):
                          </span>
                          <span className="text-[10px] text-slate-400 font-normal">Se muestra en el selector principal</span>
                        </label>
                        <input
                          type="text"
                          value={acc.name || ''}
                          onChange={(e) => handleUpdateAccount(acc.id, { name: e.target.value })}
                          placeholder="Ej. Personal, UCI Facultad, Docente, Proyecto..."
                          className="w-full bg-slate-900 border border-slate-800 focus:border-cyan-500 rounded-lg px-2.5 py-1.5 text-slate-100 text-xs focus:outline-none transition placeholder:text-slate-600 font-medium"
                        />
                      </div>

                      {/* Credentials: User & Domain */}
                      <div className="grid grid-cols-2 gap-2.5 pt-1">
                        <div>
                          <label className="text-[11px] text-slate-400 block mb-1">Usuario:</label>
                          <input
                            type="text"
                            value={acc.username}
                            onChange={(e) => handleUpdateAccount(acc.id, { username: e.target.value })}
                            placeholder="ej. usuario"
                            className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                          />
                        </div>
                        <div>
                          <label className="text-[11px] text-slate-400 block mb-1">Dominio (NetBIOS):</label>
                          <input
                            type="text"
                            value={acc.domain}
                            onChange={(e) => handleUpdateAccount(acc.id, { domain: e.target.value })}
                            placeholder="uci.cu o UCI"
                            className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                          />
                        </div>
                      </div>

                      {/* Password */}
                      <div className="mt-2.5">
                        <label className="text-[11px] text-slate-400 block mb-1">Contraseña:</label>
                        <div className="relative">
                          <input
                            type={showPass ? 'text' : 'password'}
                            value={acc.password || ''}
                            onChange={(e) => handleUpdateAccount(acc.id, { password: e.target.value })}
                            placeholder="Contraseña institucional"
                            className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 pr-8 focus:outline-none focus:border-cyan-500"
                          />
                          <button
                            type="button"
                            onClick={() => togglePasswordVisibility(acc.id)}
                            className="absolute right-2.5 top-2 text-slate-400 hover:text-slate-200"
                            title={showPass ? 'Ocultar contraseña' : 'Ver contraseña'}
                          >
                            {showPass ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* TAB 3: NETWORK & WINTUN */}
          {activeTab === 'network' && (
            <div className="space-y-4">
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                  <Shield className="w-4 h-4 text-cyan-400" />
                  Adaptador Virtual Wintun (L3 VPN)
                </h3>

                <div>
                  <label className="text-slate-400 block mb-1">Modo de Captura:</label>
                  <select
                    value={cfg.capture_mode}
                    onChange={(e) => setCfg({ ...cfg, capture_mode: e.target.value as 'tun' | 'system_proxy' })}
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-cyan-500"
                  >
                    <option value="tun">tun (Adaptador Wintun L3 completo - Captura todo el sistema)</option>
                    <option value="system_proxy">system_proxy (Solo proxy del sistema - No requiere Wintun)</option>
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-slate-400 block mb-1">Nombre Adaptador:</label>
                    <input
                      type="text"
                      value={cfg.tun_name}
                      onChange={(e) => setCfg({ ...cfg, tun_name: e.target.value })}
                      placeholder="UniTunnel0"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">IP Virtual TUN IPv4:</label>
                    <input
                      type="text"
                      value={cfg.tun_ipv4}
                      onChange={(e) => setCfg({ ...cfg, tun_ipv4: e.target.value })}
                      placeholder="10.254.1.2"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-slate-400 block mb-1">Máscara Subred:</label>
                    <input
                      type="text"
                      value={cfg.tun_mask}
                      onChange={(e) => setCfg({ ...cfg, tun_mask: e.target.value })}
                      placeholder="255.255.255.0"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">DNS Upstream (DoH):</label>
                    <input
                      type="text"
                      value={cfg.tun_dns}
                      onChange={(e) => setCfg({ ...cfg, tun_dns: e.target.value })}
                      placeholder="1.1.1.1,8.8.8.8"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                </div>
              </div>

              {/* Bypass Routes */}
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-slate-200">Rutas y Dominios Excluidos (Bypass)</h3>
                  <span className="text-[11px] text-cyan-400 font-medium">Bypass directo a intranet</span>
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Los servicios del campus universitario (<span className="text-cyan-300 font-mono font-semibold">*uci.cu</span>) y redes privadas se excluyen obligatoriamente del proxy para evitar errores 403 y permitir acceso directo a Cuota, Moodle, Correo, etc.
                </p>

                <div className="flex gap-2">
                  <input
                    type="text"
                    value={newBypassInput}
                    onChange={(e) => setNewBypassInput(e.target.value)}
                    placeholder="ej. *uci.cu, 10.0.0.0/8 o 192.168.1.0/24"
                    className="flex-1 bg-slate-900 border border-slate-800 rounded-lg px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500 text-xs"
                  />
                  <button
                    onClick={handleAddBypass}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-cyan-300 font-medium transition text-xs"
                  >
                    Añadir
                  </button>
                </div>

                <div className="flex flex-wrap gap-1.5 pt-1">
                  {cfg.bypass_routes.map((route) => {
                    const isUci = route.includes('uci.cu');
                    return (
                      <span
                        key={route}
                        className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border font-mono text-[11px] ${
                          isUci
                            ? 'bg-cyan-950/70 border-cyan-600/50 text-cyan-200 font-semibold'
                            : 'bg-slate-900 border-slate-800 text-slate-300'
                        }`}
                      >
                        {isUci && <span className="w-1.5 h-1.5 rounded-full bg-cyan-400 shrink-0" />}
                        {route}
                        <button
                          onClick={() => handleRemoveBypass(route)}
                          className="text-slate-500 hover:text-red-400 ml-0.5"
                          title="Eliminar regla de bypass"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </span>
                    );
                  })}
                </div>
              </div>
            </div>
          )}

          {/* TAB 4: WINDOWS SYSTEM INTEGRATION */}
          {activeTab === 'system' && (
            <div className="space-y-4">
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                  <HardDrive className="w-4 h-4 text-cyan-400" />
                  Integración con el Sistema Windows
                </h3>

                <div className="space-y-3">
                  <div className="flex items-center justify-between p-2.5 rounded-lg bg-slate-900/60 border border-slate-800/80">
                    <div>
                      <span className="font-medium text-slate-200 block">Iniciar con Windows</span>
                      <span className="text-[11px] text-slate-400">Registra UniTunnel en HKCU\Software\Microsoft\Windows\CurrentVersion\Run</span>
                    </div>
                    <input
                      type="checkbox"
                      checked={cfg.auto_start}
                      onChange={(e) => setCfg({ ...cfg, auto_start: e.target.checked })}
                      className="w-4 h-4 rounded text-cyan-600 focus:ring-0 bg-slate-900 border-slate-700"
                    />
                  </div>

                  <div className="flex items-center justify-between p-2.5 rounded-lg bg-slate-900/60 border border-slate-800/80">
                    <div>
                      <span className="font-medium text-slate-200 block">Iniciar Minimizado</span>
                      <span className="text-[11px] text-slate-400">Oculta la ventana principal y permanece en la bandeja del sistema (System Tray)</span>
                    </div>
                    <input
                      type="checkbox"
                      checked={cfg.start_minimized}
                      onChange={(e) => setCfg({ ...cfg, start_minimized: e.target.checked })}
                      className="w-4 h-4 rounded text-cyan-600 focus:ring-0 bg-slate-900 border-slate-700"
                    />
                  </div>

                  <div className="flex items-center justify-between p-2.5 rounded-lg bg-slate-900/60 border border-slate-800/80">
                    <div>
                      <span className="font-medium text-slate-200 block">Fijar al Frente (Siempre Visible)</span>
                      <span className="text-[11px] text-slate-400">Mantiene la ventana visible sobre cualquier otra aplicación hasta que se minimice</span>
                    </div>
                    <input
                      type="checkbox"
                      checked={cfg.always_on_top}
                      onChange={(e) => setCfg({ ...cfg, always_on_top: e.target.checked })}
                      className="w-4 h-4 rounded text-cyan-600 focus:ring-0 bg-slate-900 border-slate-700"
                    />
                  </div>
                </div>
              </div>

              {/* Punto de acceso móvil sin internet (Wi-Fi Direct / SoftAP) */}
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-lg bg-cyan-950/80 border border-cyan-800/60 flex items-center justify-center text-cyan-400">
                      <Wifi className="w-4 h-4" />
                    </div>
                    <div>
                      <h4 className="font-semibold text-slate-200 text-sm">Punto de Acceso sin Internet</h4>
                      <p className="text-[11px] text-slate-400">
                        Wi-Fi Direct (SoftAP), sin exigir que Windows vea internet
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={handleToggleForceHotspot}
                    disabled={forceHotspotLoading}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none ${
                      forceHotspotActive ? 'bg-cyan-600' : 'bg-slate-700'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                        forceHotspotActive ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                <p className="text-[11px] text-slate-400 leading-relaxed">
                  Activa el punto de acceso móvil aunque Windows no detecte internet (red local o proxy institucional), que es lo que bloquea el interruptor de Configuración. Usa el <strong>SSID y la contraseña que ya tengas configurados</strong> en Windows y no modifica ningún ajuste de red.
                </p>

                {forceHotspotNotice && (
                  <div className="p-2.5 rounded-lg bg-cyan-950/40 border border-cyan-800/50 text-cyan-300 text-[11px] leading-relaxed whitespace-pre-line">
                    {forceHotspotNotice}
                  </div>
                )}

                <div className="pt-1 flex items-center justify-between">
                  <button
                    type="button"
                    onClick={handleCheckHotspotDiag}
                    disabled={diagLoading}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium transition"
                  >
                    <RefreshCw className={`w-3.5 h-3.5 ${diagLoading ? 'animate-spin' : ''}`} />
                    Verificar Wi-Fi Direct (SoftAP)
                  </button>
                  {forceHotspotActive && (
                    <span className="text-[11px] font-mono text-cyan-400 font-medium">Punto de acceso activo</span>
                  )}
                </div>

                {diagResult && (
                  <pre className="mt-2 p-2.5 rounded-lg bg-slate-900 border border-slate-800 text-[10px] text-slate-300 font-mono whitespace-pre-wrap max-h-40 overflow-y-auto">
                    {diagResult}
                  </pre>
                )}
              </div>

              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-2">
                <h4 className="font-semibold text-slate-300">Ubicación del Archivo de Configuración</h4>
                <p className="text-slate-400 font-mono text-[11px] bg-slate-900 p-2 rounded border border-slate-800">
                  config.json (junto a unitunnel.exe)
                </p>
                <p className="text-[11px] text-slate-500">
                  Cualquier cambio guardado desde este panel se sincroniza de inmediato con el archivo JSON y con el backend de Go.
                </p>
              </div>

              {/* Acerca de UniTunnel / Información del Sistema */}
              <div className="p-4 rounded-xl bg-gradient-to-b from-slate-950/90 to-slate-900/70 border border-cyan-900/40 shadow-lg shadow-black/40 space-y-3.5">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-600 via-teal-600 to-emerald-600 flex items-center justify-center text-white shadow-lg shadow-cyan-950/80 shrink-0 border border-cyan-400/30">
                    <ShieldCheck className="w-5 h-5 text-white" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <h4 className="font-bold text-slate-100 text-sm tracking-tight">UniTunnel Desktop</h4>
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-cyan-950 text-cyan-300 border border-cyan-800/60 font-semibold">
                        v2.4.0
                      </span>
                    </div>
                    <p className="text-[11px] text-slate-400 truncate">
                      Acceso Universitario & Túnel de Red Transparente
                    </p>
                  </div>
                </div>

                <p className="text-[11px] text-slate-300 leading-relaxed">
                  Diseñado para la comunidad de la <strong>Universidad de las Ciencias Informáticas (UCI)</strong>. Proporciona enrutamiento de capa 3 con adaptador Wintun nativo, autenticación NTLMv2 automática en servidores Squid y bypass de Wi-Fi Direct.
                </p>

                <div className="grid grid-cols-2 gap-2 text-[10px] font-mono pt-1">
                  <div className="p-2 rounded-lg bg-slate-900/90 border border-slate-800 space-y-0.5">
                    <span className="text-slate-500 uppercase tracking-wider block text-[9px] font-sans font-semibold">Plataforma</span>
                    <span className="text-slate-200">Windows 10 / 11 (x64)</span>
                  </div>
                  <div className="p-2 rounded-lg bg-slate-900/90 border border-slate-800 space-y-0.5">
                    <span className="text-slate-500 uppercase tracking-wider block text-[9px] font-sans font-semibold">Desarrollo</span>
                    <span className="text-cyan-400 font-sans font-medium">Yoan B. · Red UCI</span>
                  </div>
                  <div className="p-2 rounded-lg bg-slate-900/90 border border-slate-800 space-y-0.5">
                    <span className="text-slate-500 uppercase tracking-wider block text-[9px] font-sans font-semibold">Core & Runtime</span>
                    <span className="text-slate-200">Go 1.23 · Wails v3 Alpha</span>
                  </div>
                  <div className="p-2 rounded-lg bg-slate-900/90 border border-slate-800 space-y-0.5">
                    <span className="text-slate-500 uppercase tracking-wider block text-[9px] font-sans font-semibold">Driver L3 Virtual</span>
                    <span className="text-emerald-400">Wintun 0.14.1 TUN</span>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-1.5 pt-1 text-[10px] text-slate-400">
                  <span className="px-2 py-0.5 rounded-md bg-slate-900 border border-slate-800/90 text-slate-300">Squid NTLMv2</span>
                  <span className="px-2 py-0.5 rounded-md bg-slate-900 border border-slate-800/90 text-slate-300">SOCKS5 & HTTP</span>
                  <span className="px-2 py-0.5 rounded-md bg-slate-900 border border-slate-800/90 text-slate-300">SoftAP Wi-Fi Direct</span>
                  <span className="px-2 py-0.5 rounded-md bg-slate-900 border border-slate-800/90 text-emerald-400">Software Libre Académico</span>
                </div>
              </div>
            </div>
          )}
        </div>

        {pendingReconnect.length > 0 && (
          <div className="px-4 py-2.5 border-t border-amber-500/30 bg-amber-500/10 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <p className="text-[11px] text-amber-200/90 leading-relaxed">
              <span className="font-semibold">{pendingReconnect.join(', ')}</span>
              {' '}— estos ajustes solo se aplican al reconectar el túnel.
            </p>
          </div>
        )}

        {/* Footer Actions */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/90 flex items-center justify-between">
          <button
            onClick={handleResetDefaults}
            className="px-3 py-2 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-900 text-xs flex items-center gap-1.5 transition"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            Restablecer
          </button>

          <div className="flex items-center gap-2">
            {saveError && (
              <span className="text-red-400 text-xs flex items-center gap-1 font-medium animate-in fade-in max-w-[220px]">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                <span className="truncate" title={saveError}>No se guardó</span>
              </span>
            )}
            {saveSuccess && (
              <span className="text-emerald-400 text-xs flex items-center gap-1 font-medium animate-in fade-in">
                <CheckCircle2 className="w-4 h-4" />
                ¡Guardado!
              </span>
            )}
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 rounded-lg bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white font-semibold text-xs shadow-lg shadow-cyan-600/20 flex items-center gap-1.5 transition disabled:opacity-50"
            >
              <Save className="w-3.5 h-3.5" />
              {saving ? 'Guardando...' : 'Guardar'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

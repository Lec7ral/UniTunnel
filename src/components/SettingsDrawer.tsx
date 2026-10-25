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
  Sliders,
  Globe,
  Gauge,
  CheckCircle2,
  HardDrive,
  Wifi,
  RefreshCw,
  AlertTriangle,
} from 'lucide-react';
import { AppConfig, UserAccount, defaultConfig, bridge } from '../services/wailsBridge';

interface SettingsDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  config: AppConfig;
  onSave: (newConfig: AppConfig) => Promise<boolean>;
}

export const SettingsDrawer: React.FC<SettingsDrawerProps> = ({
  isOpen,
  onClose,
  config: initialConfig,
  onSave,
}) => {
  const [cfg, setCfg] = useState<AppConfig>(initialConfig);
  const [activeTab, setActiveTab] = useState<'proxy' | 'accounts' | 'network' | 'system' | 'vps'>('proxy');
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [showPasswordMap, setShowPasswordMap] = useState<Record<string, boolean>>({});
  const [newBypassInput, setNewBypassInput] = useState('');

  // Force Offline Hotspot state & rollback logic
  const [forceHotspotActive, setForceHotspotActive] = useState(false);
  const [forceHotspotLoading, setForceHotspotLoading] = useState(false);
  const [forceHotspotNotice, setForceHotspotNotice] = useState<string | null>(null);

  const handleToggleForceHotspot = async (enable: boolean) => {
    setForceHotspotLoading(true);
    setForceHotspotNotice(null);
    try {
      const result = await bridge.toggleForceOfflineHotspot(enable);
      setForceHotspotActive(result.enabled);
      setForceHotspotNotice(result.message);
    } catch {
      setForceHotspotActive(false);
      setForceHotspotNotice('Fallo ejecutando comandos netsh. Se aplicó la reversión automática.');
    } finally {
      setForceHotspotLoading(false);
    }
  };

  // Synchronize when opened
  React.useEffect(() => {
    if (isOpen) {
      setCfg(initialConfig);
      setSaveSuccess(false);
    }
  }, [isOpen, initialConfig]);

  if (!isOpen) return null;

  const handleSave = async () => {
    setSaving(true);
    const success = await onSave(cfg);
    setSaving(false);
    if (success) {
      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 2500);
    }
  };

  const handleResetDefaults = () => {
    if (window.confirm('¿Deseas restablecer todos los valores de configuración a los predeterminados?')) {
      setCfg({ ...defaultConfig, accounts: cfg.accounts });
    }
  };

  const handleAddAccount = () => {
    const newId = `acc_${Date.now()}`;
    const newAcc: UserAccount = {
      id: newId,
      name: `Cuenta ${cfg.accounts.length + 1}`,
      username: '',
      domain: 'uci.cu',
      password: '',
    };
    setCfg((prev) => ({
      ...prev,
      accounts: [...prev.accounts, newAcc],
      active_account_id: prev.active_account_id || newId,
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

        {/* Tab Navigation */}
        <div className="flex border-b border-slate-800 bg-slate-950/50 px-3 pt-2 gap-1 overflow-x-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
          <button
            onClick={() => setActiveTab('proxy')}
            className={`px-3 py-2 text-xs font-medium rounded-t-lg transition flex items-center gap-1.5 whitespace-nowrap border-b-2 ${
              activeTab === 'proxy'
                ? 'border-cyan-500 text-cyan-400 bg-slate-900'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Server className="w-3.5 h-3.5" />
            Proxy &amp; Cuota
          </button>
          <button
            onClick={() => setActiveTab('accounts')}
            className={`px-3 py-2 text-xs font-medium rounded-t-lg transition flex items-center gap-1.5 whitespace-nowrap border-b-2 ${
              activeTab === 'accounts'
                ? 'border-cyan-500 text-cyan-400 bg-slate-900'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Users className="w-3.5 h-3.5" />
            Cuentas ({cfg.accounts.length})
          </button>
          <button
            onClick={() => setActiveTab('network')}
            className={`px-3 py-2 text-xs font-medium rounded-t-lg transition flex items-center gap-1.5 whitespace-nowrap border-b-2 ${
              activeTab === 'network'
                ? 'border-cyan-500 text-cyan-400 bg-slate-900'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Shield className="w-3.5 h-3.5" />
            Wintun / L3
          </button>
          <button
            onClick={() => setActiveTab('system')}
            className={`px-3 py-2 text-xs font-medium rounded-t-lg transition flex items-center gap-1.5 whitespace-nowrap border-b-2 ${
              activeTab === 'system'
                ? 'border-cyan-500 text-cyan-400 bg-slate-900'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <HardDrive className="w-3.5 h-3.5" />
            Windows
          </button>
          <button
            onClick={() => setActiveTab('vps')}
            className={`px-3 py-2 text-xs font-medium rounded-t-lg transition flex items-center gap-1.5 whitespace-nowrap border-b-2 ${
              activeTab === 'vps'
                ? 'border-cyan-500 text-cyan-400 bg-slate-900'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Globe className="w-3.5 h-3.5" />
            VPS Opcional
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
                  <h3 className="font-semibold text-slate-200">Perfiles de Cuenta UCI</h3>
                  <p className="text-[11px] text-slate-400">Permite alternar rápidamente entre cuentas sin reescribir contraseñas</p>
                </div>
                <button
                  onClick={handleAddAccount}
                  className="px-2.5 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white font-medium flex items-center gap-1 transition"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Nueva Cuenta
                </button>
              </div>

              <div className="space-y-3">
                {cfg.accounts.map((acc, index) => {
                  const isActive = acc.id === cfg.active_account_id;
                  const showPass = !!showPasswordMap[acc.id];
                  return (
                    <div
                      key={acc.id}
                      className={`p-3.5 rounded-xl border transition ${
                        isActive
                          ? 'bg-slate-950 border-cyan-500/60 shadow-sm shadow-cyan-950'
                          : 'bg-slate-950/40 border-slate-800'
                      }`}
                    >
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <input
                            type="radio"
                            name="active_account"
                            checked={isActive}
                            onChange={() => setCfg({ ...cfg, active_account_id: acc.id })}
                            className="text-cyan-500 focus:ring-0 w-3.5 h-3.5"
                          />
                          <input
                            type="text"
                            value={acc.name}
                            onChange={(e) => handleUpdateAccount(acc.id, { name: e.target.value })}
                            placeholder={`Cuenta ${index + 1}`}
                            className="bg-transparent font-semibold text-slate-100 text-xs focus:outline-none border-b border-transparent focus:border-cyan-500"
                          />
                          {isActive && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800/60 font-semibold">
                              Activa
                            </span>
                          )}
                        </div>

                        {cfg.accounts.length > 1 && (
                          <button
                            onClick={() => handleDeleteAccount(acc.id)}
                            className="text-slate-500 hover:text-red-400 p-1 transition"
                            title="Eliminar cuenta"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>

                      <div className="grid grid-cols-2 gap-2.5 pt-1">
                        <div>
                          <label className="text-[11px] text-slate-400 block mb-0.5">Usuario:</label>
                          <input
                            type="text"
                            value={acc.username}
                            onChange={(e) => handleUpdateAccount(acc.id, { username: e.target.value })}
                            placeholder="ej. yoanbrp"
                            className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                          />
                        </div>
                        <div>
                          <label className="text-[11px] text-slate-400 block mb-0.5">Dominio (NetBIOS):</label>
                          <input
                            type="text"
                            value={acc.domain}
                            onChange={(e) => handleUpdateAccount(acc.id, { domain: e.target.value })}
                            placeholder="uci.cu o UCI"
                            className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                          />
                        </div>
                      </div>

                      <div className="mt-2.5">
                        <label className="text-[11px] text-slate-400 block mb-0.5">Contraseña:</label>
                        <div className="relative">
                          <input
                            type={showPass ? 'text' : 'password'}
                            value={acc.password || ''}
                            onChange={(e) => handleUpdateAccount(acc.id, { password: e.target.value })}
                            placeholder="Contraseña institucional"
                            className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 font-mono text-slate-200 pr-8 focus:outline-none focus:border-cyan-500"
                          />
                          <button
                            type="button"
                            onClick={() => togglePasswordVisibility(acc.id)}
                            className="absolute right-2 top-2 text-slate-400 hover:text-slate-200"
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
                  <h3 className="font-semibold text-slate-200">Rutas Excluidas (Bypass LAN / Intranet)</h3>
                  <span className="text-[11px] text-slate-400">Tráfico directo sin pasar por el túnel</span>
                </div>

                <div className="flex gap-2">
                  <input
                    type="text"
                    value={newBypassInput}
                    onChange={(e) => setNewBypassInput(e.target.value)}
                    placeholder="ej. 10.0.0.0/8 o 192.168.1.0/24"
                    className="flex-1 bg-slate-900 border border-slate-800 rounded-lg px-3 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                  <button
                    onClick={handleAddBypass}
                    className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-cyan-300 font-medium transition"
                  >
                    Añadir
                  </button>
                </div>

                <div className="flex flex-wrap gap-1.5 pt-1">
                  {cfg.bypass_routes.map((route) => (
                    <span
                      key={route}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-slate-900 border border-slate-800 text-slate-300 font-mono text-[11px]"
                    >
                      {route}
                      <button
                        onClick={() => handleRemoveBypass(route)}
                        className="text-slate-500 hover:text-red-400"
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </span>
                  ))}
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

              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Wifi className="w-4 h-4 text-emerald-400" />
                    <div>
                      <h4 className="font-semibold text-slate-200 text-xs">Forzar Hotspot sin internet</h4>
                      <p className="text-[11px] text-slate-400">Ejecuta `netsh wlan` y bypass NCSI para permitir anclaje offline</p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {forceHotspotLoading ? (
                      <RefreshCw className="w-4 h-4 text-cyan-400 animate-spin" />
                    ) : (
                      <button
                        onClick={() => handleToggleForceHotspot(!forceHotspotActive)}
                        className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                          forceHotspotActive ? 'bg-emerald-600' : 'bg-slate-800'
                        }`}
                        title={forceHotspotActive ? 'Desactivar y revertir' : 'Activar Forzar Hotspot'}
                      >
                        <span
                          className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                            forceHotspotActive ? 'translate-x-5' : 'translate-x-0'
                          }`}
                        />
                      </button>
                    )}
                  </div>
                </div>

                {/* Status Notice & Automated Rollback Feedback */}
                {forceHotspotNotice && (
                  <div
                    className={`p-2.5 rounded-lg border text-xs flex items-start gap-2 ${
                      forceHotspotActive
                        ? 'bg-emerald-950/40 border-emerald-800/60 text-emerald-200'
                        : 'bg-slate-900 border-slate-800 text-slate-300'
                    }`}
                  >
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
                    <div className="space-y-0.5">
                      <span className="font-medium block">{forceHotspotNotice}</span>
                      <span className="text-[10px] text-slate-400">
                        {forceHotspotActive
                          ? 'En caso de desconexión o falla, se ejecutará la reversión automática de `netsh`.'
                          : 'Configuración de red restaurada a los valores de fábrica de Windows.'}
                      </span>
                    </div>
                  </div>
                )}

                <div className="flex items-center justify-between pt-1 border-t border-slate-900">
                  <span className="text-[11px] text-slate-400">Diagnóstico Realtek RTL8821CE (SoftAP)</span>
                  <button
                    onClick={async () => {
                      const res = await bridge.checkHotspotCapability();
                      alert(res);
                    }}
                    className="px-2.5 py-1 rounded-md bg-slate-900 hover:bg-slate-800 text-cyan-300 font-medium text-[11px] border border-slate-800 transition shadow-sm"
                  >
                    Verificar Wi-Fi Direct
                  </button>
                </div>
              </div>

              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-2">
                <h4 className="font-semibold text-slate-300">Ubicación del Archivo de Configuración</h4>
                <p className="text-slate-400 font-mono text-[11px] bg-slate-900 p-2 rounded border border-slate-800">
                  %USERPROFILE%\UniTunnel\config.json
                </p>
                <p className="text-[11px] text-slate-500">
                  Cualquier cambio guardado desde este panel se sincroniza de inmediato con el archivo JSON y con el backend de Go.
                </p>
              </div>
            </div>
          )}

          {/* TAB 5: OPTIONAL VPS */}
          {activeTab === 'vps' && (
            <div className="space-y-4">
              <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-slate-200 flex items-center gap-1.5">
                    <Globe className="w-4 h-4 text-cyan-400" />
                    Puente VPS Secundario (Salida Externa)
                  </h3>
                  <input
                    type="checkbox"
                    checked={cfg.vps_enabled}
                    onChange={(e) => setCfg({ ...cfg, vps_enabled: e.target.checked })}
                    className="w-4 h-4 rounded text-cyan-600 focus:ring-0 bg-slate-900 border-slate-700"
                  />
                </div>
                <p className="text-slate-400 text-[11px]">
                  Permite encadenar el túnel universitario a través de un servidor VPS externo para cifrado adicional y acceso sin filtros.
                </p>

                {cfg.vps_enabled && (
                  <div className="space-y-3 pt-2">
                    <div>
                      <label className="text-slate-400 block mb-1">Servidor VPS (IP o Dominio:Puerto):</label>
                      <input
                        type="text"
                        value={cfg.vps_server}
                        onChange={(e) => setCfg({ ...cfg, vps_server: e.target.value })}
                        placeholder="vps.ejemplo.com:443"
                        className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                      />
                    </div>
                    <div>
                      <label className="text-slate-400 block mb-1">Clave Secreta Pre-compartida (PSK):</label>
                      <input
                        type="password"
                        value={cfg.vps_secret}
                        onChange={(e) => setCfg({ ...cfg, vps_secret: e.target.value })}
                        placeholder="Token o contraseña de enlace"
                        className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

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

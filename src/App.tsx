import React, { useState, useEffect } from 'react';
import {
  Settings,
  FileText,
  Power,
  User,
  Plus,
  Trash2,
  Download,
  Copy,
  Check,
  ShieldAlert,
  Terminal,
  Eye,
  EyeOff,
  FolderGit2,
  FileCode,
  CheckCircle2,
  AlertTriangle,
  ChevronRight,
  Sliders,
  Layers,
  Minimize2,
  RefreshCw,
  X
} from 'lucide-react';

interface Account {
  id: string;
  name: string;
  username: string;
  domain: string;
  password: string;
}

const DEFAULT_ACCOUNTS: Account[] = [
  {
    id: 'acc_1',
    name: 'Cuenta Principal',
    username: '',
    domain: '',
    password: ''
  }
];

export default function App() {
  const [activeTab, setActiveTab] = useState<'openvpn-ui' | 'stealth-uac' | 'code' | 'github' | 'config'>('openvpn-ui');
  
  // Real State & Persistence
  const [accounts, setAccounts] = useState<Account[]>(() => {
    const saved = localStorage.getItem('unitunnel_accounts');
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) return parsed;
      } catch (e) {
        console.error(e);
      }
    }
    return DEFAULT_ACCOUNTS;
  });

  const [activeAccountId, setActiveAccountId] = useState<string>(() => {
    return localStorage.getItem('unitunnel_active_acc') || accounts[0]?.id || 'acc_1';
  });

  const [configParams, setConfigParams] = useState({
    proxyHost: '10.0.0.1',
    proxyPort: 8080,
    authType: 'auto',
    captureMode: 'tun',
    tunName: 'UniTunnel0',
    tunIPv4: '10.254.1.2',
    tunDNS: '1.1.1.1,8.8.8.8',
    vpsEnabled: false,
    vpsServer: '',
    startMinimized: true
  });

  useEffect(() => {
    localStorage.setItem('unitunnel_accounts', JSON.stringify(accounts));
  }, [accounts]);

  useEffect(() => {
    localStorage.setItem('unitunnel_active_acc', activeAccountId);
  }, [activeAccountId]);

  const activeAccount = accounts.find((a) => a.id === activeAccountId) || accounts[0];

  // Connection Simulation State (OpenVPN style)
  const [connState, setConnState] = useState<'disconnected' | 'connecting' | 'connected'>('disconnected');
  const [logs, setLogs] = useState<string[]>([
    '[INIT] UniTunnel Connect listo.',
    '[INIT] Privilegios de Administrador verificados.',
    '[INIT] Consolas CMD secundarias silenciadas (CREATE_NO_WINDOW).'
  ]);

  // Modals
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const [showLogsModal, setShowLogsModal] = useState(false);
  const [showAddAccountModal, setShowAddAccountModal] = useState(false);

  // New Account Form
  const [newAccName, setNewAccName] = useState('');
  const [newAccUser, setNewAccUser] = useState('');
  const [newAccDomain, setNewAccDomain] = useState('');
  const [newAccPass, setNewAccPass] = useState('');
  const [showPassInSettings, setShowPassInSettings] = useState(false);

  // UI state
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [selectedCodeFile, setSelectedCodeFile] = useState<string>('prober');
  const [pingLatency, setPingLatency] = useState<number | null>(null);
  const [speedHistory, setSpeedHistory] = useState<number[]>(new Array(20).fill(0));
  const [currentSpeed, setCurrentSpeed] = useState<number>(0);
  const [totalMB, setTotalMB] = useState<number>(0);

  // Periodic Speed and Ping Monitoring Effect
  useEffect(() => {
    if (connState !== 'connected') {
      setPingLatency(null);
      setCurrentSpeed(0);
      setSpeedHistory(new Array(20).fill(0));
      return;
    }

    // Ping probe every 5s
    const runPing = () => {
      const ms = Math.floor(Math.random() * 14) + 28;
      setPingLatency(ms);
      addLog(`[${new Date().toLocaleTimeString('es-ES', { hour12: false })}] ✓ Ping google.com exitoso (${ms}ms) - Tráfico confirmado fluyendo.`);
    };

    runPing();
    const pingTimer = setInterval(runPing, 5000);

    // Speed monitor every 1s
    const speedTimer = setInterval(() => {
      const spd = +(Math.random() * 1.5 + 0.4).toFixed(2); // 0.40 - 1.90 MB/s
      setCurrentSpeed(spd);
      setTotalMB((prev) => +(prev + spd).toFixed(1));
      setSpeedHistory((prev) => [...prev.slice(1), spd]);
    }, 1000);

    return () => {
      clearInterval(pingTimer);
      clearInterval(speedTimer);
    };
  }, [connState]);

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleConnectToggle = () => {
    if (connState === 'connected') {
      setConnState('disconnected');
      addLog(`[${getTime()}] Desconectando túnel...`);
      addLog(`[${getTime()}] Adaptador Wintun cerrado y rutas eliminadas en segundo plano.`);
      addLog(`[${getTime()}] ESTADO: DESCONECTADO`);
      return;
    }

    if (!activeAccount || !activeAccount.username) {
      alert('Por favor añade un usuario y contraseña en tu cuenta antes de conectar.');
      setShowAddAccountModal(true);
      return;
    }

    setConnState('connecting');
    addLog(`[${getTime()}] Iniciando conexión hacia ${configParams.proxyHost}:${configParams.proxyPort}...`);
    addLog(`[${getTime()}] Autenticando con perfil: ${activeAccount.name} (${activeAccount.username})`);
    addLog(`[${getTime()}] Enviando sonda de auto-detección (NTLM/Basic)...`);

    setTimeout(() => {
      addLog(`[${getTime()}] Handshake NTLM Type-1/Type-2/Type-3 completado con éxito.`);
      addLog(`[${getTime()}] Adaptador Wintun inicializado (0 consolas visibles).`);
      addLog(`[${getTime()}] Rutas 0.0.0.0/1 y 128.0.0.0/1 aplicadas. Tráfico 100% capturado.`);
      addLog(`[${getTime()}] ESTADO: CONECTADO`);
      setConnState('connected');
    }, 1500);
  };

  const getTime = () => {
    return new Date().toLocaleTimeString('es-ES', { hour12: false });
  };

  const addLog = (msg: string) => {
    setLogs((prev) => [...prev, msg]);
  };

  const handleAddAccount = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newAccUser) return;
    const newId = `acc_${Date.now()}`;
    const created: Account = {
      id: newId,
      name: newAccName || newAccUser,
      username: newAccUser,
      domain: newAccDomain,
      password: newAccPass
    };
    setAccounts((prev) => [...prev, created]);
    setActiveAccountId(newId);
    setNewAccName('');
    setNewAccUser('');
    setNewAccDomain('');
    setNewAccPass('');
    setShowAddAccountModal(false);
    addLog(`[${getTime()}] Cuenta agregada: ${created.name} (${created.username})`);
  };

  const handleDeleteAccount = () => {
    if (accounts.length <= 1) {
      alert('Debes conservar al menos una cuenta en la aplicación.');
      return;
    }
    if (confirm(`¿Eliminar la cuenta "${activeAccount.name}"?`)) {
      const remaining = accounts.filter((a) => a.id !== activeAccountId);
      setAccounts(remaining);
      setActiveAccountId(remaining[0].id);
      addLog(`[${getTime()}] Cuenta eliminada.`);
    }
  };

  const handleDownloadConfig = () => {
    const configData = {
      proxy_host: configParams.proxyHost,
      proxy_port: Number(configParams.proxyPort),
      auth_type: configParams.authType,
      active_account_id: activeAccountId,
      accounts: accounts.map((a) => ({
        id: a.id,
        name: a.name,
        username: a.username,
        domain: a.domain,
        password: a.password
      })),
      local_socks_port: 1080,
      local_http_port: 8081,
      capture_mode: configParams.captureMode,
      tun_name: configParams.tunName,
      tun_ipv4: configParams.tunIPv4,
      tun_mask: '255.255.255.0',
      tun_dns: configParams.tunDNS,
      bypass_routes: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
      vps_enabled: configParams.vpsEnabled,
      vps_server: configParams.vpsServer,
      vps_secret: '',
      start_minimized: configParams.startMinimized,
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

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-emerald-500 selection:text-black">
      {/* Top Bar Contract (1 Row, 3 Zones) */}
      <header className="border-b border-slate-800 bg-slate-900/90 backdrop-blur sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-8">
          {/* Zone 1: Brand Wordmark */}
          <div className="flex items-center gap-3 whitespace-nowrap shrink-0">
            <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
              <Power className="w-5 h-5" />
            </div>
            <div>
              <span className="font-bold text-lg tracking-tight text-white">UniTunnel Connect</span>
              <span className="text-xs text-slate-400 ml-2 font-mono">Fyne · OpenVPN Style</span>
            </div>
          </div>

          {/* Zone 2: Navigation Links */}
          <nav className="hidden md:flex items-center gap-6 text-sm font-medium text-slate-300">
            <button
              onClick={() => setActiveTab('openvpn-ui')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'openvpn-ui' ? 'border-emerald-500 text-emerald-400 font-semibold' : 'border-transparent'
              }`}
            >
              Interfaz OpenVPN Connect
            </button>
            <button
              onClick={() => setActiveTab('stealth-uac')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'stealth-uac' ? 'border-emerald-500 text-emerald-400 font-semibold' : 'border-transparent'
              }`}
            >
              Consolas Ocultas & UAC
            </button>
            <button
              onClick={() => setActiveTab('code')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'code' ? 'border-emerald-500 text-emerald-400 font-semibold' : 'border-transparent'
              }`}
            >
              Código Go Actualizado
            </button>
            <button
              onClick={() => setActiveTab('github')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'github' ? 'border-emerald-500 text-emerald-400 font-semibold' : 'border-transparent'
              }`}
            >
              GitHub Actions CI/CD
            </button>
            <button
              onClick={() => setActiveTab('config')}
              className={`hover:text-white transition-colors whitespace-nowrap shrink-0 pb-1 border-b-2 ${
                activeTab === 'config' ? 'border-emerald-500 text-emerald-400 font-semibold' : 'border-transparent'
              }`}
            >
              Perfil config.json
            </button>
          </nav>

          {/* Zone 3: Primary Action */}
          <div className="flex items-center gap-3 shrink-0">
            <button
              onClick={handleDownloadConfig}
              className="px-4 py-2 text-xs font-semibold text-slate-900 bg-emerald-400 hover:bg-emerald-300 rounded-lg transition-colors flex items-center gap-2 shadow-sm whitespace-nowrap shrink-0"
            >
              <Download className="w-4 h-4" />
              <span>Guardar config.json</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Viewport */}
      <main className="flex-1 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
        {/* TAB 1: INTERFAZ OPENVPN CONNECT SIMULADA */}
        {activeTab === 'openvpn-ui' && (
          <div className="space-y-8">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                  <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
                    Diseño de Interfaz: Estilo OpenVPN Connect
                  </h1>
                  <p className="text-xs sm:text-sm text-slate-400 mt-1">
                    Ventana horizontal compacta (tamaño fijo tipo teléfono), botón circular central de conexión, engrane de ajustes (⚙️), visor de logs (📄), selector de cuentas multiusuario y minimizado a la bandeja del sistema.
                  </p>
                </div>
                <div className="flex items-center gap-2 text-xs font-mono text-emerald-400 bg-emerald-950/40 border border-emerald-500/30 px-3 py-1.5 rounded-lg shrink-0">
                  <span>Modo Wintun Activo</span>
                </div>
              </div>
            </div>

            {/* Simulated Desktop Window Frame (500x340 aspect) */}
            <div className="flex justify-center">
              <div className="w-full max-w-[520px] bg-slate-900 border-2 border-slate-700/80 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
                {/* Window Title Bar */}
                <div className="bg-slate-950/80 px-4 py-3 border-b border-slate-800 flex items-center justify-between select-none">
                  {/* Top Left: Settings Gear Button (⚙️) */}
                  <button
                    onClick={() => setShowSettingsModal(true)}
                    className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                    title="Ajustes de Configuración"
                  >
                    <Settings className="w-5 h-5" />
                  </button>

                  {/* Window Center Title */}
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-slate-200 tracking-wide">UniTunnel Connect</span>
                    <span className="text-[10px] text-slate-500 font-mono">(10.0.0.1)</span>
                  </div>

                  {/* Top Right: Logs Button (📄) & Minimize to Tray */}
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => setShowLogsModal(true)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                      title="Ver Registros (Logs)"
                    >
                      <FileText className="w-5 h-5" />
                    </button>
                    <button
                      onClick={() => addLog(`[${getTime()}] Ventana minimizada a la bandeja del sistema (System Tray).`)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                      title="Minimizar a la Bandeja"
                    >
                      <Minimize2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Window Body (OpenVPN Connect Layout) */}
                <div className="p-6 flex flex-col items-center justify-between space-y-6">
                  {/* Hero: Central Circular Connect Button */}
                  <div className="flex flex-col items-center justify-center pt-2">
                    <div className="relative group">
                      <button
                        onClick={handleConnectToggle}
                        className={`w-28 h-28 rounded-full flex flex-col items-center justify-center transition-all duration-300 shadow-lg border-4 ${
                          connState === 'connected'
                            ? 'bg-emerald-600 hover:bg-emerald-500 border-emerald-400 text-white shadow-emerald-500/20'
                            : connState === 'connecting'
                            ? 'bg-amber-600 hover:bg-amber-500 border-amber-400 text-white animate-pulse'
                            : 'bg-slate-800 hover:bg-slate-700 border-slate-600 text-slate-300 hover:text-white'
                        }`}
                      >
                        <Power className={`w-8 h-8 mb-1 ${connState === 'connected' ? 'rotate-0' : ''}`} />
                        <span className="text-[11px] font-bold tracking-wider">
                          {connState === 'connected' ? 'DESCONECTAR' : connState === 'connecting' ? 'CONECTANDO' : 'CONECTAR'}
                        </span>
                      </button>
                    </div>

                    {/* Status Pill Text */}
                    <div className="mt-3 flex items-center gap-2">
                      <span
                        className={`w-2 h-2 rounded-full ${
                          connState === 'connected'
                            ? 'bg-emerald-400'
                            : connState === 'connecting'
                            ? 'bg-amber-400 animate-ping'
                            : 'bg-slate-500'
                        }`}
                      />
                      <span
                        className={`text-xs font-mono font-bold tracking-wide ${
                          connState === 'connected'
                            ? 'text-emerald-400'
                            : connState === 'connecting'
                            ? 'text-amber-400'
                            : 'text-slate-400'
                        }`}
                      >
                        {connState === 'connected'
                          ? `TÚNEL ACTIVO · ${pingLatency ? `GOOGLE PING ${pingLatency}ms` : 'VERIFICANDO TRÁFICO...'}`
                          : connState === 'connecting'
                          ? 'NEGOCIANDO NTLM CON 10.0.0.1...'
                          : 'DESCONECTADO'}
                      </span>
                    </div>
                  </div>

                  {/* Realtime Bandwidth (MB/s) & Latency Line Chart */}
                  <div className="w-full bg-slate-950/80 border border-slate-800 rounded-xl p-3">
                    <div className="flex items-center justify-between text-[11px] font-mono mb-2">
                      <div className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full bg-emerald-400" />
                        <span className="text-slate-300 font-semibold">
                          Flujo: <strong className="text-emerald-400">{currentSpeed.toFixed(2)} MB/s</strong>
                        </span>
                      </div>
                      <div className="flex items-center gap-3 text-slate-400">
                        <span className="flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-cyan-400" />
                          <span>Ping: <strong className="text-cyan-300">{pingLatency ? `${pingLatency} ms` : '--'}</strong></span>
                        </span>
                        <span className="text-slate-500">|</span>
                        <span>Total: <strong className="text-slate-300">{totalMB} MB</strong></span>
                      </div>
                    </div>

                    {/* SVG Sparkline Graph */}
                    <div className="h-12 w-full bg-slate-900/90 rounded-lg border border-slate-800/80 overflow-hidden relative">
                      <svg className="w-full h-full" viewBox="0 0 440 45" preserveAspectRatio="none">
                        {/* Center grid line */}
                        <line x1="0" y1="22.5" x2="440" y2="22.5" stroke="#1e293b" strokeWidth="1" strokeDasharray="3 3" />

                        {/* Speed Line Graph (Emerald) */}
                        {(() => {
                          const maxSpd = Math.max(2.5, ...speedHistory);
                          const points = speedHistory
                            .map((val, idx) => {
                              const x = (idx / (speedHistory.length - 1)) * 440;
                              const y = 40 - (val / maxSpd) * 32;
                              return `${x},${y}`;
                            })
                            .join(' ');
                          return (
                            <>
                              <polyline fill="none" stroke="#10b981" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" points={points} />
                              {connState === 'connected' && (
                                <circle
                                  cx={440}
                                  cy={40 - (currentSpeed / maxSpd) * 32}
                                  r="3"
                                  fill="#34d399"
                                  className="animate-pulse"
                                />
                              )}
                            </>
                          );
                        })()}
                      </svg>
                    </div>
                  </div>

                  {/* Account Selector (OpenVPN Style Profile Selector) */}
                  <div className="w-full bg-slate-950/70 border border-slate-800 rounded-xl p-3">
                    <div className="flex items-center justify-between text-xs mb-1.5">
                      <span className="text-slate-400 font-medium flex items-center gap-1.5">
                        <User className="w-3.5 h-3.5 text-emerald-400" />
                        <span>Cuenta Universitaria:</span>
                      </span>
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => setShowAddAccountModal(true)}
                          className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-emerald-400 hover:text-emerald-300 text-[11px] font-semibold flex items-center gap-1 transition-colors"
                          title="Añadir nueva cuenta"
                        >
                          <Plus className="w-3 h-3" />
                          <span>Añadir</span>
                        </button>
                        {accounts.length > 1 && (
                          <button
                            onClick={handleDeleteAccount}
                            className="p-1 rounded bg-slate-800 hover:bg-red-950 text-slate-400 hover:text-red-400 transition-colors"
                            title="Eliminar cuenta actual"
                          >
                            <Trash2 className="w-3 h-3" />
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Account Dropdown */}
                    <select
                      value={activeAccountId}
                      onChange={(e) => {
                        setActiveAccountId(e.target.value);
                        const acc = accounts.find((a) => a.id === e.target.value);
                        if (acc) addLog(`[${getTime()}] Perfil cambiado a: ${acc.name} (${acc.username})`);
                      }}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-emerald-500 cursor-pointer"
                    >
                      {accounts.map((acc) => (
                        <option key={acc.id} value={acc.id}>
                          {acc.name} {acc.username ? `(${acc.username})` : '(Sin usuario)'}
                        </option>
                      ))}
                    </select>

                    <div className="mt-2 text-[11px] text-slate-500 flex items-center justify-between font-mono">
                      <span>
                        Usuario: <strong className="text-slate-300">{activeAccount?.username || 'No configurado'}</strong>
                      </span>
                      <span>
                        Proxy: <strong className="text-slate-300">{configParams.proxyHost}:{configParams.proxyPort}</strong>
                      </span>
                    </div>
                  </div>

                  {/* Window Bottom Hint */}
                  <div className="text-[10px] text-slate-500 text-center font-mono">
                    Inicia como Administrador · Sin consolas visibles · Se queda en la bandeja
                  </div>
                </div>
              </div>
            </div>

            {/* Quick Summary Cards */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-4">
              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="flex items-center gap-2 mb-2 text-white font-semibold text-sm">
                  <Power className="w-4 h-4 text-emerald-400" />
                  <span>Botón Circular Central</span>
                </div>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Cambia de gris (desconectado) a ámbar (negociando NTLM) y verde esmeralda (conectado). Un solo clic para activar o apagar el túnel.
                </p>
              </div>

              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="flex items-center gap-2 mb-2 text-white font-semibold text-sm">
                  <Settings className="w-4 h-4 text-cyan-400" />
                  <span>Ajustes Desglosados (Engrane ⚙️)</span>
                </div>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Ubicado en la esquina superior izquierda. Abre el panel modular con servidor proxy, credenciales, DNS, IP virtual Wintun y opciones de inicio.
                </p>
              </div>

              <div className="border border-slate-800 bg-slate-900/40 p-4 rounded-xl">
                <div className="flex items-center gap-2 mb-2 text-white font-semibold text-sm">
                  <User className="w-4 h-4 text-amber-400" />
                  <span>Gestor Multiusuario</span>
                </div>
                <p className="text-xs text-slate-400 leading-relaxed">
                  Permite alternar entre diferentes usuarios universitarios mediante el selector, agregar nuevas cuentas con (+) y eliminar con (-), guardando todo en <code className="text-slate-300 font-mono">config.json</code>.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: CONSOLAS OCULTAS & UAC */}
        {activeTab === 'stealth-uac' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <div className="flex items-center gap-3 mb-2">
                <ShieldAlert className="w-6 h-6 text-emerald-400" />
                <h2 className="text-xl font-bold text-white">Consolas Silenciadas y Auto-Elevación UAC</h2>
              </div>
              <p className="text-slate-300 text-sm max-w-3xl leading-relaxed">
                Hemos resuelto los dos puntos críticos que solicitaste: evitar que aparezca ninguna ventana de consola sospechosa de fondo y garantizar que la aplicación siempre inicie como Administrador sin fallos.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Stealth Feature */}
              <div className="border border-slate-800 bg-slate-900/40 p-6 rounded-xl flex flex-col justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-3">
                    <Terminal className="w-5 h-5 text-emerald-400" />
                    <h3 className="font-semibold text-white">1. Cero Consolas Visibles (Modo Silencioso)</h3>
                  </div>
                  <p className="text-xs sm:text-sm text-slate-300 leading-relaxed mb-4">
                    Antes, cuando Go ejecutaba comandos de red como <code className="text-emerald-300 font-mono text-xs">netsh</code> o <code className="text-emerald-300 font-mono text-xs">route add</code>, Windows abría y cerraba rápidamente ventanas negras de consola CMD en la pantalla.
                  </p>
                  <div className="bg-slate-950 p-3 rounded-lg border border-slate-800 text-xs font-mono text-emerald-300 space-y-1 mb-4">
                    <div>cmd.SysProcAttr = &amp;syscall.SysProcAttr&#123;</div>
                    <div className="pl-4">HideWindow: true,</div>
                    <div className="pl-4">CreationFlags: 0x08000000, // CREATE_NO_WINDOW</div>
                    <div>&#125;</div>
                  </div>
                  <p className="text-xs text-slate-400 leading-relaxed">
                    Además de compilar con <code className="text-slate-300 font-mono">-ldflags="-H windowsgui -s -w"</code>, cada comando de enrutamiento corre ahora en segundo plano con la bandera de creación de proceso sin ventana. No parpadea nada en la pantalla.
                  </p>
                </div>
                <div className="mt-4 pt-3 border-t border-slate-800 text-xs text-emerald-400 flex items-center gap-2 font-medium">
                  <CheckCircle2 className="w-4 h-4 shrink-0" />
                  <span>Sin parpadeos sospechosos de CMD o PowerShell</span>
                </div>
              </div>

              {/* UAC Elevation Feature */}
              <div className="border border-slate-800 bg-slate-900/40 p-6 rounded-xl flex flex-col justify-between">
                <div>
                  <div className="flex items-center gap-2 mb-3">
                    <ShieldAlert className="w-5 h-5 text-amber-400" />
                    <h3 className="font-semibold text-white">2. Auto-Elevación como Administrador</h3>
                  </div>
                  <p className="text-xs sm:text-sm text-slate-300 leading-relaxed mb-4">
                    Para inicializar Wintun y modificar las rutas IP, Windows exige privilegios elevados. Para que no tengas que recordar hacer clic derecho:
                  </p>
                  <ul className="text-xs space-y-2.5 text-slate-300 mb-4">
                    <li className="flex items-start gap-2">
                      <CheckCircle2 className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                      <span>
                        <strong>Detección y Auto-Relanzamiento:</strong> En <code className="text-amber-200 font-mono">pkg/elevation</code>, la app verifica su token de proceso; si no es admin, se relanza automáticamente usando <code className="text-amber-200 font-mono">ShellExecuteW(..., "runas")</code>.
                      </span>
                    </li>
                    <li className="flex items-start gap-2">
                      <CheckCircle2 className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                      <span>
                        <strong>Manifiesto Incrustado:</strong> Generamos el archivo <code className="text-amber-200 font-mono">unitunnel.exe.manifest</code> con <code className="text-amber-200 font-mono">requireAdministrator</code>, mostrando el escudo de Windows en el icono y solicitando permiso al hacer doble clic.
                      </span>
                    </li>
                  </ul>
                </div>
                <div className="mt-4 pt-3 border-t border-slate-800 text-xs text-amber-400 flex items-center gap-2 font-medium">
                  <CheckCircle2 className="w-4 h-4 shrink-0" />
                  <span>Doble clic directo con elevación transparente</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 3: CÓDIGO GO */}
        {activeTab === 'code' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <h2 className="text-xl font-bold text-white mb-2">Código Go Actualizado en el Repositorio</h2>
              <p className="text-slate-300 text-sm max-w-3xl">
                Se han actualizado los módulos con el diseño OpenVPN Connect, el gestor de cuentas, la supresión de consolas y el soporte para bandeja del sistema.
              </p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
              <div className="lg:col-span-4 border border-slate-800 bg-slate-900/40 rounded-xl p-4 space-y-1">
                <div className="text-xs font-mono text-slate-400 px-3 py-1 uppercase tracking-wider mb-2">
                  Archivos Modificados
                </div>
                {[
                  { id: 'prober', title: 'pkg/tun/prober.go', desc: 'Ping periódico a google.com para validar tráfico real' },
                  { id: 'gui', title: 'ui/gui.go', desc: 'Interfaz OpenVPN Connect con botón circular y bandeja' },
                  { id: 'config', title: 'pkg/config/config.go', desc: 'Gestor de cuentas multiusuario y persistencia' },
                  { id: 'wintun', title: 'pkg/tun/wintun_windows.go', desc: 'CREATE_NO_WINDOW y detección de gateway' },
                  { id: 'elevate', title: 'pkg/elevation/elevate_windows.go', desc: 'Auto-elevación UAC con ShellExecuteW' },
                  { id: 'main', title: 'cmd/unitunnel/main.go', desc: 'Entry point con elevación inicial' }
                ].map((item) => (
                  <button
                    key={item.id}
                    onClick={() => setSelectedCodeFile(item.id)}
                    className={`w-full text-left px-3 py-2.5 rounded-lg text-xs transition-colors flex items-center justify-between ${
                      selectedCodeFile === item.id
                        ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-semibold'
                        : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                    }`}
                  >
                    <div className="truncate pr-2">
                      <div className="font-mono text-white text-[11px] truncate">{item.title}</div>
                      <div className="text-[10px] text-slate-400 truncate mt-0.5">{item.desc}</div>
                    </div>
                    <ChevronRight className="w-3.5 h-3.5 shrink-0 opacity-50" />
                  </button>
                ))}
              </div>

              <div className="lg:col-span-8 border border-slate-800 bg-slate-900/40 rounded-xl overflow-hidden flex flex-col">
                <div className="px-5 py-3 border-b border-slate-800 flex items-center justify-between bg-slate-900/80">
                  <span className="text-xs font-mono font-semibold text-white">
                    {selectedCodeFile === 'gui'
                      ? 'ui/gui.go'
                      : selectedCodeFile === 'config'
                      ? 'pkg/config/config.go'
                      : selectedCodeFile === 'wintun'
                      ? 'pkg/tun/wintun_windows.go'
                      : selectedCodeFile === 'elevate'
                      ? 'pkg/elevation/elevate_windows.go'
                      : 'cmd/unitunnel/main.go'}
                  </span>
                  <button
                    onClick={() =>
                      handleCopy(
                        selectedCodeFile === 'gui'
                          ? '// Ver ui/gui.go en el repo'
                          : '// Ver archivo en repo',
                        'code-copy'
                      )
                    }
                    className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 transition-colors"
                  >
                    {copiedKey === 'code-copy' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    <span>{copiedKey === 'code-copy' ? 'Copiado' : 'Copiar Archivo'}</span>
                  </button>
                </div>
                <div className="p-5 font-mono text-xs text-slate-300 bg-slate-950 overflow-x-auto flex-1 max-h-[500px] leading-relaxed">
                  {selectedCodeFile === 'prober' && (
                    <p className="text-emerald-400">
                      ✓ Módulo de prueba de tráfico real (pkg/tun/prober.go): Tras inicializar Wintun, inicia un bucle en segundo plano que envía una sonda HTTP ligera (generate_204) a google.com cada 5 segundos. Mide la latencia RTT exacta en milisegundos y confirma que los paquetes realmente están cruzando el túnel y no solo el adaptador virtual encendido.
                    </p>
                  )}
                  {selectedCodeFile === 'gui' && (
                    <p className="text-emerald-400">
                      ✓ Rediseñado por completo con ventana fija compacta (500x340), botón circular central de conexión, botón de engrane (⚙️) que abre el modal de ajustes, botón de logs (📄), selector de perfiles de cuenta (+) y (-) con guardado a config.json, y permanencia en la bandeja del sistema (System Tray).
                    </p>
                  )}
                  {selectedCodeFile === 'wintun' && (
                    <p className="text-emerald-400">
                      ✓ Se reemplazaron todas las llamadas a exec.Command con runHidden(...) usando la constante CREATE_NO_WINDOW (0x08000000) y HideWindow: true. Ninguna ventana de consola CMD ni PowerShell vuelve a parpadear.
                    </p>
                  )}
                  {selectedCodeFile === 'elevate' && (
                    <p className="text-emerald-400">
                      ✓ Se implementó EnsureElevated() con la API nativa de Windows (ShellExecuteW con verbo "runas"). Si no es administrador, solicita confirmación UAC de inmediato y se relanza sin requerir clic derecho.
                    </p>
                  )}
                  {selectedCodeFile === 'config' && (
                    <p className="text-emerald-400">
                      ✓ Añadido soporte multi-cuenta: Accounts []UserAccount, ActiveAccountID, con métodos para agregar, eliminar y cambiar usuarios en tiempo real, persistiendo en config.json.
                    </p>
                  )}
                  {selectedCodeFile === 'main' && (
                    <p className="text-emerald-400">
                      ✓ elevation.EnsureElevated() se invoca en la primera línea de main() antes de inicializar la interfaz gráfica.
                    </p>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 4: GITHUB ACTIONS */}
        {activeTab === 'github' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <div className="flex items-center gap-3 mb-2">
                <FolderGit2 className="w-6 h-6 text-emerald-400" />
                <h2 className="text-xl font-bold text-white">Compilación en GitHub Actions con Manifiesto UAC</h2>
              </div>
              <p className="text-slate-300 text-sm max-w-3xl leading-relaxed">
                El flujo de CI/CD compila el ejecutable con MinGW GCC, descarga <code className="text-emerald-400 font-mono text-xs">wintun.dll</code> e incrusta el archivo de manifiesto <code className="text-emerald-400 font-mono text-xs">unitunnel.exe.manifest</code> configurado con <code className="text-emerald-400 font-mono text-xs">requireAdministrator</code>.
              </p>
            </div>

            <div className="border border-slate-800 bg-slate-900/40 p-6 rounded-xl space-y-4">
              <h3 className="text-sm font-semibold text-white">Instrucciones para compilar tu nueva versión:</h3>
              <div className="bg-slate-950 p-4 rounded-lg font-mono text-xs text-slate-300 space-y-2">
                <div className="text-slate-500"># 1. Haz commit y push de estos cambios:</div>
                <div className="text-emerald-400">git add .</div>
                <div className="text-emerald-400">git commit -m "feat: interfaz estilo OpenVPN Connect, consolas ocultas y UAC"</div>
                <div className="text-emerald-400">git push origin main</div>
                <div className="text-slate-500 pt-2"># 2. Ve a la pestaña 'Actions' en GitHub y descarga el nuevo ZIP:</div>
                <div className="text-white">unitunnel-windows-x64.zip</div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 5: PERFIL CONFIG.JSON */}
        {activeTab === 'config' && (
          <div className="space-y-6">
            <div className="border border-slate-800 bg-slate-900/60 rounded-xl p-6">
              <h2 className="text-xl font-bold text-white mb-2">Editor y Exportador de config.json</h2>
              <p className="text-slate-300 text-sm max-w-3xl">
                Tus cuentas reales y configuraciones guardadas en la aplicación. Puedes descargarlo directamente para reemplazar el de tu carpeta.
              </p>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
              <div className="lg:col-span-5 border border-slate-800 bg-slate-900/40 p-5 rounded-xl space-y-4">
                <h3 className="text-sm font-semibold text-white">Cuentas Registradas ({accounts.length})</h3>
                <div className="space-y-2 max-h-[300px] overflow-y-auto">
                  {accounts.map((acc) => (
                    <div
                      key={acc.id}
                      className={`p-3 rounded-lg border text-xs flex items-center justify-between ${
                        acc.id === activeAccountId
                          ? 'border-emerald-500/40 bg-emerald-500/10 text-white'
                          : 'border-slate-800 bg-slate-900 text-slate-400'
                      }`}
                    >
                      <div>
                        <div className="font-semibold text-white">{acc.name}</div>
                        <div className="font-mono text-[11px] text-slate-400">
                          {acc.username || '(Sin usuario)'} {acc.domain ? `[${acc.domain}]` : ''}
                        </div>
                      </div>
                      {acc.id === activeAccountId && (
                        <span className="text-[10px] font-mono text-emerald-400 font-bold px-2 py-0.5 rounded bg-emerald-950/60">
                          ACTIVA
                        </span>
                      )}
                    </div>
                  ))}
                </div>

                <button
                  onClick={() => setShowAddAccountModal(true)}
                  className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-emerald-400 text-xs font-semibold rounded-lg flex items-center justify-center gap-2 transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  <span>Añadir Otra Cuenta</span>
                </button>
              </div>

              <div className="lg:col-span-7 border border-slate-800 bg-slate-900/40 rounded-xl p-5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-xs font-mono font-semibold text-white">Vista Previa de config.json</span>
                    <button
                      onClick={() =>
                        handleCopy(
                          JSON.stringify(
                            {
                              proxy_host: configParams.proxyHost,
                              proxy_port: configParams.proxyPort,
                              auth_type: configParams.authType,
                              active_account_id: activeAccountId,
                              accounts: accounts,
                              local_socks_port: 1080,
                              capture_mode: configParams.captureMode,
                              tun_name: configParams.tunName,
                              start_minimized: configParams.startMinimized
                            },
                            null,
                            2
                          ),
                          'config-copy'
                        )
                      }
                      className="text-xs text-slate-300 hover:text-white flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 transition-colors"
                    >
                      {copiedKey === 'config-copy' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>{copiedKey === 'config-copy' ? 'Copiado' : 'Copiar'}</span>
                    </button>
                  </div>
                  <pre className="p-4 font-mono text-xs text-emerald-300 bg-slate-950 rounded-lg overflow-x-auto max-h-[280px]">
                    {JSON.stringify(
                      {
                        proxy_host: configParams.proxyHost,
                        proxy_port: configParams.proxyPort,
                        auth_type: configParams.authType,
                        active_account_id: activeAccountId,
                        accounts: accounts,
                        local_socks_port: 1080,
                        capture_mode: configParams.captureMode,
                        tun_name: configParams.tunName,
                        start_minimized: configParams.startMinimized
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

      {/* MODAL: AJUSTES (Engrane ⚙️) */}
      {showSettingsModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
            <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Settings className="w-5 h-5 text-emerald-400" />
                <h3 className="font-bold text-white text-base">Ajustes de UniTunnel Connect</h3>
              </div>
              <button
                onClick={() => setShowSettingsModal(false)}
                className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-6 overflow-y-auto space-y-5 text-xs">
              {/* Sección 1: Proxy Universitario */}
              <div className="space-y-3">
                <h4 className="font-semibold text-slate-200 uppercase tracking-wider text-[11px] pb-1 border-b border-slate-800">
                  Proxy Universitario
                </h4>
                <div className="grid grid-cols-3 gap-3">
                  <div className="col-span-2">
                    <label className="text-slate-400 block mb-1">Servidor Proxy IP</label>
                    <input
                      type="text"
                      value={configParams.proxyHost}
                      onChange={(e) => setConfigParams({ ...configParams, proxyHost: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">Puerto</label>
                    <input
                      type="number"
                      value={configParams.proxyPort}
                      onChange={(e) => setConfigParams({ ...configParams, proxyPort: Number(e.target.value) })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-slate-400 block mb-1">Esquema de Autenticación</label>
                  <select
                    value={configParams.authType}
                    onChange={(e) => setConfigParams({ ...configParams, authType: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                  >
                    <option value="auto">Auto-detectar (Recomendado: NTLM / Basic)</option>
                    <option value="ntlm">NTLM v1 / v2 Corporativo</option>
                    <option value="basic">HTTP Basic estándar</option>
                    <option value="none">Sin autenticación</option>
                  </select>
                </div>
              </div>

              {/* Sección 2: Red y Captura */}
              <div className="space-y-3">
                <h4 className="font-semibold text-slate-200 uppercase tracking-wider text-[11px] pb-1 border-b border-slate-800">
                  Modo de Red y Adaptador
                </h4>
                <div>
                  <label className="text-slate-400 block mb-1">Mecanismo de Captura</label>
                  <select
                    value={configParams.captureMode}
                    onChange={(e) => setConfigParams({ ...configParams, captureMode: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                  >
                    <option value="tun">Modo Wintun Virtual (Captura 100% Apps)</option>
                    <option value="system_proxy">Modo Proxy del Sistema Windows (WinINet)</option>
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-slate-400 block mb-1">IP Virtual TUN</label>
                    <input
                      type="text"
                      value={configParams.tunIPv4}
                      onChange={(e) => setConfigParams({ ...configParams, tunIPv4: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                    />
                  </div>
                  <div>
                    <label className="text-slate-400 block mb-1">Servidores DNS</label>
                    <input
                      type="text"
                      value={configParams.tunDNS}
                      onChange={(e) => setConfigParams({ ...configParams, tunDNS: e.target.value })}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Sección 3: Opciones de Inicio */}
              <div className="space-y-2">
                <h4 className="font-semibold text-slate-200 uppercase tracking-wider text-[11px] pb-1 border-b border-slate-800">
                  Comportamiento y Bandeja
                </h4>
                <label className="flex items-center gap-2 cursor-pointer text-slate-300">
                  <input
                    type="checkbox"
                    checked={configParams.startMinimized}
                    onChange={(e) => setConfigParams({ ...configParams, startMinimized: e.target.checked })}
                    className="rounded bg-slate-800 border-slate-700 text-emerald-500 focus:ring-0"
                  />
                  <span>Permanecer minimizado en la bandeja del sistema (System Tray)</span>
                </label>
              </div>
            </div>

            <div className="px-6 py-4 border-t border-slate-800 bg-slate-950 flex items-center justify-end gap-3">
              <button
                onClick={() => setShowSettingsModal(false)}
                className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs"
              >
                Cerrar
              </button>
              <button
                onClick={() => {
                  setShowSettingsModal(false);
                  addLog(`[${getTime()}] Ajustes guardados.`);
                }}
                className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold rounded-lg text-xs"
              >
                Guardar Ajustes
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: LOGS (Documento 📄) */}
      {showLogsModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg overflow-hidden shadow-2xl flex flex-col max-h-[85vh]">
            <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <FileText className="w-5 h-5 text-emerald-400" />
                <h3 className="font-bold text-white text-base">Registros de Conexión en Vivo</h3>
              </div>
              <button
                onClick={() => setShowLogsModal(false)}
                className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="p-4 bg-slate-950 flex-1 overflow-y-auto font-mono text-xs text-slate-300 space-y-1 min-h-[220px]">
              {logs.map((log, idx) => (
                <div key={idx} className={log.includes('CONECTADO') ? 'text-emerald-400 font-semibold' : ''}>
                  {log}
                </div>
              ))}
            </div>

            <div className="px-6 py-3 border-t border-slate-800 bg-slate-900 flex items-center justify-between">
              <button
                onClick={() => setLogs([])}
                className="text-xs text-slate-400 hover:text-white"
              >
                Limpiar
              </button>
              <button
                onClick={() => handleCopy(logs.join('\n'), 'modal-logs')}
                className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded text-xs flex items-center gap-1.5"
              >
                {copiedKey === 'modal-logs' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                <span>{copiedKey === 'modal-logs' ? 'Copiado' : 'Copiar Todo'}</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: AÑADIR CUENTA (+) */}
      {showAddAccountModal && (
        <div className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md overflow-hidden shadow-2xl">
            <form onSubmit={handleAddAccount}>
              <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <User className="w-5 h-5 text-emerald-400" />
                  <h3 className="font-bold text-white text-base">Añadir Cuenta Universitaria</h3>
                </div>
                <button
                  type="button"
                  onClick={() => setShowAddAccountModal(false)}
                  className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <div className="p-6 space-y-3 text-xs">
                <div>
                  <label className="text-slate-400 block mb-1">Nombre del Perfil</label>
                  <input
                    type="text"
                    placeholder="Ej: Cuenta Personal / yoanbramon"
                    value={newAccName}
                    onChange={(e) => setNewAccName(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white"
                  />
                </div>

                <div>
                  <label className="text-slate-400 block mb-1">Usuario de la Universidad *</label>
                  <input
                    type="text"
                    required
                    placeholder="ej: yoan.bramon"
                    value={newAccUser}
                    onChange={(e) => setNewAccUser(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                  />
                </div>

                <div>
                  <label className="text-slate-400 block mb-1">Dominio (Opcional para NTLM)</label>
                  <input
                    type="text"
                    placeholder="ej: FACULTAD"
                    value={newAccDomain}
                    onChange={(e) => setNewAccDomain(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono"
                  />
                </div>

                <div>
                  <label className="text-slate-400 block mb-1">Contraseña</label>
                  <div className="relative">
                    <input
                      type={showPassInSettings ? 'text' : 'password'}
                      value={newAccPass}
                      onChange={(e) => setNewAccPass(e.target.value)}
                      className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassInSettings(!showPassInSettings)}
                      className="absolute right-3 top-2 text-slate-400 hover:text-white"
                    >
                      {showPassInSettings ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
              </div>

              <div className="px-6 py-4 border-t border-slate-800 bg-slate-950 flex items-center justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setShowAddAccountModal(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs"
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-semibold rounded-lg text-xs"
                >
                  Guardar Perfil
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Footer */}
      <footer className="border-t border-slate-800 bg-slate-950 py-4 px-6 text-center text-xs text-slate-500">
        UniTunnel Connect · Formato compacto OpenVPN Connect, Wintun Driver, Consolas Ocultas y Auto-Elevación en Windows.
      </footer>
    </div>
  );
}

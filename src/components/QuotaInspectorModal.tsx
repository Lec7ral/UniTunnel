import React, { useState } from 'react';
import {
  X,
  Send,
  Copy,
  Check,
  AlertTriangle,
  Code,
  FileText,
  User,
  Key,
  Globe,
  RefreshCw,
  Eye,
  EyeOff,
  Sparkles,
  Info,
} from 'lucide-react';
import { bridge, QuotaInspectionResult, UserAccount } from '../services/wailsBridge';

interface QuotaInspectorModalProps {
  isOpen: boolean;
  onClose: () => void;
  activeAccount?: UserAccount;
  defaultPortalUrl?: string;
}

export function QuotaInspectorModal({
  isOpen,
  onClose,
  activeAccount,
  defaultPortalUrl = 'https://cuota.uci.cu/',
}: QuotaInspectorModalProps) {
  const [portalUrl, setPortalUrl] = useState(defaultPortalUrl);
  const [username, setUsername] = useState(activeAccount?.username || '');
  const [password, setPassword] = useState(activeAccount?.password || '');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'raw' | 'json' | 'parsed'>('raw');
  const [result, setResult] = useState<QuotaInspectionResult | null>(null);
  const [copied, setCopied] = useState(false);
  const [pasteMode, setPasteMode] = useState(false);
  const [pastedRaw, setPastedRaw] = useState('');

  if (!isOpen) return null;

  const handleUseActiveAccount = () => {
    if (activeAccount) {
      setUsername(activeAccount.username || '');
      setPassword(activeAccount.password || '');
    }
  };

  const handleInspect = async () => {
    if (!username.trim()) {
      alert('Por favor introduce un nombre de usuario.');
      return;
    }
    setLoading(true);
    setResult(null);
    try {
      const res = await bridge.inspectQuota(portalUrl.trim(), username.trim(), password);
      setResult(res);
      if (res.raw_response) {
        setActiveTab('raw');
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setResult({
        url: portalUrl,
        status_code: 0,
        raw_response: '',
        timestamp: new Date().toLocaleTimeString(),
        success: false,
        error_message: msg,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleSimulateFromPaste = () => {
    if (!pastedRaw.trim()) return;
    const fixedJson = pastedRaw
      .replace(/([{,]\s*)([a-zA-Z0-9_]+)\s*:/g, '$1"$2":')
      .replace(/:\s*'([^']*)'/g, ': "$1"');

    let parseError = '';
    let parsedObj: any = null;
    try {
      parsedObj = JSON.parse(fixedJson);
    } catch (e: any) {
      parseError = e?.message || 'JSON Parse Error';
    }

    setResult({
      url: portalUrl,
      status_code: 200,
      status_text: 'Simulado',
      raw_response: pastedRaw,
      fixed_json: fixedJson,
      parse_error: parseError || undefined,
      timestamp: new Date().toLocaleTimeString(),
      success: parsedObj?.success !== undefined ? Boolean(parsedObj.success) : true,
    });
    setActiveTab('raw');
  };

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2500);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />

      {/* Modal Dialog */}
      <div className="relative z-10 w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl flex flex-col max-h-[92vh] overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        
        {/* Header */}
        <div className="p-4 sm:p-5 border-b border-slate-800 flex items-center justify-between bg-slate-950/60">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-cyan-950/70 border border-cyan-800/60 flex items-center justify-center text-cyan-400">
              <Code className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-sm sm:text-base font-bold text-slate-100 flex items-center gap-2">
                <span>Inspector de Respuesta de Cuota</span>
                <span className="text-[10px] px-1.5 py-0.2 rounded bg-cyan-950 text-cyan-300 border border-cyan-800/60 font-mono">
                  RAW Debug
                </span>
              </h2>
              <p className="text-[11px] text-slate-400">
                Consulta el servidor en vivo o pega datos para ver la estructura exacta devuelta por la UCI.
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Form and Body */}
        <div className="p-4 sm:p-5 overflow-y-auto space-y-4 text-xs">
          
          {/* Inputs Section */}
          <div className="p-3.5 rounded-xl bg-slate-950/70 border border-slate-800/80 space-y-3">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-200 text-xs flex items-center gap-1.5">
                <Globe className="w-3.5 h-3.5 text-cyan-400" />
                Parámetros de Consulta
              </span>
              <div className="flex items-center gap-2">
                {activeAccount?.username && (
                  <button
                    type="button"
                    onClick={handleUseActiveAccount}
                    className="text-[10px] text-cyan-400 hover:text-cyan-300 underline underline-offset-2 transition"
                  >
                    Usar cuenta activa ({activeAccount.username})
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => setPasteMode(!pasteMode)}
                  className={`text-[10px] px-2 py-0.5 rounded border transition ${
                    pasteMode
                      ? 'bg-cyan-950 text-cyan-300 border-cyan-600/60'
                      : 'bg-slate-900 text-slate-400 border-slate-800 hover:text-slate-200'
                  }`}
                >
                  {pasteMode ? 'Modo Consulta Directa' : 'Pegar RAW manualmente'}
                </button>
              </div>
            </div>

            {!pasteMode ? (
              <>
                <div className="space-y-1">
                  <label className="text-[11px] text-slate-400 flex items-center gap-1">
                    URL del Portal:
                  </label>
                  <input
                    type="text"
                    value={portalUrl}
                    onChange={(e) => setPortalUrl(e.target.value)}
                    placeholder="https://cuota.uci.cu/"
                    className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  <div className="space-y-1">
                    <label className="text-[11px] text-slate-400 flex items-center gap-1">
                      <User className="w-3 h-3 text-slate-400" />
                      Usuario:
                    </label>
                    <input
                      type="text"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      placeholder="ej. usuario"
                      className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 focus:outline-none focus:border-cyan-500"
                    />
                  </div>

                  <div className="space-y-1">
                    <label className="text-[11px] text-slate-400 flex items-center gap-1">
                      <Key className="w-3 h-3 text-slate-400" />
                      Contraseña:
                    </label>
                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        placeholder="Contraseña institucional"
                        className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 font-mono text-slate-200 pr-8 focus:outline-none focus:border-cyan-500"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-2 top-2 text-slate-400 hover:text-slate-200"
                      >
                        {showPassword ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                      </button>
                    </div>
                  </div>
                </div>

                <div className="pt-1 flex items-center justify-between">
                  <span className="text-[10px] text-slate-400 italic">
                    * La contraseña se envía en Base64 directamente a cuota.php sin pasar por Squid.
                  </span>
                  <button
                    onClick={handleInspect}
                    disabled={loading}
                    className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:bg-slate-800 text-white font-semibold transition shadow-md shadow-cyan-950/60"
                  >
                    {loading ? (
                      <>
                        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                        <span>Consultando...</span>
                      </>
                    ) : (
                      <>
                        <Send className="w-3.5 h-3.5" />
                        <span>Consultar Servidor</span>
                      </>
                    )}
                  </button>
                </div>
              </>
            ) : (
              <div className="space-y-2">
                <label className="text-[11px] text-slate-400 block">
                  Pega aquí la respuesta RAW obtenida (desde consola, curl o navegador):
                </label>
                <textarea
                  value={pastedRaw}
                  onChange={(e) => setPastedRaw(e.target.value)}
                  rows={4}
                  placeholder="{success:true, data:{...}}"
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg p-2.5 font-mono text-xs text-slate-200 focus:outline-none focus:border-cyan-500 resize-y"
                />
                <button
                  type="button"
                  onClick={handleSimulateFromPaste}
                  disabled={!pastedRaw.trim()}
                  className="px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 disabled:bg-slate-800 text-white font-medium transition"
                >
                  Analizar Texto Pegado
                </button>
              </div>
            )}
          </div>

          {/* Result Section */}
          {result && (
            <div className="space-y-3">
              {/* Status Header */}
              <div className="flex flex-wrap items-center justify-between gap-2 p-2.5 rounded-xl bg-slate-950/80 border border-slate-800">
                <div className="flex items-center gap-2">
                  <span
                    className={`px-2 py-0.5 rounded text-[10px] font-mono font-semibold uppercase ${
                      result.status_code === 200 || result.success
                        ? 'bg-emerald-950 text-emerald-300 border border-emerald-800/60'
                        : 'bg-red-950 text-red-300 border border-red-800/60'
                    }`}
                  >
                    {result.status_code ? `HTTP ${result.status_code}` : 'Error de Conexión'}
                  </span>
                  <span className="text-[11px] text-slate-400 font-mono truncate max-w-xs">
                    {result.url}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-[10px] text-slate-400">{result.timestamp}</span>
                  {result.raw_response && (
                    <button
                      type="button"
                      onClick={() => handleCopy(result.raw_response)}
                      className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white font-medium text-[11px] transition shadow"
                      title="Copiar texto RAW completo para enviarlo en el chat"
                    >
                      {copied ? (
                        <>
                          <Check className="w-3 h-3 text-emerald-200" />
                          <span>¡Copiado!</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3 h-3" />
                          <span>Copiar Respuesta RAW</span>
                        </>
                      )}
                    </button>
                  )}
                </div>
              </div>

              {/* Informational Callout to encourage sharing */}
              <div className="p-3 rounded-xl bg-cyan-950/40 border border-cyan-800/50 flex items-start gap-2 text-cyan-200">
                <Info className="w-4 h-4 text-cyan-400 shrink-0 mt-0.5" />
                <p className="leading-snug text-[11px]">
                  <strong>¿Datos incorrectos?</strong> Haz clic en <span className="text-cyan-300 font-semibold underline">"Copiar Respuesta RAW"</span> y pega el contenido en el chat. Con eso ajustaremos exactamente los nombres de campos que devuelve tu cuenta.
                </p>
              </div>

              {/* Tabs */}
              <div className="flex gap-1 border-b border-slate-800 pb-1">
                {[
                  { id: 'raw', label: 'Respuesta RAW (Exacta)', icon: FileText },
                  { id: 'json', label: 'JSON Normalizado', icon: Code },
                  { id: 'parsed', label: 'Interpretación Actual', icon: Sparkles },
                ].map((tab) => {
                  const Icon = tab.icon;
                  return (
                    <button
                      key={tab.id}
                      onClick={() => setActiveTab(tab.id as any)}
                      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                        activeTab === tab.id
                          ? 'bg-slate-800 text-cyan-400 border border-slate-700'
                          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
                      }`}
                    >
                      <Icon className="w-3.5 h-3.5" />
                      <span>{tab.label}</span>
                    </button>
                  );
                })}
              </div>

              {/* Tab 1: RAW Response */}
              {activeTab === 'raw' && (
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between text-[11px] text-slate-400">
                    <span>Tamaño: {result.raw_response.length} caracteres ({new Blob([result.raw_response]).size} bytes)</span>
                  </div>
                  <pre className="p-3 rounded-xl bg-slate-950 border border-slate-800 font-mono text-[11px] text-emerald-300 overflow-x-auto max-h-60 select-all whitespace-pre-wrap break-all leading-relaxed">
                    {result.raw_response || (result.error_message ? `[ERROR] ${result.error_message}` : '(Respuesta vacía)')}
                  </pre>
                </div>
              )}

              {/* Tab 2: Fixed JSON */}
              {activeTab === 'json' && (
                <div className="space-y-1.5">
                  {result.parse_error && (
                    <div className="p-2 rounded-lg bg-amber-950/60 border border-amber-800/60 text-amber-200 text-[11px] flex items-center gap-1.5">
                      <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                      <span>Aviso JSON: {result.parse_error}</span>
                    </div>
                  )}
                  <pre className="p-3 rounded-xl bg-slate-950 border border-slate-800 font-mono text-[11px] text-cyan-300 overflow-x-auto max-h-60 select-all whitespace-pre-wrap break-all leading-relaxed">
                    {(() => {
                      if (!result.fixed_json) return '(Sin JSON normalizado)';
                      try {
                        const parsed = JSON.parse(result.fixed_json);
                        return JSON.stringify(parsed, null, 2);
                      } catch {
                        return result.fixed_json;
                      }
                    })()}
                  </pre>
                </div>
              )}

              {/* Tab 3: Current Parser Interpretation */}
              {activeTab === 'parsed' && (
                <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                  <div className="text-[11px] font-semibold text-slate-300 mb-1">
                    Valores que el código extrae actualmente con esta respuesta:
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 font-mono text-[11px]">
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Asignado / Plan</div>
                      <div className="text-cyan-300 font-bold">
                        {result.quota_info?.allocated_mb ?? 0} MB
                      </div>
                    </div>
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Consumido / Usado</div>
                      <div className="text-amber-300 font-bold">
                        {result.quota_info?.used_mb ?? 0} MB
                      </div>
                    </div>
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Restante / Saldo</div>
                      <div className="text-emerald-300 font-bold">
                        {result.quota_info?.remaining_mb ?? 0} MB
                      </div>
                    </div>
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Porcentaje Usado</div>
                      <div className="text-slate-200 font-bold">
                        {(result.quota_info?.percent_used ?? 0).toFixed(1)}%
                      </div>
                    </div>
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Sobregiro Activo</div>
                      <div className={result.quota_info?.is_overdraft ? 'text-fuchsia-400 font-bold' : 'text-slate-400'}>
                        {result.quota_info?.is_overdraft ? `Sí (+${result.quota_info.overdraft_mb} MB)` : 'No'}
                      </div>
                    </div>
                    <div className="p-2 rounded bg-slate-900 border border-slate-800">
                      <div className="text-[10px] text-slate-500 uppercase">Estado General</div>
                      <div className="text-slate-200 font-bold uppercase">
                        {result.quota_info?.status_level || 'normal'}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-3 sm:p-4 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
          <span className="text-[10px] text-slate-500">
            UniTunnel v2.4 · Diagnóstico de Red UCI
          </span>
          <button
            type="button"
            onClick={onClose}
            className="px-3.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 transition text-xs font-medium"
          >
            Cerrar
          </button>
        </div>

      </div>
    </div>
  );
}

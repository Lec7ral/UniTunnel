import React, { useState } from 'react';
import { bridge, AppConfig } from '../services/wailsBridge';

interface ChainingDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  config: AppConfig | null;
  onConfigChange: (newConfig: AppConfig) => void;
}

export const ChainingDrawer: React.FC<ChainingDrawerProps> = ({
  isOpen,
  onClose,
  config,
  onConfigChange,
}) => {
  const [psiphonPort, setPsiphonPort] = useState<number>(config?.chaining_psiphon_port || 10808);
  const [copiedHttp, setCopiedHttp] = useState(false);
  const [copiedSocks, setCopiedSocks] = useState(false);
  const [downloadSuccess, setDownloadSuccess] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  if (!isOpen) return null;

  const httpPort = config?.local_http_port || 8081;
  const socksPort = config?.local_socks_port || 1080;
  const isChainingActive = config?.network_mode === 'chaining';

  const copyToClipboard = (text: string, type: 'http' | 'socks') => {
    navigator.clipboard.writeText(text);
    if (type === 'http') {
      setCopiedHttp(true);
      setTimeout(() => setCopiedHttp(false), 2000);
    } else {
      setCopiedSocks(true);
      setTimeout(() => setCopiedSocks(false), 2000);
    }
  };

  const handleDownloadProfile = async () => {
    try {
      const xml = await bridge.exportProxifierProfile(psiphonPort);
      const blob = new Blob([xml], { type: 'application/xml;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'unitunnel_proxifier.ppx';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      setDownloadSuccess(true);
      setStatusMessage('✓ Perfil .ppx descargado. Ábrelo con Proxifier (File -> Load Profile).');
      setTimeout(() => setDownloadSuccess(false), 4000);
    } catch (err) {
      setStatusMessage('Error generando perfil de Proxifier.');
    }
  };

  const handleActivateChainingMode = async () => {
    try {
      await bridge.applyNetworkMode('chaining');
      if (config) {
        onConfigChange({
          ...config,
          network_mode: 'chaining',
          capture_mode: 'chaining',
          chaining_psiphon_port: psiphonPort,
        });
      }
      setStatusMessage('✓ Modo Encadenamiento activado. Wintun y proxy de sistema desactivados.');
      setTimeout(() => setStatusMessage(null), 3500);
    } catch (err) {
      setStatusMessage('No se pudo aplicar el modo encadenamiento.');
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-sm animate-fade-in">
      <div 
        className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-xl shadow-2xl flex flex-col max-h-[92vh] overflow-hidden"
        role="dialog"
        aria-labelledby="chaining-title"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/90">
          <div>
            <h2 id="chaining-title" className="text-base font-semibold text-slate-100 tracking-tight">
              Encadenamiento con Proxifier y Psiphon
            </h2>
            <div className="flex items-center gap-2 mt-0.5 text-xs text-slate-400">
              <span>Evita bucles de red</span>
              <span aria-hidden="true">·</span>
              <span>Sin colisiones de Wintun</span>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded-lg transition-colors text-sm"
            aria-label="Cerrar modal"
          >
            ✕
          </button>
        </div>

        {/* Content Viewport */}
        <div className="p-6 overflow-y-auto space-y-6 text-sm text-slate-300">
          {statusMessage && (
            <div className="p-3 text-xs bg-cyan-950/60 border border-cyan-800/80 text-cyan-200 rounded-lg">
              {statusMessage}
            </div>
          )}

          {/* Mode Action Banner */}
          <div className="p-4 rounded-xl border border-slate-800 bg-slate-950/60 flex items-center justify-between gap-4">
            <div>
              <div className="font-medium text-slate-200 text-sm">
                Estado del Modo Encadenamiento
              </div>
              <div className="text-xs text-slate-400 mt-0.5">
                {isChainingActive
                  ? 'Activo: Wintun está desactivado. Proxifier y Psiphon tienen control total.'
                  : 'Inactivo: UniTunnel usa Wintun / proxy de sistema.'}
              </div>
            </div>
            <button
              onClick={handleActivateChainingMode}
              className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors whitespace-nowrap shrink-0 ${
                isChainingActive
                  ? 'bg-emerald-950/80 text-emerald-300 border border-emerald-800'
                  : 'bg-cyan-600 hover:bg-cyan-500 text-white shadow-sm'
              }`}
            >
              {isChainingActive ? '✓ Modo Activo' : 'Activar Ahora'}
            </button>
          </div>

          {/* Traffic Flow Pipeline */}
          <div>
            <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Flujo de la Cadena de Tráfico
            </div>
            <div className="p-3.5 rounded-lg border border-slate-800 bg-slate-950/80 space-y-2 text-xs font-mono">
              <div className="flex items-center gap-2">
                <span className="w-5 text-center text-slate-500">1.</span>
                <span className="text-slate-300 font-sans">Aplicaciones (Chrome, juegos, etc.)</span>
                <span className="text-slate-600">→</span>
                <span className="text-cyan-400 font-sans">Proxifier (WFP)</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="w-5 text-center text-slate-500">2.</span>
                <span className="text-slate-300 font-sans">Proxifier filtra:</span>
                <span className="text-emerald-400 font-sans">UCI directo a 1080</span>
                <span className="text-slate-600">·</span>
                <span className="text-amber-400 font-sans">Internet a Psiphon</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="w-5 text-center text-slate-500">3.</span>
                <span className="text-slate-300 font-sans">Psiphon usa UniTunnel:</span>
                <span className="text-cyan-300 font-sans">Upstream HTTP en {httpPort}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="w-5 text-center text-slate-500">4.</span>
                <span className="text-slate-300 font-sans">UniTunnel inyecta NTLMv2</span>
                <span className="text-slate-600">→</span>
                <span className="text-slate-200 font-sans">Squid UCI 10.0.0.1:8080</span>
              </div>
            </div>
          </div>

          {/* Quick Copy Ports */}
          <div>
            <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Puertos Locales de UniTunnel
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="p-3 rounded-lg border border-slate-800 bg-slate-950/40">
                <div className="text-xs text-slate-400">Upstream Proxy (para Psiphon)</div>
                <div className="flex items-center justify-between mt-1">
                  <span className="font-mono text-sm font-semibold text-cyan-400 tabular-nums">
                    127.0.0.1:{httpPort}
                  </span>
                  <button
                    onClick={() => copyToClipboard(`127.0.0.1:${httpPort}`, 'http')}
                    className="px-2 py-1 text-xs text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded transition-colors"
                  >
                    {copiedHttp ? '✓ Copiado' : 'Copiar'}
                  </button>
                </div>
              </div>

              <div className="p-3 rounded-lg border border-slate-800 bg-slate-950/40">
                <div className="text-xs text-slate-400">SOCKS5 Local (para Proxifier)</div>
                <div className="flex items-center justify-between mt-1">
                  <span className="font-mono text-sm font-semibold text-emerald-400 tabular-nums">
                    127.0.0.1:{socksPort}
                  </span>
                  <button
                    onClick={() => copyToClipboard(`127.0.0.1:${socksPort}`, 'socks')}
                    className="px-2 py-1 text-xs text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded transition-colors"
                  >
                    {copiedSocks ? '✓ Copiado' : 'Copiar'}
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* Proxifier Profile Generator */}
          <div className="p-4 rounded-xl border border-slate-800 bg-slate-950/40 space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium text-slate-200">
                  Generador de Perfil para Proxifier (.ppx)
                </div>
                <div className="text-xs text-slate-400 mt-0.5">
                  Preconfigurado con reglas para evitar bucles con <span className="font-mono">unitunnel.exe</span> y <span className="font-mono">10.0.0.1</span>.
                </div>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <label htmlFor="psiphon-port-input" className="text-xs text-slate-400 whitespace-nowrap">
                Puerto SOCKS de Psiphon:
              </label>
              <input
                id="psiphon-port-input"
                type="number"
                value={psiphonPort}
                onChange={(e) => setPsiphonPort(parseInt(e.target.value, 10) || 10808)}
                className="w-24 px-2.5 py-1 text-xs font-mono tabular-nums bg-slate-900 border border-slate-700 rounded text-slate-200 focus:outline-none focus:border-cyan-500"
                min="1024"
                max="65535"
              />
            </div>

            <button
              onClick={handleDownloadProfile}
              className="w-full py-2 px-3 text-xs font-semibold text-slate-100 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              <span>{downloadSuccess ? '✓ Perfil Guardado' : '📥 Descargar Perfil unitunnel_proxifier.ppx'}</span>
            </button>
          </div>

          {/* Quick Setup Guide */}
          <div>
            <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">
              Pasos para conectar Psiphon
            </div>
            <ol className="space-y-2 text-xs text-slate-400 list-decimal list-inside bg-slate-950/40 p-3.5 rounded-lg border border-slate-800">
              <li>Abre <span className="text-slate-200 font-medium">Psiphon</span> y ve a <span className="text-slate-200 font-medium">Ajustes → Proxy aguas arriba (Upstream Proxy)</span>.</li>
              <li>Marca la casilla y configura <span className="font-mono text-cyan-300">Host: 127.0.0.1</span> y <span className="font-mono text-cyan-300">Puerto: {httpPort}</span>.</li>
              <li>En <span className="text-slate-200 font-medium">Autenticación</span> en Psiphon, déjala desmarcada (UniTunnel ya autentica con NTLMv2).</li>
              <li>Conecta Psiphon. Una vez conectado, importa el perfil descargado en <span className="text-slate-200 font-medium">Proxifier</span>.</li>
            </ol>
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-900/90 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-1.5 text-xs font-medium text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors"
          >
            Cerrar
          </button>
        </div>
      </div>
    </div>
  );
};

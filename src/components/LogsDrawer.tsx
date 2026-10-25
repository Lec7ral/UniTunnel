import React, { useState, useEffect, useRef } from 'react';
import {
  X,
  Copy,
  Trash2,
  Terminal,
  Search,
  Check,
  ArrowDown,
  Filter,
} from 'lucide-react';

interface LogsDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  logs: string[];
  onClear: () => void;
}

export const LogsDrawer: React.FC<LogsDrawerProps> = ({
  isOpen,
  onClose,
  logs,
  onClear,
}) => {
  const [filterCategory, setFilterCategory] = useState<string>('todos');
  const [searchQuery, setSearchQuery] = useState('');
  const [copied, setCopied] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isOpen && autoScroll) {
      logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, isOpen, autoScroll]);

  if (!isOpen) return null;

  const handleCopyLogs = () => {
    navigator.clipboard.writeText(logs.join('\n'));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const filteredLogs = logs.filter((line) => {
    const matchesSearch = searchQuery === '' || line.toLowerCase().includes(searchQuery.toLowerCase());
    if (!matchesSearch) return false;

    if (filterCategory === 'todos') return true;
    if (filterCategory === 'red') return line.includes('[RED]') || line.includes('Métrica');
    if (filterCategory === 'tunel') return line.includes('[TÚNEL]') || line.includes('Wintun') || line.includes('SOCKS');
    if (filterCategory === 'cuota') return line.includes('[CUOTA]');
    if (filterCategory === 'auth') return line.includes('[AUTH]') || line.includes('NTLM') || line.includes('AD');
    if (filterCategory === 'error') return line.includes('[ERROR]') || line.includes('Error') || line.includes('407');
    return true;
  });

  const getLogBadge = (line: string) => {
    if (line.includes('[ERROR]')) return { text: 'ERROR', cls: 'bg-red-950 text-red-400 border-red-800/60' };
    if (line.includes('[RED]')) return { text: 'RED', cls: 'bg-cyan-950 text-cyan-400 border-cyan-800/60' };
    if (line.includes('[CUOTA]')) {
      if (line.includes('Sobregiro')) return { text: 'CUOTA', cls: 'bg-fuchsia-950 text-fuchsia-400 border-fuchsia-800/60' };
      return { text: 'CUOTA', cls: 'bg-amber-950 text-amber-400 border-amber-800/60' };
    }
    if (line.includes('[TÚNEL]')) return { text: 'TÚNEL', cls: 'bg-emerald-950 text-emerald-400 border-emerald-800/60' };
    if (line.includes('[AUTH]')) return { text: 'AUTH', cls: 'bg-blue-950 text-blue-400 border-blue-800/60' };
    return { text: 'SISTEMA', cls: 'bg-slate-800 text-slate-300 border-slate-700' };
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-slate-950/70 backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />

      {/* Slide-over panel from RIGHT */}
      <div className="relative z-10 w-full max-w-lg bg-slate-900 border-l border-slate-800 shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-200">
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/90">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-emerald-500/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
              <Terminal className="w-4 h-4" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-sm font-bold text-slate-100">Registro de Eventos</h2>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  {logs.length} eventos
                </span>
              </div>
              <p className="text-[11px] text-slate-400">Túnel, métricas de red y autenticación</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search & Category Filter Bar */}
        <div className="p-3 border-b border-slate-800 bg-slate-950/60 space-y-2.5">
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-2.5" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Buscar en el registro..."
              className="w-full bg-slate-900 border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs font-mono text-slate-200 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
            />
          </div>

          <div className="flex items-center gap-1.5 overflow-x-auto text-[11px] [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
            <Filter className="w-3 h-3 text-slate-500 shrink-0" />
            {[
              { id: 'todos', label: 'Todos' },
              { id: 'red', label: 'Red / Métricas' },
              { id: 'tunel', label: 'Túnel' },
              { id: 'cuota', label: 'Cuota' },
              { id: 'auth', label: 'Auth' },
              { id: 'error', label: 'Errores' },
            ].map((cat) => (
              <button
                key={cat.id}
                onClick={() => setFilterCategory(cat.id)}
                className={`px-2 py-1 rounded font-medium whitespace-nowrap transition ${
                  filterCategory === cat.id
                    ? 'bg-cyan-600 text-white shadow-sm'
                    : 'bg-slate-900 text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                }`}
              >
                {cat.label}
              </button>
            ))}
          </div>
        </div>

        {/* Terminal Output Area */}
        <div className="flex-1 overflow-y-auto p-4 bg-slate-950 font-mono text-[11px] space-y-2 select-text [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
          {filteredLogs.length === 0 ? (
            <div className="text-center py-12 text-slate-600">
              <Terminal className="w-8 h-8 mx-auto mb-2 opacity-40" />
              <p>No hay eventos que coincidan con el filtro.</p>
            </div>
          ) : (
            filteredLogs.map((logLine, idx) => {
              const badge = getLogBadge(logLine);
              const isErr = logLine.includes('[ERROR]');
              return (
                <div
                  key={idx}
                  className={`p-2 rounded border leading-relaxed break-all ${
                    isErr
                      ? 'bg-red-950/20 border-red-900/40 text-red-300'
                      : 'bg-slate-900/60 border-slate-800/80 text-slate-300 hover:border-slate-700'
                  }`}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className={`text-[9px] px-1.5 py-0.2 rounded border font-semibold tracking-wider ${badge.cls}`}>
                      {badge.text}
                    </span>
                    <span className="text-[10px] text-slate-500">
                      {logLine.slice(1, 9)}
                    </span>
                  </div>
                  <div className="text-slate-200">
                    {logLine.replace(/^\[\d{1,2}:\d{2}:\d{2}\]\s*(\[[A-ZÁÉÍÓÚ]+\])?\s*/, '')}
                  </div>
                </div>
              );
            })
          )}
          <div ref={logEndRef} />
        </div>

        {/* Footer Actions */}
        <div className="p-3 border-t border-slate-800 bg-slate-900/90 flex items-center justify-between text-xs">
          <div className="flex items-center gap-2">
            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className={`p-1.5 rounded flex items-center gap-1 text-[11px] transition ${
                autoScroll ? 'bg-cyan-950 text-cyan-400 border border-cyan-800/60' : 'text-slate-400 hover:text-slate-200'
              }`}
              title="Auto-scroll"
            >
              <ArrowDown className="w-3.5 h-3.5" />
              Auto-scroll
            </button>
            <button
              onClick={onClear}
              className="p-1.5 rounded text-slate-400 hover:text-red-400 hover:bg-slate-800 flex items-center gap-1 text-[11px] transition"
              title="Limpiar registro"
            >
              <Trash2 className="w-3.5 h-3.5" />
              Limpiar
            </button>
          </div>

          <button
            onClick={handleCopyLogs}
            className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium text-xs flex items-center gap-1.5 transition"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            {copied ? '¡Copiado!' : 'Copiar Registro'}
          </button>
        </div>
      </div>
    </div>
  );
};

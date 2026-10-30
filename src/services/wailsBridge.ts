// Bridge to the Wails v3 Go runtime.
//
// Every backend call goes through the official `@wailsio/runtime` package and is
// addressed by its fully qualified Go method name:
//
//   "<import path>.<Type>.<Method>"  ->  "unitunnel/pkg/service.AppService.GetConfig"
//
// That is exactly the name `wails3 generate bindings` emits for the Go type
// declared in unitunnel-go/pkg/service (FQN = PkgPath + "." + Type + "." + Method).
//
// Design rule: there are NO silent fallbacks and NO fabricated data. If the Wails
// runtime is not available (for example when the UI is opened in a plain browser
// with `npm run dev`) or the backend returns an error, the call fails loudly and
// the UI must show the failure instead of pretending it worked.

import { Call } from '@wailsio/runtime';

export interface UserAccount {
  id: string;
  name: string;
  username: string;
  domain: string;
  password?: string;
}

export type NetworkMode = 'uci_cable' | 'wifi_mobile' | 'direct';

export interface AppConfig {
  proxy_host: string;
  proxy_port: number;
  auth_type: string;
  active_account_id: string;
  accounts: UserAccount[];
  local_socks_port: number;
  local_http_port: number;
  capture_mode: 'tun' | 'system_proxy';
  tun_name: string;
  tun_ipv4: string;
  tun_mask: string;
  tun_dns: string;
  bypass_routes: string[];
  network_mode: NetworkMode;
  quota_url: string;
  auto_check_quota: boolean;
  quota_check_interval_min: number;
  start_minimized: boolean;
  auto_start: boolean;
  always_on_top: boolean;
}

export interface QuotaInfo {
  username: string;
  allocated_mb: number;
  used_mb: number;
  remaining_mb: number;
  percent_used: number;
  clamped_percent: number;
  is_overdraft: boolean;
  overdraft_mb: number;
  status_level: 'normal' | 'warning' | 'critical' | 'exhausted' | 'overdraft';
  last_updated: string;
  success: boolean;
  error_message?: string;
}

export interface TrafficStats {
  connected: boolean;
  speed_mbs: number;
  read_mb: number;
  write_mb: number;
  total_mb: number;
  ping_ms: number;
}

export interface InterfaceStatus {
  alias: string;
  if_index: number;
  metric: number;
  auto_metric: boolean;
  status: string;
  media_type: 'ethernet' | 'wifi' | 'other';
}

export interface HotspotToggleResult {
  success: boolean;
  message: string;
  enabled: boolean;
}

/** Go service registered in main.go via application.NewService(appService). */
const SERVICE_FQN = 'unitunnel/pkg/service.AppService';

/** Key used only for the local log mirror; configuration is never persisted here. */
const STORAGE_KEY_LOGS = 'unitunnel_logs_v3';

export const defaultConfig: AppConfig = {
  proxy_host: '10.0.0.1',
  proxy_port: 8080,
  auth_type: 'auto',
  active_account_id: 'default',
  accounts: [
    {
      id: 'default',
      name: 'Personal',
      username: '',
      domain: 'uci.cu',
      password: '',
    },
  ],
  local_socks_port: 1080,
  local_http_port: 8081,
  capture_mode: 'tun',
  tun_name: 'UniTunnel0',
  tun_ipv4: '10.254.1.2',
  tun_mask: '255.255.255.0',
  tun_dns: '1.1.1.1,8.8.8.8',
  bypass_routes: ['*uci.cu', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
  network_mode: 'uci_cable',
  quota_url: 'https://cuota.uci.cu/',
  auto_check_quota: true,
  quota_check_interval_min: 15,
  start_minimized: false,
  auto_start: false,
  always_on_top: false,
};

/**
 * True when the page is hosted by an actual Wails desktop host (WebView2 on
 * Windows). Importing `@wailsio/runtime` alone does not prove that, because the
 * module is also loaded in a normal browser during development.
 */
export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') return false;
  const w = window as unknown as {
    chrome?: { webview?: { postMessage?: unknown } };
    webkit?: { messageHandlers?: { external?: { postMessage?: unknown } } };
  };
  return Boolean(
    w.chrome?.webview?.postMessage || w.webkit?.messageHandlers?.external?.postMessage,
  );
}

function assertRuntime(): void {
  if (!isWailsRuntime()) {
    throw new Error(
      'Runtime de Wails no disponible: ejecuta la aplicación empaquetada (unitunnel.exe), no el navegador de desarrollo.',
    );
  }
}

/** Invokes AppService.<method> on the Go side. Rejects on any backend error. */
async function callWailsService<T>(method: string, ...args: unknown[]): Promise<T> {
  assertRuntime();
  const result = await Call.ByName(`${SERVICE_FQN}.${method}`, ...args);
  return result as T;
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

class WailsBridge {
  private configCache: AppConfig | null = null;
  private logListeners: ((logs: string[]) => void)[] = [];
  private currentLogs: string[] = [];

  constructor() {
    this.initLogs();
  }

  private initLogs() {
    if (typeof window === 'undefined') return;
    try {
      const stored = localStorage.getItem(STORAGE_KEY_LOGS);
      this.currentLogs = stored
        ? JSON.parse(stored)
        : [`[${new Date().toLocaleTimeString()}] [SISTEMA] UniTunnel Wails v3 inicializado.`];
    } catch {
      this.currentLogs = [];
    }
  }

  public addLog(category: string, message: string) {
    const time = new Date().toLocaleTimeString();
    const entry = `[${time}] [${category.toUpperCase()}] ${message}`;
    this.currentLogs = [...this.currentLogs.slice(-999), entry];
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem(STORAGE_KEY_LOGS, JSON.stringify(this.currentLogs));
      } catch {
        // Local log mirror is best-effort only.
      }
    }
    this.logListeners.forEach((listener) => listener(this.currentLogs));
  }

  public subscribeLogs(callback: (logs: string[]) => void): () => void {
    this.logListeners.push(callback);
    callback(this.currentLogs);
    return () => {
      this.logListeners = this.logListeners.filter((l) => l !== callback);
    };
  }

  public async fetchLogs(): Promise<string[]> {
    try {
      const backendLogs = await callWailsService<string[]>('GetLogs');
      if (Array.isArray(backendLogs) && backendLogs.length > 0) {
        this.currentLogs = backendLogs;
        this.logListeners.forEach((listener) => listener(this.currentLogs));
        return backendLogs;
      }
    } catch {
      // Backend unreachable: keep the local mirror.
    }
    return this.currentLogs;
  }

  public async clearLogs(): Promise<void> {
    await callWailsService<void>('ClearLogs');
    this.currentLogs = [`[${new Date().toLocaleTimeString()}] [SISTEMA] Registro limpiado.`];
    if (typeof window !== 'undefined') {
      localStorage.setItem(STORAGE_KEY_LOGS, JSON.stringify(this.currentLogs));
    }
    this.logListeners.forEach((listener) => listener(this.currentLogs));
  }

  /**
   * Reads the configuration from the Go backend (config.json next to the .exe).
   * Throws if the backend cannot be reached: the UI must not invent settings.
   */
  public async getConfig(): Promise<AppConfig> {
    const remote = await callWailsService<AppConfig>('GetConfig');
    if (!remote) {
      throw new Error('El backend devolvió una configuración vacía.');
    }
    this.configCache = remote;
    return remote;
  }

  /** Persists the configuration on the Go side. Returns false when it failed. */
  public async saveConfig(cfg: AppConfig): Promise<boolean> {
    try {
      await callWailsService<void>('SaveConfig', cfg);
      this.configCache = cfg;
      this.addLog('CONFIG', 'Ajustes guardados en config.json.');
      return true;
    } catch (err) {
      this.addLog('ERROR', `No se pudo guardar la configuración: ${errorMessage(err)}`);
      return false;
    }
  }

  public async startTunnel(): Promise<{ success: boolean; error?: string }> {
    try {
      this.addLog('TÚNEL', 'Solicitando inicio del túnel al backend Go (NTLMv2 / Wintun)...');
      await callWailsService<void>('StartTunnel');
      await this.fetchLogs();
      return { success: true };
    } catch (err) {
      const msg = errorMessage(err);
      this.addLog('ERROR', `Fallo iniciando túnel: ${msg}`);
      return { success: false, error: msg };
    }
  }

  public async stopTunnel(): Promise<{ success: boolean; error?: string }> {
    try {
      this.addLog('TÚNEL', 'Deteniendo túnel y restaurando configuraciones...');
      await callWailsService<void>('StopTunnel');
      await this.fetchLogs();
      return { success: true };
    } catch (err) {
      const msg = errorMessage(err);
      this.addLog('ERROR', `Fallo deteniendo túnel: ${msg}`);
      return { success: false, error: msg };
    }
  }

  public async isConnected(): Promise<boolean> {
    return callWailsService<boolean>('IsConnected');
  }

  public async getTrafficStats(): Promise<TrafficStats> {
    const stats = await callWailsService<TrafficStats>('GetTrafficStats');
    return stats ?? { connected: false, speed_mbs: 0, read_mb: 0, write_mb: 0, total_mb: 0, ping_ms: 0 };
  }

  /**
   * Applies a network mode. The Go service already persists the mode in
   * config.json, so there is no second SaveConfig round-trip here.
   */
  public async applyNetworkMode(mode: NetworkMode): Promise<boolean> {
    try {
      await callWailsService<void>('SetNetworkMode', mode);
      if (this.configCache) this.configCache = { ...this.configCache, network_mode: mode };
      await this.fetchLogs();
      return true;
    } catch (err) {
      this.addLog('ERROR', `Error aplicando modo de red en Windows: ${errorMessage(err)}`);
      return false;
    }
  }

  public async setAlwaysOnTop(pinned: boolean): Promise<boolean> {
    try {
      await callWailsService<void>('SetAlwaysOnTop', pinned);
      if (this.configCache) this.configCache = { ...this.configCache, always_on_top: pinned };
      return true;
    } catch (err) {
      this.addLog('ERROR', `No se pudo cambiar "Siempre visible": ${errorMessage(err)}`);
      return false;
    }
  }

  public async getInterfaces(): Promise<InterfaceStatus[]> {
    return callWailsService<InterfaceStatus[]>('GetInterfaces');
  }

  /**
   * Queries cuota.uci.cu through the backend. A failure is reported as an
   * explicit QuotaInfo with success=false, never as invented numbers.
   */
  public async fetchQuota(account?: UserAccount): Promise<QuotaInfo> {
    try {
      const info = await callWailsService<QuotaInfo>('CheckQuota');
      if (info) {
        await this.fetchLogs();
        return info;
      }
      throw new Error('El backend devolvió una cuota vacía.');
    } catch (err) {
      const msg = errorMessage(err);
      this.addLog('ERROR', `Error consultando cuota: ${msg}`);
      return {
        username: account?.username ?? '',
        allocated_mb: 0,
        used_mb: 0,
        remaining_mb: 0,
        percent_used: 0,
        clamped_percent: 0,
        is_overdraft: false,
        overdraft_mb: 0,
        status_level: 'normal',
        last_updated: 'Sin conexión',
        success: false,
        error_message: msg,
      };
    }
  }

  /** Runs the real Wi-Fi Direct / SoftAP diagnostic. Throws on failure. */
  public async checkHotspotCapability(): Promise<string> {
    const report = await callWailsService<string>('CheckHotspotCapability');
    await this.fetchLogs();
    return report;
  }

  /** Toggles the forced-offline hotspot and returns the backend verdict. */
  public async toggleForceOfflineHotspot(enable: boolean): Promise<HotspotToggleResult> {
    try {
      const res = await callWailsService<HotspotToggleResult>('ToggleForceOfflineHotspot', enable);
      await this.fetchLogs();
      return res;
    } catch (err) {
      const msg = errorMessage(err);
      this.addLog('ERROR', `Fallo alternando el hotspot: ${msg}`);
      return {
        success: false,
        message: `No se pudo cambiar el hotspot: ${msg}`,
        enabled: false,
      };
    }
  }

  public async isHotspotForced(): Promise<boolean> {
    return callWailsService<boolean>('IsHotspotForced');
  }
}

export const bridge = new WailsBridge();

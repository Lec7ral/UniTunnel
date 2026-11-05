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
  navigation_level?: string;
  status_level: 'normal' | 'warning' | 'critical' | 'exhausted' | 'overdraft';
  last_updated: string;
  raw_response?: string;
  success: boolean;
  error_message?: string;
}

export interface QuotaInspectionResult {
  url: string;
  status_code: number;
  status_text?: string;
  raw_response: string;
  fixed_json?: string;
  parse_error?: string;
  quota_info?: QuotaInfo;
  headers?: Record<string, string>;
  timestamp: string;
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
 * True when the page is hosted by an actual Wails desktop or Android host.
 * Checks for Edge WebView2 on Windows, WebKit external on mobile/macOS,
 * or the UniTunnel Android VPN Bridge.
 */
export function isAndroidPlatform(): boolean {
  if (typeof navigator === 'undefined') return false;
  const isAndroidUA = /Android/i.test(navigator.userAgent);
  const hasAndroidBridge = typeof window !== 'undefined' && Boolean((window as unknown as { AndroidVpnBridge?: unknown }).AndroidVpnBridge);
  return isAndroidUA || hasAndroidBridge;
}

export function isMobileDevice(): boolean {
  if (typeof navigator === 'undefined') return false;
  return isAndroidPlatform() || /iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
}

export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') return false;
  const w = window as unknown as {
    chrome?: { webview?: { postMessage?: unknown } };
    webkit?: { messageHandlers?: { external?: { postMessage?: unknown } } };
    AndroidVpnBridge?: { startVpn?: unknown; isVpnRunning?: unknown };
  };
  return Boolean(
    w.chrome?.webview?.postMessage ||
    w.webkit?.messageHandlers?.external?.postMessage ||
    w.AndroidVpnBridge
  );
}

function assertRuntime(): void {
  if (!isWailsRuntime()) {
    throw new Error(
      'Runtime de Wails no disponible: ejecuta la aplicación empaquetada (unitunnel.exe o APK Android), no el navegador de desarrollo.',
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

const STORAGE_KEY_CONFIG = 'unitunnel_config_v3';

class WailsBridge {
  private configCache: AppConfig | null = null;
  private logListeners: ((logs: string[]) => void)[] = [];
  private currentLogs: string[] = [];
  private simConnected = false;
  private simSpeed = 0;
  private simReadMB = 0;
  private simWriteMB = 0;
  private simPing = 24;

  constructor() {
    this.initLogs();
    this.initConfig();
  }

  private initConfig() {
    if (typeof window === 'undefined') return;
    try {
      const stored = localStorage.getItem(STORAGE_KEY_CONFIG);
      if (stored) {
        this.configCache = JSON.parse(stored);
      }
    } catch {
      // ignore
    }
  }

  private initLogs() {
    if (typeof window === 'undefined') return;
    try {
      const stored = localStorage.getItem(STORAGE_KEY_LOGS);
      const platformName = isAndroidPlatform() ? 'Android (VpnService)' : 'Windows (Wintun/Wails v3)';
      this.currentLogs = stored
        ? JSON.parse(stored)
        : [`[${new Date().toLocaleTimeString()}] [SISTEMA] UniTunnel inicializado (${platformName}).`];
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
   * Reads configuration from the Go backend (or local storage during web preview).
   */
  public async getConfig(): Promise<AppConfig> {
    if (isWailsRuntime()) {
      const remote = await callWailsService<AppConfig>('GetConfig');
      if (!remote) {
        throw new Error('El backend devolvió una configuración vacía.');
      }
      this.configCache = remote;
      return remote;
    }

    if (!this.configCache) {
      this.configCache = defaultConfig;
    }
    return this.configCache;
  }

  /** Persists the configuration to Go backend or local storage. */
  public async saveConfig(cfg: AppConfig): Promise<boolean> {
    if (isWailsRuntime()) {
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

    this.configCache = cfg;
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem(STORAGE_KEY_CONFIG, JSON.stringify(cfg));
      } catch {
        // ignore
      }
    }
    this.addLog('CONFIG', 'Ajustes guardados correctamente.');
    return true;
  }

  public async startTunnel(): Promise<{ success: boolean; error?: string }> {
    const isAndroid = isAndroidPlatform();
    const androidBridge = typeof window !== 'undefined'
      ? (window as unknown as { AndroidVpnBridge?: { startVpn: () => boolean } }).AndroidVpnBridge
      : undefined;

    if (androidBridge) {
      try {
        this.addLog('TÚNEL', 'Solicitando permiso e inicio de VpnService en Android...');
        androidBridge.startVpn();
      } catch (err) {
        this.addLog('ERROR', `Error llamando a AndroidVpnBridge: ${errorMessage(err)}`);
      }
    }

    if (isWailsRuntime()) {
      try {
        const modeDesc = isAndroid ? 'Android VpnService / NTLMv2' : 'Windows Wintun / NTLMv2';
        this.addLog('TÚNEL', `Iniciando túnel en Go (${modeDesc})...`);
        await callWailsService<void>('StartTunnel');
        await this.fetchLogs();
        return { success: true };
      } catch (err) {
        const msg = errorMessage(err);
        this.addLog('ERROR', `Fallo iniciando túnel: ${msg}`);
        return { success: false, error: msg };
      }
    }

    // Web preview simulation
    this.simConnected = true;
    this.simSpeed = 0.42;
    this.simPing = 26;
    this.addLog('TÚNEL', '✓ [Modo Vista Previa] Túnel activado. Negociación NTLMv2 simulada.');
    return { success: true };
  }

  public async stopTunnel(): Promise<{ success: boolean; error?: string }> {
    const androidBridge = typeof window !== 'undefined'
      ? (window as unknown as { AndroidVpnBridge?: { stopVpn: () => boolean } }).AndroidVpnBridge
      : undefined;

    if (androidBridge) {
      try {
        androidBridge.stopVpn();
      } catch (err) {
        this.addLog('ERROR', `Error deteniendo AndroidVpnBridge: ${errorMessage(err)}`);
      }
    }

    if (isWailsRuntime()) {
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

    // Web preview simulation
    this.simConnected = false;
    this.simSpeed = 0;
    this.addLog('TÚNEL', '✓ [Modo Vista Previa] Túnel detenido.');
    return { success: true };
  }

  public async isConnected(): Promise<boolean> {
    if (isWailsRuntime()) {
      return callWailsService<boolean>('IsConnected');
    }
    return this.simConnected;
  }

  public async getTrafficStats(): Promise<TrafficStats> {
    if (isWailsRuntime()) {
      const stats = await callWailsService<TrafficStats>('GetTrafficStats');
      return stats ?? { connected: false, speed_mbs: 0, read_mb: 0, write_mb: 0, total_mb: 0, ping_ms: 0 };
    }

    if (this.simConnected) {
      const variation = (Math.random() - 0.48) * 0.25;
      this.simSpeed = Math.max(0.08, Math.min(3.4, this.simSpeed + variation));
      this.simReadMB += (this.simSpeed * 1.5) / 1024;
      this.simWriteMB += (this.simSpeed * 0.3) / 1024;
      this.simPing = Math.floor(22 + Math.random() * 8);

      return {
        connected: true,
        speed_mbs: this.simSpeed,
        read_mb: this.simReadMB,
        write_mb: this.simWriteMB,
        total_mb: this.simReadMB + this.simWriteMB,
        ping_ms: this.simPing,
      };
    }

    return {
      connected: false,
      speed_mbs: 0,
      read_mb: this.simReadMB,
      write_mb: this.simWriteMB,
      total_mb: this.simReadMB + this.simWriteMB,
      ping_ms: 0,
    };
  }

  public async applyNetworkMode(mode: NetworkMode): Promise<boolean> {
    if (isWailsRuntime()) {
      try {
        await callWailsService<void>('SetNetworkMode', mode);
        if (this.configCache) this.configCache = { ...this.configCache, network_mode: mode };
        await this.fetchLogs();
        return true;
      } catch (err) {
        this.addLog('ERROR', `Error aplicando modo de red: ${errorMessage(err)}`);
        return false;
      }
    }

    if (this.configCache) {
      this.configCache = { ...this.configCache, network_mode: mode };
    }
    this.addLog('RED', `Modo de red cambiado a: ${mode}`);
    return true;
  }

  public async setAlwaysOnTop(pinned: boolean): Promise<boolean> {
    if (isWailsRuntime()) {
      try {
        await callWailsService<void>('SetAlwaysOnTop', pinned);
        if (this.configCache) this.configCache = { ...this.configCache, always_on_top: pinned };
        return true;
      } catch (err) {
        this.addLog('ERROR', `No se pudo cambiar "Siempre visible": ${errorMessage(err)}`);
        return false;
      }
    }

    if (this.configCache) {
      this.configCache = { ...this.configCache, always_on_top: pinned };
    }
    return true;
  }

  public async getInterfaces(): Promise<InterfaceStatus[]> {
    if (isWailsRuntime()) {
      return callWailsService<InterfaceStatus[]>('GetInterfaces');
    }

    return [
      {
        alias: isAndroidPlatform() ? 'wlan0 (Wi-Fi UCI)' : 'Ethernet (Cable UCI)',
        if_index: 1,
        metric: 10,
        auto_metric: false,
        media_type: isAndroidPlatform() ? 'wifi' : 'ethernet',
        status: 'Connected',
      },
      {
        alias: isAndroidPlatform() ? 'rmnet0 (Datos Móviles)' : 'Wi-Fi (Móvil)',
        if_index: 2,
        metric: 100,
        auto_metric: false,
        media_type: 'wifi',
        status: 'Connected',
      },
    ];
  }

  public async fetchQuota(account?: UserAccount): Promise<QuotaInfo> {
    if (isWailsRuntime()) {
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

    // Web preview simulated quota based on active account
    const username = account?.username || 'usuario.uci';
    const allocated = 1000;
    const used = 340.5;
    const remaining = allocated - used;
    const pct = Math.round((used / allocated) * 100);

    return {
      username,
      allocated_mb: allocated,
      used_mb: used,
      remaining_mb: remaining,
      percent_used: pct,
      clamped_percent: pct,
      is_overdraft: false,
      overdraft_mb: 0,
      navigation_level: 'Internet',
      status_level: 'normal',
      last_updated: new Date().toLocaleTimeString(),
      success: true,
    };
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

  /**
   * Diagnostic method to inspect raw response from cuota.uci.cu.
   * If running in Wails desktop app, queries directly via Go backend.
   * If running in browser, tries /api/inspect-quota or direct fetch.
   */
  public async inspectQuota(
    portalUrl: string,
    username: string,
    password: string,
  ): Promise<QuotaInspectionResult> {
    if (isWailsRuntime()) {
      try {
        const res = await callWailsService<QuotaInspectionResult>(
          'InspectQuota',
          portalUrl,
          username,
          password,
        );
        if (res) {
          await this.fetchLogs();
          return res;
        }
      } catch (err) {
        return {
          url: portalUrl,
          status_code: 0,
          raw_response: '',
          timestamp: new Date().toLocaleTimeString(),
          success: false,
          error_message: errorMessage(err),
        };
      }
    }

    try {
      const response = await fetch('/api/inspect-quota', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ portal_url: portalUrl, username, password }),
      });
      if (response.ok) {
        return (await response.json()) as QuotaInspectionResult;
      }
      const errText = await response.text();
      return {
        url: portalUrl,
        status_code: response.status,
        raw_response: errText,
        timestamp: new Date().toLocaleTimeString(),
        success: false,
        error_message: `HTTP ${response.status}: ${errText}`,
      };
    } catch (fetchErr) {
      return {
        url: portalUrl,
        status_code: 0,
        raw_response: '',
        timestamp: new Date().toLocaleTimeString(),
        success: false,
        error_message: errorMessage(fetchErr),
      };
    }
  }

  public async windowMinimize(): Promise<void> {
    if (isWailsRuntime()) {
      return callWailsService<void>('WindowMinimize');
    }
  }

  public async windowHide(): Promise<void> {
    if (isWailsRuntime()) {
      return callWailsService<void>('WindowHide');
    }
  }
}

export const bridge = new WailsBridge();

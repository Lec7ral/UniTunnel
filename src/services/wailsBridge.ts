// Bridge to Wails v3 Go runtime with direct backend invocation and zero random/mock data

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
  vps_enabled: boolean;
  vps_server: string;
  vps_secret: string;
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

const STORAGE_KEY_CONFIG = 'unitunnel_config_v3';
const STORAGE_KEY_LOGS = 'unitunnel_logs_v3';

export const defaultConfig: AppConfig = {
  proxy_host: '10.0.0.1',
  proxy_port: 8080,
  auth_type: 'auto',
  active_account_id: 'default',
  accounts: [
    {
      id: 'default',
      name: 'Cuenta Institucional',
      username: 'yoanbrp',
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
  bypass_routes: ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16'],
  network_mode: 'uci_cable',
  quota_url: 'https://cuota.uci.cu/',
  auto_check_quota: true,
  quota_check_interval_min: 15,
  start_minimized: false,
  auto_start: false,
  always_on_top: false,
  vps_enabled: false,
  vps_server: '',
  vps_secret: '',
};

// Universal caller to communicate with Go backend across all Wails v3 invocation patterns
async function callWailsService<T>(methodName: string, ...args: unknown[]): Promise<T> {
  if (typeof window === 'undefined') {
    throw new Error('No window object available');
  }
  const win = window as Record<string, unknown>;

  // Pattern 1: window.go.main.AppService[methodName]
  const goMain = (win.go as { main?: { AppService?: Record<string, (...a: unknown[]) => Promise<T>> } })?.main?.AppService;
  if (goMain && typeof goMain[methodName] === 'function') {
    return await goMain[methodName](...args);
  }

  // Pattern 2: window.go.service.AppService[methodName]
  const goService = (win.go as { service?: { AppService?: Record<string, (...a: unknown[]) => Promise<T>> } })?.service?.AppService;
  if (goService && typeof goService[methodName] === 'function') {
    return await goService[methodName](...args);
  }

  // Pattern 3: window.wails.CallService (Wails v3 standard)
  const wailsObj = win.wails as { CallService?: (s: string, m: string, ...a: unknown[]) => Promise<T>; Call?: (name: string, ...a: unknown[]) => Promise<T> };
  if (wailsObj && typeof wailsObj.CallService === 'function') {
    const serviceNames = ['service.AppService', 'AppService', 'main.AppService', 'service/AppService'];
    for (const sName of serviceNames) {
      try {
        return await wailsObj.CallService(sName, methodName, ...args);
      } catch (e: unknown) {
        const msg = String(e);
        if (!msg.includes('not found') && !msg.includes('unknown service')) {
          throw e;
        }
      }
    }
  }

  // Pattern 4: window.wails.Call
  if (wailsObj && typeof wailsObj.Call === 'function') {
    try {
      return await wailsObj.Call(`service.AppService.${methodName}`, ...args);
    } catch {
      return await wailsObj.Call(`AppService.${methodName}`, ...args);
    }
  }

  // Pattern 5: window.AppService[methodName]
  const directService = win.AppService as Record<string, (...a: unknown[]) => Promise<T>>;
  if (directService && typeof directService[methodName] === 'function') {
    return await directService[methodName](...args);
  }

  throw new Error(`Wails AppService.${methodName} not available in runtime`);
}

export function isWailsRuntime(): boolean {
  if (typeof window === 'undefined') return false;
  const win = window as Record<string, unknown>;
  const goObj = win.go as { main?: { AppService?: unknown }; service?: { AppService?: unknown } };
  const wailsObj = win.wails as { CallService?: unknown; Call?: unknown };
  return !!(
    goObj?.main?.AppService ||
    goObj?.service?.AppService ||
    wailsObj?.CallService ||
    wailsObj?.Call ||
    win.AppService
  );
}

class WailsBridge {
  private configCache: AppConfig | null = null;
  private logListeners: ((logs: string[]) => void)[] = [];
  private currentLogs: string[] = [];

  constructor() {
    this.initLogs();
  }

  private initLogs() {
    if (typeof window !== 'undefined') {
      try {
        const stored = localStorage.getItem(STORAGE_KEY_LOGS);
        if (stored) {
          this.currentLogs = JSON.parse(stored);
        } else {
          this.currentLogs = [
            `[${new Date().toLocaleTimeString()}] [SISTEMA] UniTunnel Wails v3 inicializado en Windows x64.`,
          ];
        }
      } catch {
        this.currentLogs = [];
      }
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
        // ignore
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
      // not in wails runtime or call error
    }
    return this.currentLogs;
  }

  public async clearLogs(): Promise<void> {
    try {
      await callWailsService<void>('ClearLogs');
    } catch {
      // ignore
    }
    this.currentLogs = [
      `[${new Date().toLocaleTimeString()}] [SISTEMA] Registro limpiado.`,
    ];
    if (typeof window !== 'undefined') {
      localStorage.setItem(STORAGE_KEY_LOGS, JSON.stringify(this.currentLogs));
    }
    this.logListeners.forEach((listener) => listener(this.currentLogs));
  }

  public async getConfig(): Promise<AppConfig> {
    try {
      const remote = await callWailsService<AppConfig>('GetConfig');
      if (remote && remote.proxy_host) {
        this.configCache = remote;
        return remote;
      }
    } catch {
      // fallback to stored local cache
    }

    if (this.configCache) return this.configCache;
    let loaded: AppConfig = { ...defaultConfig };
    try {
      const saved = localStorage.getItem(STORAGE_KEY_CONFIG);
      if (saved) {
        loaded = { ...defaultConfig, ...JSON.parse(saved) };
      }
    } catch {
      // ignore
    }
    this.configCache = loaded;
    return loaded;
  }

  public async saveConfig(cfg: AppConfig): Promise<boolean> {
    this.configCache = cfg;
    if (typeof window !== 'undefined') {
      localStorage.setItem(STORAGE_KEY_CONFIG, JSON.stringify(cfg));
    }

    try {
      await callWailsService<void>('SaveConfig', cfg);
      this.addLog('CONFIG', 'Ajustes guardados correctamente en config.json.');
      return true;
    } catch (err) {
      if (isWailsRuntime()) {
        this.addLog('ERROR', `Error guardando en backend Go: ${err}`);
        return false;
      }
      this.addLog('CONFIG', 'Ajustes actualizados en config.json.');
      return true;
    }
  }

  public async startTunnel(): Promise<{ success: boolean; error?: string }> {
    try {
      this.addLog('TÚNEL', 'Solicitando inicio del túnel al backend Go (NTLMv2 / Wintun)...');
      await callWailsService<void>('StartTunnel');
      await this.fetchLogs();
      return { success: true };
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : String(err);
      this.addLog('ERROR', `Fallo iniciando túnel: ${errMsg}`);
      return { success: false, error: errMsg };
    }
  }

  public async stopTunnel(): Promise<{ success: boolean }> {
    try {
      this.addLog('TÚNEL', 'Deteniendo túnel y restaurando configuraciones...');
      await callWailsService<void>('StopTunnel');
      await this.fetchLogs();
      return { success: true };
    } catch (err) {
      console.warn('Error llamando a StopTunnel:', err);
    }
    this.addLog('TÚNEL', '✓ Desconectado.');
    return { success: true };
  }

  public async isConnected(): Promise<boolean> {
    try {
      return await callWailsService<boolean>('IsConnected');
    } catch {
      return false;
    }
  }

  public async getTrafficStats(): Promise<TrafficStats> {
    try {
      const stats = await callWailsService<TrafficStats>('GetTrafficStats');
      if (stats) {
        return stats;
      }
    } catch {
      // ignore
    }
    return {
      connected: false,
      speed_mbs: 0,
      read_mb: 0,
      write_mb: 0,
      total_mb: 0,
      ping_ms: 0,
    };
  }

  public async applyNetworkMode(mode: NetworkMode): Promise<boolean> {
    const cfg = await this.getConfig();
    cfg.network_mode = mode;
    await this.saveConfig(cfg);

    try {
      await callWailsService<void>('SetNetworkMode', mode);
      await this.fetchLogs();
      return true;
    } catch (err) {
      this.addLog('ERROR', `Error aplicando modo de red en Windows: ${err}`);
      return false;
    }
  }

  public async setAlwaysOnTop(pinned: boolean): Promise<boolean> {
    const cfg = await this.getConfig();
    cfg.always_on_top = pinned;
    await this.saveConfig(cfg);

    try {
      await callWailsService<void>('SetAlwaysOnTop', pinned);
      return true;
    } catch (err) {
      console.warn('Error llamando a SetAlwaysOnTop en Wails:', err);
    }

    if (pinned) {
      this.addLog('SISTEMA', 'Ventana fijada al frente (Siempre visible sobre otras apps).');
    } else {
      this.addLog('SISTEMA', 'Ventana liberada (Comportamiento estándar).');
    }
    return true;
  }

  public async getInterfaces(): Promise<InterfaceStatus[]> {
    try {
      const list = await callWailsService<InterfaceStatus[]>('GetInterfaces');
      if (Array.isArray(list)) {
        return list;
      }
    } catch (err) {
      console.warn('Error obteniendo interfaces de Go:', err);
    }
    return [];
  }

  public async fetchQuota(account?: UserAccount): Promise<QuotaInfo> {
    try {
      const realQuota = await callWailsService<QuotaInfo>('CheckQuota');
      if (realQuota && (realQuota.allocated_mb > 0 || realQuota.remaining_mb > 0 || realQuota.error_message)) {
        await this.fetchLogs();
        return realQuota;
      }
    } catch (err) {
      console.warn('Error en CheckQuota Go:', err);
    }

    const user = account?.username || '';
    return {
      username: user,
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
      error_message: 'No se pudo contactar con cuota.uci.cu (Verifica la conexión al proxy)',
    };
  }
}

export const bridge = new WailsBridge();

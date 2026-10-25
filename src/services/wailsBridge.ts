// Bridge to Wails v3 Go runtime with seamless browser fallback

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
      password: '••••••••••••',
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

function isWailsRuntime(): boolean {
  return typeof window !== 'undefined' && (window as unknown as { go?: { main?: { AppService?: unknown } } }).go?.main?.AppService !== undefined;
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
            `[${new Date().toLocaleTimeString()}] [RED] Modo inicial: Cable UCI (Ethernet métrica 10, Wi-Fi métrica 100).`,
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
        // quota exceeded or ignore
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

  public clearLogs() {
    this.currentLogs = [
      `[${new Date().toLocaleTimeString()}] [SISTEMA] Registro limpiado.`,
    ];
    if (typeof window !== 'undefined') {
      localStorage.setItem(STORAGE_KEY_LOGS, JSON.stringify(this.currentLogs));
    }
    this.logListeners.forEach((listener) => listener(this.currentLogs));
  }

  public async getConfig(): Promise<AppConfig> {
    if (isWailsRuntime()) {
      try {
        const remote = await (window as unknown as { go: { main: { AppService: { GetConfig: () => Promise<AppConfig> } } } }).go.main.AppService.GetConfig();
        this.configCache = remote;
        return remote;
      } catch (err) {
        console.warn('Fallo llamada Wails GetConfig, usando local:', err);
      }
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

    if (isWailsRuntime()) {
      try {
        await (window as unknown as { go: { main: { AppService: { SaveConfig: (c: AppConfig) => Promise<void> } } } }).go.main.AppService.SaveConfig(cfg);
        this.addLog('CONFIG', 'Ajustes guardados en config.json.');
        return true;
      } catch (err) {
        this.addLog('ERROR', `Error guardando en Go: ${err}`);
        return false;
      }
    }

    this.addLog('CONFIG', 'Ajustes actualizados correctamente en config.json.');
    return true;
  }

  public async applyNetworkMode(mode: NetworkMode): Promise<boolean> {
    const cfg = await this.getConfig();
    cfg.network_mode = mode;
    await this.saveConfig(cfg);

    if (isWailsRuntime()) {
      try {
        await (window as unknown as { go: { main: { AppService: { SetNetworkMode: (m: string) => Promise<void> } } } }).go.main.AppService.SetNetworkMode(mode);
        return true;
      } catch (err) {
        this.addLog('ERROR', `Error aplicando modo de red en Windows: ${err}`);
        return false;
      }
    }

    // Simulated Windows PowerShell Execution
    if (mode === 'uci_cable') {
      this.addLog('RED', 'Aplicando PowerShell: Set-NetIPInterface -InterfaceAlias "*Ethernet*" -InterfaceMetric 10');
      this.addLog('RED', 'Aplicando PowerShell: Set-NetIPInterface -InterfaceAlias "*Wi-Fi*" -InterfaceMetric 100');
      this.addLog('RED', '✓ Modo Cable UCI Activado: Prioridad en cable Ethernet universitario.');
    } else if (mode === 'wifi_mobile') {
      this.addLog('RED', 'Aplicando PowerShell: Set-NetIPInterface -InterfaceAlias "*Wi-Fi*" -InterfaceMetric 10');
      this.addLog('RED', 'Aplicando PowerShell: Set-NetIPInterface -InterfaceAlias "*Ethernet*" -InterfaceMetric 100');
      this.addLog('RED', '✓ Modo Wi-Fi / Móvil Activado: Prioridad en Wi-Fi. Puedes navegar con datos de tu teléfono móvil sin desenchufar el cable de red.');
    } else {
      this.addLog('RED', 'Aplicando PowerShell: Set-NetIPInterface -AutomaticMetric Enabled');
      this.addLog('RED', '✓ Modo Directo Activado: Métricas automáticas restauradas en Windows.');
    }
    return true;
  }

  public async setAlwaysOnTop(pinned: boolean): Promise<boolean> {
    const cfg = await this.getConfig();
    cfg.always_on_top = pinned;
    await this.saveConfig(cfg);

    if (isWailsRuntime()) {
      try {
        await (window as unknown as { go: { main: { AppService: { SetAlwaysOnTop: (p: boolean) => Promise<void> } } } }).go.main.AppService.SetAlwaysOnTop(pinned);
        return true;
      } catch (err) {
        console.warn('Error llamando a SetAlwaysOnTop en Wails:', err);
      }
    }

    if (pinned) {
      this.addLog('SISTEMA', 'Ventana fijada al frente (Siempre visible sobre otras apps).');
    } else {
      this.addLog('SISTEMA', 'Ventana desanclada (Profundidad estándar).');
    }
    return true;
  }

  public async getInterfaces(): Promise<InterfaceStatus[]> {
    if (isWailsRuntime()) {
      try {
        return await (window as unknown as { go: { main: { AppService: { GetInterfaces: () => Promise<InterfaceStatus[]> } } } }).go.main.AppService.GetInterfaces();
      } catch (err) {
        console.warn('Error obteniendo interfaces de Go:', err);
      }
    }

    const cfg = await this.getConfig();
    if (cfg.network_mode === 'wifi_mobile') {
      return [
        { alias: 'Wi-Fi (Punto de acceso móvil)', if_index: 12, metric: 10, auto_metric: false, status: 'Connected', media_type: 'wifi' },
        { alias: 'Ethernet (Cable UCI)', if_index: 8, metric: 100, auto_metric: false, status: 'Connected', media_type: 'ethernet' },
      ];
    } else if (cfg.network_mode === 'uci_cable') {
      return [
        { alias: 'Ethernet (Cable UCI)', if_index: 8, metric: 10, auto_metric: false, status: 'Connected', media_type: 'ethernet' },
        { alias: 'Wi-Fi (Inalámbrica)', if_index: 12, metric: 100, auto_metric: false, status: 'Connected', media_type: 'wifi' },
      ];
    }
    return [
      { alias: 'Ethernet (Cable UCI)', if_index: 8, metric: 25, auto_metric: true, status: 'Connected', media_type: 'ethernet' },
      { alias: 'Wi-Fi (Inalámbrica)', if_index: 12, metric: 35, auto_metric: true, status: 'Connected', media_type: 'wifi' },
    ];
  }

  public async fetchQuota(account?: UserAccount): Promise<QuotaInfo> {
    if (isWailsRuntime()) {
      try {
        return await (window as unknown as { go: { main: { AppService: { CheckQuota: () => Promise<QuotaInfo> } } } }).go.main.AppService.CheckQuota();
      } catch (err) {
        console.warn('Error en Wails CheckQuota:', err);
      }
    }

    // High fidelity simulator matching cuota.uci.cu
    const user = account?.username || 'yoanbrp';
    const total = 500;
    // Allow demonstration of normal vs overdraft
    const used = +(340 + Math.random() * 80).toFixed(1);
    const isOverdraft = used > total;
    const overdraftMb = isOverdraft ? +(used - total).toFixed(1) : 0;
    const remaining = isOverdraft ? -overdraftMb : +(total - used).toFixed(1);
    const percent = +((used / total) * 100).toFixed(1);
    const clamped = Math.min(100, Math.max(0, percent));

    const result: QuotaInfo = {
      username: user,
      allocated_mb: total,
      used_mb: used,
      remaining_mb: remaining,
      percent_used: percent,
      clamped_percent: clamped,
      is_overdraft: isOverdraft,
      overdraft_mb: overdraftMb,
      status_level: isOverdraft ? 'overdraft' : percent > 90 ? 'critical' : percent > 70 ? 'warning' : 'normal',
      last_updated: new Date().toLocaleTimeString(),
      success: true,
    };

    if (isOverdraft) {
      this.addLog('CUOTA', `ℹ Saldo cuota: Sobregiro +${overdraftMb} MB (${used} MB de ${total} MB · ${percent}% - Sobregiro permitido).`);
    } else {
      this.addLog('CUOTA', `✓ Saldo cuota: ${remaining} MB restantes de ${total} MB (${percent}% consumido).`);
    }

    return result;
  }

  public async checkHotspotCapability(): Promise<string> {
    if (isWailsRuntime()) {
      try {
        return await (window as unknown as { go: { main: { AppService: { CheckHotspotCapability: () => Promise<string> } } } }).go.main.AppService.CheckHotspotCapability();
      } catch (err) {
        console.warn('Error en Wails CheckHotspotCapability:', err);
      }
    }
    this.addLog('HOTSPOT', 'Análisis Hotspot: Realtek RTL8821CE · Wi-Fi Direct compatible · Requiere adaptador loopback para omitir validación NCSI sin internet.');
    return 'Adaptador Realtek RTL8821CE · Wi-Fi Direct compatible · Requiere adaptador loopback.';
  }
}

export const bridge = new WailsBridge();

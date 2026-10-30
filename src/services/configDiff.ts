import type { AppConfig } from './wailsBridge';

/**
 * Fields the running tunnel reads once, when it is built (Engine, Dialer,
 * Wintun adapter, local listeners). Changing any of them while connected has no
 * effect until the user reconnects, so the UI must say so instead of pretending
 * the new value is already in use.
 */
const RECONNECT_FIELDS: ReadonlyArray<{ field: keyof AppConfig; label: string }> = [
  { field: 'proxy_host', label: 'servidor proxy' },
  { field: 'proxy_port', label: 'puerto del proxy' },
  { field: 'auth_type', label: 'método de autenticación' },
  { field: 'active_account_id', label: 'cuenta activa' },
  { field: 'accounts', label: 'cuentas' },
  { field: 'local_socks_port', label: 'puerto SOCKS5 local' },
  { field: 'local_http_port', label: 'puerto HTTP local' },
  { field: 'capture_mode', label: 'modo de captura' },
  { field: 'tun_name', label: 'nombre del adaptador' },
  { field: 'tun_ipv4', label: 'IP del adaptador' },
  { field: 'tun_mask', label: 'máscara del adaptador' },
  { field: 'tun_dns', label: 'resolvers DNS' },
  { field: 'bypass_routes', label: 'rutas de bypass' },
  { field: 'auto_check_quota', label: 'consulta automática de cuota' },
  { field: 'quota_check_interval_min', label: 'intervalo de consulta de cuota' },
];

function accountsFingerprint(cfg: AppConfig): string {
  return cfg.accounts
    .map((a) => [a.id, a.name, a.username, a.domain, a.password ?? ''].join('\u0000'))
    .join('\u0001');
}

function fieldValue(cfg: AppConfig, field: keyof AppConfig): string {
  if (field === 'accounts') return accountsFingerprint(cfg);
  const value = cfg[field];
  if (Array.isArray(value)) return value.join('\u0000');
  return String(value ?? '');
}

/**
 * Returns the labels of the settings that changed and that the live tunnel will
 * not pick up until the next connection.
 */
export function reconnectReasons(next: AppConfig, current: AppConfig): string[] {
  return RECONNECT_FIELDS.filter(
    ({ field }) => fieldValue(next, field) !== fieldValue(current, field),
  ).map(({ label }) => label);
}

/** True when saving `next` over `current` requires reconnecting to take effect. */
export function requiresReconnect(next: AppConfig, current: AppConfig): boolean {
  return reconnectReasons(next, current).length > 0;
}

/** Builds the user-facing warning for a list of changed settings. */
export function reconnectWarning(reasons: string[]): string {
  if (reasons.length === 0) return '';
  const list = reasons.join(', ');
  return `Cambios guardados, pero el túnel actual sigue usando la configuración anterior (${list}). Reconecta para aplicarlos.`;
}

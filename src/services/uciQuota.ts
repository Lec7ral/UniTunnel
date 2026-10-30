/**
 * UCI Quota parsing and query utilities.
 *
 * Implements the verified protocol used to check UCI internet data quota:
 * 1. Target URL: https://cuota.uci.cu/php/cuota.php
 * 2. Method: POST with application/x-www-form-urlencoded
 * 3. Parameters:
 *    - `username`: Plain text username
 *    - `userpassword`: Base64 encoded password
 * 4. Normalization of PHP loose pseudo-JSON (unquoted keys) via fixToJsonString
 */

export function fixToJsonString(str: string): string {
  // Fix unquoted keys: {key: or , key: -> {"key": or , "key":
  let res = str.replace(/([{,]\s*)([a-zA-Z0-9_]+)\s*:/g, '$1"$2":');
  // Fix single-quoted values: :'foo' -> :"foo"
  res = res.replace(/:\s*'([^']*)'/g, ': "$1"');
  return res.trim();
}

export function encodeBase64Password(password: string): string {
  try {
    // Unicode-safe base64 encoding for browser and Node environments
    return btoa(unescape(encodeURIComponent(password)));
  } catch {
    return btoa(password);
  }
}

export interface UCIQuotaRawResponse {
  success: boolean | string | number;
  data?: unknown;
  msg?: string;
  error?: string;
}

/**
 * Resolves quota portal URL to the actual backend PHP endpoint
 */
export function resolveQuotaEndpoint(portalUrl?: string): string {
  const u = (portalUrl || '').trim();
  if (!u) return 'https://cuota.uci.cu/php/cuota.php';
  if (u.endsWith('.php') || u.includes('cuota.php')) return u;
  return `${u.replace(/\/+$/, '')}/php/cuota.php`;
}

import { existsSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

const DIST_DIR = fileURLToPath(new URL('./unitunnel-go/dist', import.meta.url));

/**
 * `emptyOutDir` wipes unitunnel-go/dist on every build, including the tracked
 * `.gitkeep` that keeps `//go:embed all:dist` (unitunnel-go/main.go) compiling
 * on a clean clone. Recreate it so building the frontend never dirties the tree.
 */
function keepDistPlaceholder(): Plugin {
  return {
    name: 'unitunnel-keep-dist-placeholder',
    closeBundle() {
      const placeholder = join(DIST_DIR, '.gitkeep');
      if (!existsSync(placeholder)) {
        writeFileSync(
          placeholder,
          '# Placeholder so `//go:embed all:dist` always finds a directory.\n',
          'utf8',
        );
      }
    },
  };
}

/** Dev-time API middleware to proxy quota queries from browser without CORS issues */
function quotaInspectPlugin(): Plugin {
  return {
    name: 'unitunnel-quota-inspect-api',
    configureServer(server) {
      server.middlewares.use('/api/inspect-quota', (req, res) => {
        if (req.method !== 'POST') {
          res.statusCode = 405;
          res.end(JSON.stringify({ error: 'Method not allowed' }));
          return;
        }

        let body = '';
        req.on('data', (chunk) => {
          body += chunk;
        });
        req.on('end', async () => {
          try {
            const parsed = JSON.parse(body || '{}');
            const username = parsed.username || '';
            const password = parsed.password || '';
            let portalUrl = parsed.portal_url || 'https://cuota.uci.cu/';
            if (!portalUrl.includes('.php')) {
              portalUrl = portalUrl.replace(/\/+$/, '') + '/php/cuota.php';
            }

            const base64Pass = Buffer.from(password).toString('base64');
            const https = await import('node:https');
            const urlMod = await import('node:url');

            const parsedTarget = new urlMod.URL(portalUrl);
            const postData = new URLSearchParams({
              username: username,
              userpassword: base64Pass,
            }).toString();

            const options = {
              hostname: parsedTarget.hostname,
              port: parsedTarget.port || (parsedTarget.protocol === 'https:' ? 443 : 80),
              path: parsedTarget.pathname + parsedTarget.search,
              method: 'POST',
              headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
                'Content-Length': Buffer.byteLength(postData),
                'User-Agent': 'UniTunnel/2.4 (Vite-Inspector)',
                'Accept': '*/*',
              },
              rejectUnauthorized: false,
              timeout: 12000,
            };

            const client = https.request(options, (upstreamRes) => {
              let resData = '';
              upstreamRes.on('data', (c) => {
                resData += c;
              });
              upstreamRes.on('end', () => {
                const fixedJson = resData
                  .replace(/([{,]\s*)([a-zA-Z0-9_]+)\s*:/g, '$1"$2":')
                  .replace(/:\s*'([^']*)'/g, ': "$1"');
                let parseError = '';
                try {
                  JSON.parse(fixedJson);
                } catch (e: any) {
                  parseError = e?.message || 'JSON parse error';
                }

                res.setHeader('Content-Type', 'application/json');
                res.end(
                  JSON.stringify({
                    url: portalUrl,
                    status_code: upstreamRes.statusCode || 200,
                    status_text: upstreamRes.statusMessage || 'OK',
                    raw_response: resData,
                    fixed_json: fixedJson,
                    parse_error: parseError || undefined,
                    headers: upstreamRes.headers,
                    timestamp: new Date().toLocaleTimeString(),
                    success: true,
                  }),
                );
              });
            });

            client.on('error', (err) => {
              res.setHeader('Content-Type', 'application/json');
              res.end(
                JSON.stringify({
                  url: portalUrl,
                  status_code: 0,
                  status_text: 'Connection Error',
                  raw_response: '',
                  error_message: err.message,
                  timestamp: new Date().toLocaleTimeString(),
                  success: false,
                }),
              );
            });

            client.write(postData);
            client.end();
          } catch (e: any) {
            res.statusCode = 500;
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify({ error: e?.message || 'Internal Server Error' }));
          }
        });
      });
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), keepDistPlaceholder(), quotaInspectPlugin()],
  server: {
    port: 3000,
    host: '0.0.0.0',
  },
  build: {
    // The Go binary embeds unitunnel-go/dist via `//go:embed all:dist`
    // (unitunnel-go/main.go). Building anywhere else produces an .exe with an
    // empty frontend, so this path must stay in sync with that directive.
    outDir: 'unitunnel-go/dist',
    emptyOutDir: true,
    sourcemap: false,
  },
});

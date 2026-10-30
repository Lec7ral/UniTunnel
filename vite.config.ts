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

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), keepDistPlaceholder()],
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

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { execFileSync } from 'node:child_process';

const buildHash = (() => { try { return execFileSync('git', ['rev-parse', '--short=12', 'HEAD'], { encoding: 'utf8' }).trim(); } catch { return 'unknown'; } })();
const buildDate = new Date().toISOString().slice(0, 10);

const sourceFile = fileURLToPath(new URL('./src/game.html', import.meta.url));

function gameModules() {
  return {
    name: 'game-modules',
    resolveId(id) {
      if (id.startsWith('virtual:game-')) return `\0${id}`;
    },
    load(id) {
      if (!id.startsWith('\0virtual:game-')) return;
      this.addWatchFile(sourceFile);
      const source = readFileSync(sourceFile, 'utf8');
      const script = source.match(/<script>([\s\S]*?)<\/script>/)[1];
      const css = source.match(/<style>([\s\S]*?)<\/style>/)[1];
      const shell = source.match(/<body>([\s\S]*?)<script>/)[1];
      if (id.endsWith('style.css')) return css;
      if (id.endsWith('shell')) return `export default ${JSON.stringify(shell)};`;
      if (id.endsWith('controller')) {
        return `export function mountGame(document) {\n${script}\nreturn disposeGame;\n}`;
      }
    },
    handleHotUpdate(ctx) {
      if (ctx.file === sourceFile) {
        ctx.server.ws.send({ type: 'full-reload' });
        return [];
      }
    },
  };
}

export default defineConfig({
  base: '/prisoner-of-war/',
  define: { 'import.meta.env.VITE_BUILD_HASH': JSON.stringify(buildHash), 'import.meta.env.VITE_BUILD_DATE': JSON.stringify(buildDate) },
  plugins: [react(), gameModules()],
  server: {
    proxy: {
      '/prisoner-of-war/api': {
        target: 'http://127.0.0.1:8080',
        rewrite: path => path.replace(/^\/prisoner-of-war/, ''),
      },
      '/prisoner-of-war/skins': {
        target: 'http://127.0.0.1:8080',
        rewrite: path => path.replace(/^\/prisoner-of-war/, ''),
      },
      '/prisoner-of-war/skills/icons': {
        target: 'http://127.0.0.1:8080',
        rewrite: path => path.replace(/^\/prisoner-of-war/, ''),
      },
    },
  },
});

import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// Окно собрано из той же библиотеки, что клиент: $ui и $lib — это web/src/lib,
// шрифты и значки — web/static (план 45).
const web = fileURLToPath(new URL('../../../web', import.meta.url));

export default defineConfig({
	plugins: [svelte()],
	publicDir: `${web}/static`,
	resolve: {
		alias: {
			$ui: `${web}/src/lib/components`,
			$lib: `${web}/src/lib`
		}
	},
	// Свой порт: 5173 — клиент, 5175 — Мастерская. 127.0.0.1 — wails dev ждёт сервер по IPv4.
	server: { host: '127.0.0.1', port: 5178, strictPort: true, fs: { allow: [web, '.'] } },
	build: { outDir: 'dist', emptyOutDir: true }
});

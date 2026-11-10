import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// Окно собрано из той же библиотеки, что клиент: $ui и $lib — это web/src/lib,
// шрифты и значки — web/static (план 45).
const web = fileURLToPath(new URL('../../../web', import.meta.url));

export default defineConfig({
	// Оболочка живёт под /__shell/, сервер — в корне окна.
	base: '/__shell/',
	plugins: [svelte()],
	publicDir: `${web}/static`,
	resolve: {
		alias: {
			'$app/navigation': fileURLToPath(new URL('./src/app-navigation.ts', import.meta.url)),
			$ui: `${web}/src/lib/components`,
			$lib: `${web}/src/lib`
		}
	},
	// Свой порт: 5173 — клиент, 5175 — Мастерская, 5178 — Подпись.
	server: { host: '127.0.0.1', port: 5179, strictPort: true, fs: { allow: [web, '.'] } },
	build: { outDir: 'dist', emptyOutDir: true }
});

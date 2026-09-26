import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { defineConfig } from 'vitest/config';

const root = path.resolve(fileURLToPath(import.meta.url), '..');
const version = readFileSync(path.join(root, '..', 'VERSION'), 'utf8').trim();
// Ревизия index.html в прекэше service worker'а. Workbox перекачивает запись
// только при смене ревизии, а index.html ссылается на хешированные чанки
// своей сборки. Ревизия = VERSION оставляла у установленного PWA старую
// оболочку после обновления из исходников без смены версии: её чанков уже
// нет ни в новом прекэше, ни на сервере, и клиент не загружался (план 42, SW-1).
const shellRevision = `${version}+${Date.now()}`;
const isTest = process.env.VITEST === 'true';

export default defineConfig({
	plugins: [
		sveltekit(),
		SvelteKitPWA({
			// Регистрация от корня, не от текущей страницы: SvelteKit с относительными
			// путями давал ./sw.js, и первый заход по /admin/… или /join/… просил
			// /admin/sw.js — 404, PWA не ставился.
			base: '/',
			scope: '/',
			strategies: 'generateSW',
			registerType: 'autoUpdate',
			manifest: {
				name: 'Wynd',
				short_name: 'Wynd',
				theme_color: '#F4F0E9',
				background_color: '#F4F0E9',
				display: 'standalone',
				start_url: '/',
				lang: 'ru',
				icons: [
					{ src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
					{ src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
					{
						src: '/icon-512.png',
						sizes: '512x512',
						type: 'image/png',
						purpose: 'maskable'
					}
				]
			},
			workbox: {
				navigateFallback: '/index.html',
				navigateFallbackDenylist: [/^\/api/],
				// The plugin's default patterns leave out woff2, so offline would
				// drop back to system fonts.
				globPatterns: ['client/**/*.{js,css,ico,png,svg,webp,webmanifest,woff2}'],
				runtimeCaching: [],
				// Показ пушей: generateSW сам событие push не обрабатывает, и
				// сигнал приходил в пустоту — уведомление не показывалось никому.
				importScripts: ['/push-sw.js']
			},
			integration: {
				// Плагин всегда дописывает шаблон prerendered/**. У SPA пререндеренных
				// страниц нет, и workbox на каждой сборке предупреждал о пустом
				// шаблоне. Шаблон снимается, только пока каталога нет.
				beforeBuildServiceWorker(options) {
					const wb = options.workbox;
					if (!wb.globDirectory || existsSync(path.join(wb.globDirectory, 'prerendered'))) return;
					wb.globPatterns = wb.globPatterns?.filter((g) => !g.startsWith('prerendered/'));
				}
			},
			kit: {
				adapterFallback: 'index.html',
				spa: {
					fallbackRevision: async () => shellRevision
				}
			}
		})
	],
	define: {
		__WYND_VERSION__: JSON.stringify(version)
	},
	resolve: {
		conditions: isTest ? ['browser'] : undefined
	},
	server: {
		host: '127.0.0.1',
		watch: {
			// Avoid reload loops when production build writes web/dist (or stale web/build).
			ignored: ['**/dist/**', '**/build/**']
		},
		proxy: {
			'/api': {
				target: 'http://127.0.0.1:7676',
				changeOrigin: true
			}
		}
	},
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		environment: 'jsdom',
		setupFiles: ['src/test/setup.ts']
	}
});

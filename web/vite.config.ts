import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { sveltekit } from '@sveltejs/kit/vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';
import { defineConfig } from 'vitest/config';

const root = path.resolve(fileURLToPath(import.meta.url), '..');
const version = readFileSync(path.join(root, '..', 'VERSION'), 'utf8').trim();
const isTest = process.env.VITEST === 'true';

export default defineConfig({
	plugins: [
		sveltekit(),
		SvelteKitPWA({
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
				runtimeCaching: []
			},
			kit: {
				adapterFallback: 'index.html',
				spa: {
					fallbackRevision: async () => version
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

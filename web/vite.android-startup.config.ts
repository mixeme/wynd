import { defineConfig } from 'vite';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { compile } from 'svelte/compiler';
import { fileURLToPath } from 'node:url';
import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';

const web = fileURLToPath(new URL('.', import.meta.url));
export default defineConfig({
	root: path.join(web, 'src/android-startup'),
	base: '/startup/',
	publicDir: false,
	plugins: [
		svelte({ configFile: false, preprocess: vitePreprocess() }),
		{
			name: 'wynd-startup-frame',
			resolveId(id) {
				if (id === 'virtual:startup-frame.css') return '\0startup-frame.css';
			},
			load(id) {
				if (id !== '\0startup-frame.css') return;
				// Общая основа страницы и шрифты берутся из действующего layout, без копии оформления.
				const source = readFileSync(path.join(web, 'src/routes/+layout.svelte'), 'utf8');
				const fonts = source.match(/<svelte:head>[\s\S]*?<style>([\s\S]*?)<\/style>/)?.[1];
				if (!fonts) throw new Error('Wynd font baseline missing');
				return (
					fonts + '\n' + compile(source, { filename: 'layout.svelte', css: 'external' }).css!.code
				);
			},
			generateBundle() {
				for (const name of readdirSync(path.join(web, 'static/fonts'))) {
					if (name === 'OFL.txt')
						this.emitFile({
							type: 'asset',
							fileName: `fonts/${name}`,
							source: readFileSync(path.join(web, 'static/fonts', name))
						});
				}
				this.emitFile({
					type: 'asset',
					fileName: 'icons.svg',
					source: readFileSync(path.join(web, 'static/icons.svg'))
				});
			}
		}
	],
	resolve: {
		alias: {
			$ui: path.join(web, 'src/lib/components'),
			$lib: path.join(web, 'src/lib'),
			'/fonts': path.join(web, 'static/fonts'),
			'$app/navigation': path.join(web, 'src/android-startup/navigation.ts')
		}
	},
	build: {
		outDir: path.join(web, '../android/app/build/generated/startup-assets/startup'),
		emptyOutDir: true,
		target: 'es2017'
	}
});

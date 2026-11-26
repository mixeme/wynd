// @ts-nocheck — Node-скрипт сторожа; типы не тянем в svelte-check.
// Сторож клавиатуры (C144): layout viewport не сжимает никто, кроме страницы.
//
// `interactive-widget=resizes-content` просит браузер менять layout viewport
// под клавиатуру. Пока клавиатура уезжает, её место оказывается вне холста
// страницы, и оболочка красит его своим фоном — на телефоне это белая полоса
// на пол-экрана (запись владельца 26.11). Высоту окна считает
// session/viewport.ts по visualViewport; метка возвращает прежнее поведение.
import fs from 'node:fs';
import path from 'node:path';

const FORBIDDEN = /interactive-widget\s*=\s*["']?\s*resizes-content/i;

function htmlFiles(dir) {
	const out = [];
	let entries;
	try {
		entries = fs.readdirSync(dir, { withFileTypes: true });
	} catch {
		return out;
	}
	for (const entry of entries) {
		const abs = path.join(dir, entry.name);
		if (entry.isDirectory()) out.push(...htmlFiles(abs));
		else if (entry.name.endsWith('.html')) out.push(abs);
	}
	return out;
}

function relPosix(root, file) {
	return path.relative(root, file).split(path.sep).join('/');
}

/**
 * Страницы, которые снова просят браузер сжимать layout viewport под клавиатуру.
 * @param {string} webRoot
 * @returns {string[]}
 */
export function checkViewportMeta(webRoot) {
	const hits = [];
	for (const file of htmlFiles(path.join(webRoot, 'src'))) {
		const source = fs.readFileSync(file, 'utf8');
		const rel = relPosix(webRoot, file);
		for (const match of source.matchAll(/<meta\b[^>]*>/gi)) {
			const tag = match[0];
			if (!/name\s*=\s*["']?viewport/i.test(tag)) continue;
			if (!FORBIDDEN.test(tag)) continue;
			const line = source.slice(0, match.index).split('\n').length;
			hits.push(`${rel}:${line}: ${tag.replace(/\s+/g, ' ').trim()}`);
		}
	}
	return hits;
}

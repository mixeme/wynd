// Сторож шапок (C140): структурный класс шапки живёт у одного компонента.
//
// Копия шапки в чужом компоненте расходится с принятой геометрией, а сторож
// экранов (ui-guard) её не видел: он смотрит только `src/routes`, а `$ui`
// считает готовой библиотекой. Так «Кадр» полгода нёс свою копию шапки
// редактора и разошёлся с ним на 12 px (C136). Здесь класс шапки разрешён
// только своему компоненту или слою сборки — chrome/ и layouts/.
import fs from 'node:fs';
import path from 'node:path';
import { markupElements, posixRel, walkSvelte } from './ui-guard.mjs';

/**
 * Владельцы структурных классов шапки. `files` — ровно один компонент,
 * `layers` — слой сборки (там шапка и собирается).
 */
export const CHROME_CLASS_OWNERS = [
	{
		cls: 'cbar',
		layers: ['src/lib/components/chrome/', 'src/lib/layouts/'],
		why: 'цветная шапка — только chrome/ или layout'
	},
	{
		cls: 'compose-top',
		files: ['src/lib/components/chrome/ComposeHeader.svelte'],
		why: 'шапка редактора — только ComposeHeader'
	},
	{
		cls: 'appbar',
		files: ['src/lib/components/chrome/AppBar.svelte'],
		why: 'шапка улочки — только AppBar'
	},
	{
		cls: 'idn',
		files: ['src/lib/components/chrome/CircleBar.svelte'],
		why: 'имя в шапке круга — только CircleBar'
	},
	{
		cls: 'idn-t',
		files: ['src/lib/components/chrome/CircleBar.svelte'],
		why: 'имя в шапке круга — только CircleBar'
	},
	{
		cls: 'bar-act',
		files: ['src/lib/components/chrome/CircleBar.svelte'],
		why: 'действие в шапке круга — только CircleBar'
	},
	{
		cls: 'top-back',
		files: ['src/lib/layouts/FormLayout.svelte'],
		why: 'стрелка шапки с названием — только FormLayout'
	}
];

function markupOf(source) {
	const blank = (text) => text.replace(/[^\n]/g, ' ');
	const markup = source
		.replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, blank)
		.replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, blank);
	return markup.replace(/<!--[\s\S]*?-->/g, blank);
}

function lineAt(source, index) {
	let line = 1;
	for (let i = 0; i < index && i < source.length; i++) {
		if (source.charCodeAt(i) === 10) line++;
	}
	return line;
}

function prodScreens(webRoot) {
	return walkSvelte(path.join(webRoot, 'src', 'routes'))
		.map((file) => ({ file, rel: posixRel(webRoot, file) }))
		.filter(({ rel }) => rel && !rel.includes('/dev/'));
}

/**
 * Классы шапки вне своего хозяина — в библиотеке и боевых экранах.
 * @param {string} webRoot
 * @param {Array<{cls: string, files?: string[], layers?: string[], why: string}>} [owners]
 * @returns {string[]}
 */
export function checkChromeOwners(webRoot, owners = CHROME_CLASS_OWNERS) {
	const hits = [];
	const files = [
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'components')),
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'layouts')),
		...prodScreens(webRoot).map(({ file }) => file)
	];
	for (const file of files) {
		const rel = posixRel(webRoot, file);
		if (!rel) continue;
		const source = fs.readFileSync(file, 'utf8');
		const markup = markupOf(source);
		const lines = source.split('\n');
		for (const el of markupElements(markup)) {
			for (const cls of el.classes) {
				const rule = owners.find((r) => r.cls === cls);
				if (!rule) continue;
				if (rule.files?.includes(rel)) continue;
				if (rule.layers?.some((layer) => rel.startsWith(layer))) continue;
				const line = lineAt(markup, el.index);
				hits.push(`${rel}:${line}:${(lines[line - 1] ?? '').trim()} — .${cls}: ${rule.why}`);
			}
		}
	}
	return hits;
}

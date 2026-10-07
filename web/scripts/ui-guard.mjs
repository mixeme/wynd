// @ts-nocheck — Node-скрипт сторожа; типы не тянем в svelte-check.
import fs from 'node:fs';
import path from 'node:path';

const ROUTE_SVELTE = new Set(['+page.svelte', '+layout.svelte', '+error.svelte']);
const IMPORT_RE = /import\s+(?:type\s+)?([\s\S]*?)\s+from\s+['"]([^'"]+)['"]/g;
const TAG_RE = /<([A-Z][A-Za-z0-9]*)(?:\.[A-Z][A-Za-z0-9]*)*\b/g;
// import('….svelte') и import.meta.glob('….svelte') — с любым видом кавычек.
const DYNAMIC_SVELTE_IMPORT_RE =
	/\bimport(?:\.meta\.glob(?:Eager)?)?\s*\(\s*\[?\s*(['"`])[^'"`]*\.svelte\1/g;
const SVELTE_COMPONENT_RE = /<svelte:component\b/g;

export function posixRel(from, to) {
	const rel = path.relative(from, to);
	if (!rel || rel.startsWith('..') || path.isAbsolute(rel)) return null;
	return rel.replaceAll('\\', '/');
}

export function walkSvelte(dir, acc = []) {
	if (!fs.existsSync(dir)) return acc;
	for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
		const p = path.join(dir, ent.name);
		if (ent.isDirectory()) walkSvelte(p, acc);
		else if (ent.name.endsWith('.svelte')) acc.push(p);
	}
	return acc;
}

function lineAt(source, index) {
	let line = 1;
	for (let i = 0; i < index && i < source.length; i++) {
		if (source.charCodeAt(i) === 10) line++;
	}
	return line;
}

function hit(rel, line, text) {
	return `${rel}:${line}:${String(text).trim()}`;
}

function stripTagBlocks(source, tag) {
	return source.replace(new RegExp(`<${tag}\\b[^>]*>[\\s\\S]*?<\\/${tag}>`, 'gi'), (block) =>
		block.replace(/[^\n]/g, ' ')
	);
}

function markupOf(source) {
	let markup = stripTagBlocks(source, 'script');
	markup = stripTagBlocks(markup, 'style');
	return markup.replace(/<!--[\s\S]*?-->/g, (block) => block.replace(/[^\n]/g, ' '));
}

function defaultImportName(clause) {
	const trimmed = clause.trim();
	if (!trimmed || trimmed.startsWith('{') || trimmed.startsWith('*')) return null;
	const body = trimmed.startsWith('type ') ? trimmed.slice(5).trim() : trimmed;
	if (body.startsWith('{') || body.startsWith('*')) return null;
	const m = body.match(/^([A-Za-z_$][\w$]*)/);
	return m ? m[1] : null;
}

function isScreenFile(rel) {
	return rel.startsWith('src/routes/') && rel.endsWith('.svelte');
}

function allowedSvelteSpec(spec) {
	return spec.startsWith('$ui/') || spec.startsWith('$lib/layouts/');
}

function resolveLibrarySpec(spec) {
	if (spec.startsWith('$ui/')) return { kind: 'ui', file: spec.slice('$ui/'.length) };
	if (spec.startsWith('$lib/layouts/')) {
		return { kind: 'layout', file: spec.slice('$lib/layouts/'.length) };
	}
	return null;
}

export function libraryInventory(webRoot) {
	const ui = new Set();
	const layouts = new Set();
	for (const file of walkSvelte(path.join(webRoot, 'src', 'lib', 'components'))) {
		const rel = posixRel(path.join(webRoot, 'src', 'lib', 'components'), file);
		if (rel) ui.add(rel);
	}
	for (const file of walkSvelte(path.join(webRoot, 'src', 'lib', 'layouts'))) {
		const rel = posixRel(path.join(webRoot, 'src', 'lib', 'layouts'), file);
		if (rel) layouts.add(rel);
	}
	return { ui, layouts };
}

export const UI_GAP_PLAN_HINT = [
	'If existing $ui + $lib/layouts objectively cannot assemble the screen, do not invent markup or a one-off .svelte.',
	'Write docs/plans/<slug>.plan.md (Тип: пробел Wynd UI) with why the current library is not enough and a «Добавить в библиотеку» table, then stop.',
	'Extending the library is a separate task. That later task may add a file only if an open gap plan lists its path.'
].join(' ');

const GAP_TYPE_RE = /\*\*Тип:\*\*\s*пробел Wynd UI/i;
const GAP_CLOSED_RE = /\*\*Статус:\*\*\s*закрыт/i;
const GAP_ADD_SECTION_RE = /##\s+Добавить в библиотеку\s*\n([\s\S]*?)(?=\n##\s|$)/i;
const GAP_PATH_RE =
	/(?:\$ui\/|\$lib\/layouts\/|web\/src\/lib\/(?:components|layouts)\/|src\/lib\/(?:components|layouts)\/)[\w-]+(?:\/[\w-]+)*\.svelte/g;

export function toLibraryWebRel(spec) {
	let p = String(spec).replaceAll('\\', '/').replace(/^\/+/, '');
	if (p.startsWith('web/')) p = p.slice(4);
	if (p.startsWith('$ui/')) return `src/lib/components/${p.slice('$ui/'.length)}`;
	if (p.startsWith('$lib/layouts/')) return `src/lib/layouts/${p.slice('$lib/layouts/'.length)}`;
	return p;
}

export function authorizedPathsFromGapPlan(markdown) {
	if (!GAP_TYPE_RE.test(markdown) || GAP_CLOSED_RE.test(markdown)) return [];
	const section = markdown.match(GAP_ADD_SECTION_RE);
	if (!section) return [];
	GAP_PATH_RE.lastIndex = 0;
	const paths = [];
	for (const raw of section[1].matchAll(GAP_PATH_RE)) {
		paths.push(toLibraryWebRel(raw[0]));
	}
	return paths;
}

const GAP_WHY_RE = /##\s+Почему нельзя собрать из имеющихся/i;

export function checkOpenGapPlans(repoRoot) {
	const hits = [];
	const dir = path.join(repoRoot, 'docs', 'plans');
	if (!fs.existsSync(dir)) return hits;
	for (const name of fs.readdirSync(dir)) {
		if (!name.endsWith('.plan.md') || name.includes('.template.')) continue;
		const abs = path.join(dir, name);
		if (!fs.statSync(abs).isFile()) continue;
		const text = fs.readFileSync(abs, 'utf8');
		GAP_TYPE_RE.lastIndex = 0;
		if (!GAP_TYPE_RE.test(text)) continue;
		GAP_CLOSED_RE.lastIndex = 0;
		if (GAP_CLOSED_RE.test(text)) continue;
		const rel = `docs/plans/${name}`;
		GAP_WHY_RE.lastIndex = 0;
		if (!GAP_WHY_RE.test(text)) {
			hits.push(`${rel}:1:gap plan needs ## Почему нельзя собрать из имеющихся`);
		}
		if (authorizedPathsFromGapPlan(text).length === 0) {
			hits.push(
				`${rel}:1:gap plan «Добавить в библиотеку» must list $ui or $lib/layouts .svelte paths`
			);
		}
	}
	return hits;
}

export function findAuthorizingGapPlan(repoRoot, webRelPosix) {
	const dir = path.join(repoRoot, 'docs', 'plans');
	if (!fs.existsSync(dir)) return null;
	const want = toLibraryWebRel(webRelPosix);
	for (const name of fs.readdirSync(dir)) {
		if (!name.endsWith('.plan.md') || name.includes('.template.')) continue;
		const abs = path.join(dir, name);
		if (!fs.statSync(abs).isFile()) continue;
		const text = fs.readFileSync(abs, 'utf8');
		if (authorizedPathsFromGapPlan(text).includes(want)) {
			return `docs/plans/${name}`;
		}
	}
	return null;
}

function denyNewLibrary(kind) {
	const what =
		kind === 'layout'
			? 'Do not create new layout .svelte files in a screen task.'
			: 'Do not create new Wynd UI ($ui) .svelte files in a screen task.';
	return { deny: true, message: `${what} ${UI_GAP_PLAN_HINT}` };
}

export function classifyNewSvelte(webRelPosix, exists, repoRoot) {
	if (!webRelPosix?.endsWith('.svelte')) return null;
	if (webRelPosix.startsWith('src/lib/components/') && !exists) {
		if (repoRoot && findAuthorizingGapPlan(repoRoot, webRelPosix)) return null;
		return denyNewLibrary('ui');
	}
	if (webRelPosix.startsWith('src/lib/layouts/') && !exists) {
		if (repoRoot && findAuthorizingGapPlan(repoRoot, webRelPosix)) return null;
		return denyNewLibrary('layout');
	}
	if (webRelPosix.startsWith('src/routes/')) {
		const base = path.posix.basename(webRelPosix);
		if (!ROUTE_SVELTE.has(base)) {
			return {
				deny: true,
				message: `Route UI belongs in ${[...ROUTE_SVELTE].join(' / ')}. Do not add extra .svelte files under routes. ${UI_GAP_PLAN_HINT}`
			};
		}
	}
	return null;
}

/**
 * @returns {string[]} hits `rel:line:text`
 */
export function analyzeScreenSource(rel, source, inventory) {
	if (!isScreenFile(rel)) return [];
	const hits = [];
	const allowedTags = new Set();

	for (const match of source.matchAll(IMPORT_RE)) {
		const clause = match[1];
		const spec = match[2];
		const line = lineAt(source, match.index);
		const def = defaultImportName(clause);

		if (spec === 'bits-ui' || spec.startsWith('bits-ui/')) {
			hits.push(hit(rel, line, 'bits-ui is library-internal — screens import $ui components, not Bits UI'));
			continue;
		}

		if (def && spec.endsWith('.svelte')) {
			if (!allowedSvelteSpec(spec)) {
				hits.push(
					hit(
						rel,
						line,
						`component import must be $ui/... or $lib/layouts/... (got ${spec})`
					)
				);
				continue;
			}
			const resolved = resolveLibrarySpec(spec);
			if (!resolved) continue;
			const known = resolved.kind === 'ui' ? inventory.ui : inventory.layouts;
			if (!known.has(resolved.file)) {
				hits.push(
					hit(
						rel,
						line,
						`${spec} is not in the Wynd UI library — use an existing component`
					)
				);
				continue;
			}
			allowedTags.add(def);
		}
	}

	// Динамический импорт компонента и <svelte:component> обходят проверку
	// импортов выше: тег берётся из переменной, а не из `import X from` (GUARD-4).
	for (const match of source.matchAll(DYNAMIC_SVELTE_IMPORT_RE)) {
		hits.push(
			hit(
				rel,
				lineAt(source, match.index),
				`dynamic import of a .svelte file (${match[0]}) — import $ui components statically`
			)
		);
	}

	const markup = markupOf(source);
	for (const match of markup.matchAll(SVELTE_COMPONENT_RE)) {
		hits.push(
			hit(
				rel,
				lineAt(markup, match.index),
				'<svelte:component> is not allowed in screens — render an imported $ui component'
			)
		);
	}
	for (const match of markup.matchAll(TAG_RE)) {
		const name = match[1];
		if (allowedTags.has(name)) continue;
		hits.push(
			hit(
				rel,
				lineAt(markup, match.index),
				`<${name}> is not a Wynd UI / layout component imported in this screen`
			)
		);
	}

	return hits;
}

export function analyzeScreenFile(absFile, webRoot, inventory, source) {
	const rel = posixRel(webRoot, absFile);
	if (!rel) return [];
	return analyzeScreenSource(rel, source ?? fs.readFileSync(absFile, 'utf8'), inventory);
}

function searchSource(rel, source, re) {
	const hits = [];
	const lines = source.split('\n');
	for (let i = 0; i < lines.length; i++) {
		re.lastIndex = 0;
		if (re.test(lines[i])) hits.push(hit(rel, i + 1, lines[i]));
	}
	return hits;
}

/**
 * Классы, которые разрешено вешать на <button> внутри $ui. Список нужен,
 * чтобы новый класс кнопки не появлялся молча; вёрстку класса дублировать
 * больше не нужно — её накрывает сброс :where(button) в ui.css (REF-8).
 */
export const BUTTON_LAYOUT_CLASSES = new Set([
	'btn', 'row2', 'r', 'circle-row-action', 'fold', 'att', 'att-play', 'cm', 'rcho', 'addph',
	'send', 'chip', 'inp', 'one', 'add', 'di', 'pic', 'scrim', 'pay-banner-main',
	'pay-reminder', 'cell', 'thumb', 'thumb-body', 'map-sheet', 'fab-menu-item', 'audio-bar-main',
	'voice', 'vrec-shutter', 'vrec-send', 'dayc'
]);

export const BUTTON_TEXT_CLASSES = new Set(['act', 't', 'rt', 'under', 'done', 'sq', 'mini', 'preview', 'vrec-text']);

function stripCssComments(css) {
	return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

/** @returns {Array<{ selectors: string[], declarations: string[] }>} */
export function parseCssRules(css) {
	const stripped = stripCssComments(css);
	const rules = [];
	const re = /([^{}]+)\{([^{}]*)\}/g;
	for (const match of stripped.matchAll(re)) {
		const selectors = match[1]
			.split(',')
			.map((s) => s.trim())
			.filter(Boolean);
		const declarations = match[2]
			.split(';')
			.map((d) => d.trim())
			.filter(Boolean);
		if (selectors.length && declarations.length) rules.push({ selectors, declarations });
	}
	return rules;
}


/**
 * Открывающие теги разметки: имя, позиция и токены классов (GUARD-1).
 *
 * Регулярки по `class="…"` пропускали `class={'btn'}`, шаблонные строки,
 * интерполяции внутри значения и директиву `class:btn`, а `class="btn`
 * ловил только класс в начале атрибута. Здесь тег читается целиком — с
 * учётом кавычек и вложенных `{…}` — и классы собираются из всех написаний:
 * слова значения, строковые литералы выражений, имена директив `class:`.
 * Работает по разметке после `markupOf` — скрипт и стили уже вырезаны.
 * @returns {Array<{ tag: string, index: number, classes: Set<string>, styles: number }>}
 */
export function markupElements(markup) {
	const out = [];
	const tagStart = /<([A-Za-z][\w.:-]*)/g;
	let m;
	while ((m = tagStart.exec(markup))) {
		const end = scanTagEnd(markup, tagStart.lastIndex);
		const attrs = parseAttributes(markup.slice(tagStart.lastIndex, end));
		const classes = new Set();
		let styles = 0;
		for (const { name, value } of attrs) {
			if (name === 'class') {
				for (const token of classTokens(value)) classes.add(token);
			} else if (name.startsWith('class:')) {
				classes.add(name.slice('class:'.length));
			} else if (name === 'style' || name.startsWith('style:')) {
				styles++;
			}
		}
		out.push({ tag: m[1], index: m.index, classes, styles });
		tagStart.lastIndex = end;
	}
	return out;
}

/** Индекс `>`, закрывающего тег, — вне кавычек и фигурных скобок. */
function scanTagEnd(s, i) {
	let depth = 0;
	let quote = '';
	for (; i < s.length; i++) {
		const c = s[i];
		if (quote) {
			if (c === '\\') i++;
			else if (c === quote) quote = '';
			continue;
		}
		if (depth > 0 && (c === '"' || c === "'" || c === '`')) quote = c;
		else if (depth === 0 && (c === '"' || c === "'")) quote = c;
		else if (c === '{') depth++;
		else if (c === '}') depth = Math.max(0, depth - 1);
		else if (c === '>' && depth === 0) return i;
	}
	return s.length;
}

/** Атрибуты тега: имя и сырое значение (без внешних кавычек, `{…}` — как есть). */
function parseAttributes(s) {
	const attrs = [];
	let i = 0;
	while (i < s.length) {
		while (i < s.length && /\s|\//.test(s[i])) i++;
		if (i >= s.length) break;
		if (s[i] === '{') {
			// {...spread} или {name} — классов в них не ищем.
			i = skipBraces(s, i);
			continue;
		}
		const nameStart = i;
		while (i < s.length && !/[\s=>/]/.test(s[i])) i++;
		const name = s.slice(nameStart, i);
		let value = '';
		if (s[i] === '=') {
			i++;
			const c = s[i];
			if (c === '"' || c === "'") {
				const close = s.indexOf(c, i + 1);
				const stop = close === -1 ? s.length : close;
				value = s.slice(i + 1, stop);
				i = stop + 1;
			} else if (c === '{') {
				const stop = skipBraces(s, i);
				value = s.slice(i, stop);
				i = stop;
			} else {
				const start = i;
				while (i < s.length && !/[\s>]/.test(s[i])) i++;
				value = s.slice(start, i);
			}
		}
		if (name) attrs.push({ name, value });
		else i++;
	}
	return attrs;
}

/** Позиция сразу за `{…}`, начинающимся в i, с учётом строк внутри. */
function skipBraces(s, i) {
	let depth = 0;
	let quote = '';
	for (; i < s.length; i++) {
		const c = s[i];
		if (quote) {
			if (c === '\\') i++;
			else if (c === quote) quote = '';
			continue;
		}
		if (c === '"' || c === "'" || c === '`') quote = c;
		else if (c === '{') depth++;
		else if (c === '}' && --depth === 0) return i + 1;
	}
	return s.length;
}

const STRING_LITERAL_RE = /'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)"|`((?:[^`\\]|\\.)*)`/g;

/**
 * Тексты строковых литералов выражения, включая литералы внутри `${…}`
 * шаблонных строк: `pad ${big ? "btn" : ""}` даёт и «pad», и «btn».
 */
function literalTexts(expr) {
	const out = [];
	for (const lit of expr.matchAll(STRING_LITERAL_RE)) {
		if (lit[3] === undefined) {
			out.push(lit[1] ?? lit[2] ?? '');
			continue;
		}
		out.push(lit[3].replace(/\$\{[^}]*\}/g, ' '));
		for (const inner of lit[3].matchAll(/\$\{([^}]*)\}/g)) out.push(...literalTexts(inner[1]));
	}
	return out;
}

/** Слова класса: вне `{…}` — как есть, внутри — из строковых литералов. */
export function classTokens(value) {
	const tokens = [];
	let plain = '';
	let i = 0;
	while (i < value.length) {
		if (value[i] === '{') {
			const stop = skipBraces(value, i);
			const expr = value.slice(i + 1, stop - 1);
			for (const text of literalTexts(expr)) tokens.push(...text.split(/\s+/));
			plain += ' ';
			i = stop;
			continue;
		}
		plain += value[i++];
	}
	tokens.push(...plain.split(/\s+/));
	return tokens.filter((t) => /^[\w-]+$/.test(t));
}

/** Сырые классы в экранах: тег (или `*` — любой), классы, только боевые экраны. */
export const RAW_CLASS_RULES = [
	{ key: 'rawBtn', tag: '*', classes: ['btn'], prodOnly: false },
	{ key: 'rawFldInput', tag: 'input', classes: ['fld'], prodOnly: false },
	{ key: 'rawTextarea', tag: 'textarea', classes: ['fld', 'ta'], prodOnly: false },
	{ key: 'rawLab', tag: '*', classes: ['lab'], prodOnly: false },
	{
		key: 'rawRouteButtons',
		tag: 'button',
		classes: ['row2', 'rcho', 'one', 'add', 'cm', 'att', 'act', 'compose-text'],
		prodOnly: true
	},
	{ key: 'rawRouteDivRow2', tag: 'div', classes: ['row2'], prodOnly: true },
	{ key: 'rawRouteComposeText', tag: '*', classes: ['compose-text'], prodOnly: true }
];

/**
 * Нарушения RAW_CLASS_RULES в одном экране, по ключу правила.
 * @returns {Record<string, string[]>}
 */
export function rawClassHits(rel, source) {
	const markup = markupOf(source);
	const lines = source.split('\n');
	const prod = !rel.includes('/dev/');
	/** @type {Record<string, string[]>} */
	const out = {};
	for (const rule of RAW_CLASS_RULES) out[rule.key] = [];
	for (const el of markupElements(markup)) {
		for (const rule of RAW_CLASS_RULES) {
			if (rule.prodOnly && !prod) continue;
			if (rule.tag !== '*' && rule.tag !== el.tag) continue;
			if (!rule.classes.some((c) => el.classes.has(c))) continue;
			const line = lineAt(markup, el.index);
			out[rule.key].push(hit(rel, line, lines[line - 1] ?? ''));
		}
	}
	return out;
}

/**
 * @returns {string[]}
 */
export function collectUiButtonClasses(webRoot) {
	const classes = new Set();
	const re = /<button\b[^>]*\bclass="([^"]+)"/g;
	for (const file of walkSvelte(path.join(webRoot, 'src', 'lib', 'components'))) {
		const source = fs.readFileSync(file, 'utf8');
		for (const match of source.matchAll(re)) {
			for (const token of match[1].split(/\s+/)) {
				if (token && !token.includes('{')) classes.add(token);
			}
		}
	}
	return [...classes].sort();
}

/**
 * @returns {string[]}
 */
export function checkUnknownUiButtonClasses(webRoot) {
	const known = new Set([
		...BUTTON_LAYOUT_CLASSES,
		...BUTTON_TEXT_CLASSES,
		'ib',
		'sw',
		'nav',
		'prev',
		'next',
		'idn',
		'done'
	]);
	const hits = [];
	for (const className of collectUiButtonClasses(webRoot)) {
		if (known.has(className)) continue;
		hits.push(
			`${className}: <button class="${className}"> in $ui — add it to BUTTON_LAYOUT_CLASSES or BUTTON_TEXT_CLASSES in ui-guard.mjs`
		);
	}
	return hits;
}

/**
 * Храповик на инлайн-стили (GUI-4). Правило «экран — только $ui, без своих
 * <style>» выдавило вёрстку в сотни `style="…"`. Разом их не убрать, но и
 * расти им больше нельзя: в `inline-style-budget.json` записано, сколько их
 * сейчас в каждом экране. Больше — ошибка; меньше — тоже ошибка, но с
 * просьбой записать новое число: так счётчик и ходит только вниз.
 *
 * Служебные классы для замены лежат в конце `ui.css` (.mt-*, .gutter, .grow).
 * @returns {string[]}
 */
export function checkInlineStyleBudget(webRoot) {
	const budgetPath = path.join(webRoot, 'scripts', 'inline-style-budget.json');
	if (!fs.existsSync(budgetPath)) return ['inline-style-budget.json missing'];
	/** @type {Record<string, number>} */
	const budget = JSON.parse(fs.readFileSync(budgetPath, 'utf8'));
	const hits = [];
	const seen = new Set();

	for (const file of walkSvelte(path.join(webRoot, 'src', 'routes'))) {
		const rel = posixRel(webRoot, file);
		if (!rel || !rel.endsWith('.svelte') || rel.includes('/dev/')) continue;
		const source = fs.readFileSync(file, 'utf8');
		// style="…", style={…} и style:prop — все три написания (GUARD-2): раньше
		// считался только первый, и храповик обходился переписыванием в {…}.
		const count = markupElements(markupOf(source)).reduce((n, el) => n + el.styles, 0);
		const allowed = budget[rel] ?? 0;
		seen.add(rel);
		if (count > allowed) {
			hits.push(
				`${rel}: инлайн-стилей ${count}, в бюджете ${allowed} — служебный класс в ui.css вместо style=`
			);
			continue;
		}
		if (count < allowed) {
			hits.push(`${rel}: инлайн-стилей ${count} — запишите это число в inline-style-budget.json`);
		}
	}
	for (const rel of Object.keys(budget)) {
		if (!seen.has(rel)) hits.push(`${rel}: экрана нет — уберите строку из inline-style-budget.json`);
	}
	return hits;
}

/**
 * Экраны, у которых есть свой `<style>` (GUARD-3). Новые не заводятся:
 * вёрстка экрана — это $ui и служебные классы ui.css. Список ходит только
 * вниз: убрали `<style>` из экрана — уберите и строку отсюда.
 * Экраны `/dev/` не проверяются.
 */
export const STYLE_BLOCK_SCREENS = new Set([
	'src/routes/+layout.svelte',
	'src/routes/+page.svelte',
	'src/routes/admin/pay/+page.svelte',
	'src/routes/admin/pay/donate/+page.svelte',
	'src/routes/admin/pay/requests/[id]/+page.svelte',
	'src/routes/admin/pay/subscription/+page.svelte'
]);

const STYLE_BLOCK_RE = /<style\b/i;

/**
 * Есть ли у экрана свой `<style>` — вне комментариев разметки и скрипта.
 */
export function hasStyleBlock(source) {
	let text = stripTagBlocks(source, 'script');
	text = text.replace(/<!--[\s\S]*?-->/g, ' ');
	return STYLE_BLOCK_RE.test(text);
}

/**
 * @returns {string[]}
 */
export function checkStyleBlocks(webRoot, allowed = STYLE_BLOCK_SCREENS) {
	const hits = [];
	const seen = new Set();
	for (const file of walkSvelte(path.join(webRoot, 'src', 'routes'))) {
		const rel = posixRel(webRoot, file);
		if (!rel || rel.includes('/dev/')) continue;
		if (!hasStyleBlock(fs.readFileSync(file, 'utf8'))) continue;
		seen.add(rel);
		if (!allowed.has(rel)) {
			hits.push(`${rel}: свой <style> в экране — служебный класс в ui.css или компонент $ui`);
		}
	}
	for (const rel of allowed) {
		if (!seen.has(rel)) {
			hits.push(`${rel}: <style> в экране больше нет — уберите строку из STYLE_BLOCK_SCREENS`);
		}
	}
	return hits;
}

/**
 * Классы компонентов Wynd UI на голом HTML-теге боевого экрана (план 47,
 * сторож п. 1). Компонент есть — экран зовёт его, а не верстает класс сам.
 * BANNED не встречаются нигде (с 0.18.28 — все). RATCHET — на случай, если
 * класс придётся оставить до своего компонента: экран → классы, только вниз.
 */
export const LIBRARY_CLASS_BANNED = [
	'h1s',
	'att',
	'men',
	'codebox',
	'addph',
	'sfield',
	'panel',
	'tm',
	'hint',
	'qr',
	'chk',
	'danger',
	'pic'
];
/** @type {Record<string, string[]>} */
export const LIBRARY_CLASS_RATCHET = {};
/** `{@html}` в экранах: не бывает — QR рисует `QrCode` (0.18.15). */
/** @type {Set<string>} */
export const HTML_TAG_SCREENS = new Set();

function prodScreens(webRoot) {
	return walkSvelte(path.join(webRoot, 'src', 'routes'))
		.map((file) => ({ file, rel: posixRel(webRoot, file) }))
		.filter(({ rel }) => rel && !rel.includes('/dev/'));
}

/**
 * @param {string} webRoot
 * @param {string[]} [banned]
 * @param {Record<string, string[]>} [ratchet]
 * @param {Set<string>} [htmlScreens]
 * @returns {string[]}
 */
export function checkLibraryClasses(
	webRoot,
	banned = LIBRARY_CLASS_BANNED,
	ratchet = LIBRARY_CLASS_RATCHET,
	htmlScreens = HTML_TAG_SCREENS
) {
	const watched = new Set([...banned, ...Object.values(ratchet).flat()]);
	const hits = [];
	const seen = new Set();
	const seenHtml = new Set();
	for (const { file, rel } of prodScreens(webRoot)) {
		const source = fs.readFileSync(file, 'utf8');
		const markup = markupOf(source);
		const lines = source.split('\n');
		const allowed = new Set(ratchet[rel] ?? []);
		for (const el of markupElements(markup)) {
			if (!/^[a-z]/.test(el.tag)) continue;
			for (const cls of el.classes) {
				if (!watched.has(cls)) continue;
				const line = lineAt(markup, el.index);
				if (allowed.has(cls)) {
					seen.add(`${rel}|${cls}`);
					continue;
				}
				hits.push(
					`${hit(rel, line, lines[line - 1] ?? '')} — .${cls} на <${el.tag}>: есть компонент $ui`
				);
			}
		}
		// Выбор файлов — FilePicker: у сырого поля каждый экран забывал сброс.
		for (const m of markup.matchAll(/<input\b[^>]*\btype="file"/g)) {
			const line = lineAt(markup, m.index ?? 0);
			hits.push(`${hit(rel, line, lines[line - 1] ?? '')} — <input type="file">: есть FilePicker`);
		}
		if (/\{@html\b/.test(markup)) {
			seenHtml.add(rel);
			if (!htmlScreens.has(rel)) hits.push(`${rel}: {@html} в экране — компонент $ui вместо разметки строкой`);
		}
	}
	for (const [rel, classes] of Object.entries(ratchet)) {
		for (const cls of classes) {
			if (!seen.has(`${rel}|${cls}`)) {
				hits.push(`${rel}: .${cls} больше нет — уберите из LIBRARY_CLASS_RATCHET`);
			}
		}
	}
	for (const rel of htmlScreens) {
		if (!seenHtml.has(rel)) hits.push(`${rel}: {@html} больше нет — уберите из HTML_TAG_SCREENS`);
	}
	return hits;
}

/**
 * Каждый файл библиотеки (`$ui`, `$lib/layouts`) назван в справочнике
 * `docs/reference/ui-components.md` или в открытом плане пробела (план 47,
 * сторож п. 2): иначе компонент заводится мимо правила «сначала план».
 * @returns {string[]}
 */
export function checkLibraryRegistry(webRoot) {
	const repoRoot = path.resolve(webRoot, '..');
	const refPath = path.join(repoRoot, 'docs', 'reference', 'ui-components.md');
	if (!fs.existsSync(refPath)) return ['docs/reference/ui-components.md missing'];
	const reference = fs.readFileSync(refPath, 'utf8');
	const hits = [];
	const files = [
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'components')),
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'layouts'))
	];
	for (const file of files) {
		const rel = posixRel(webRoot, file);
		const name = path.basename(file, '.svelte');
		if (new RegExp(`(^|[^A-Za-z0-9])${name}([^A-Za-z0-9]|$)`).test(reference)) continue;
		if (findAuthorizingGapPlan(repoRoot, rel)) continue;
		hits.push(`${rel}: ${name} нет в docs/reference/ui-components.md и ни в одном открытом плане пробела`);
	}
	return hits;
}

/** Простые классы ui.css: селектор ровно `.name`. */
function simpleCssClasses(css) {
	const out = new Set();
	for (const rule of parseCssRules(css)) {
		for (const sel of rule.selectors) {
			const m = sel.match(/^\.([\w-]+)$/);
			if (m) out.add(m[1]);
		}
	}
	return out;
}

/**
 * Строки классов в `<script>` экрана (план 47, сторож п. 4): `const row =
 * 'flex-mid gap-10'`, потом `class={row}` — сторож классов таких токенов не
 * видит. Строка из двух и больше слов, где каждое — класс ui.css, — нарушение.
 * @returns {string[]}
 */
export function checkScriptClassStrings(webRoot) {
	const css = fs.readFileSync(path.join(webRoot, 'src', 'lib', 'styles', 'ui.css'), 'utf8');
	const classes = simpleCssClasses(css);
	const hits = [];
	for (const { file, rel } of prodScreens(webRoot)) {
		const source = fs.readFileSync(file, 'utf8');
		const scripts = source.match(/<script\b[^>]*>[\s\S]*?<\/script>/gi) ?? [];
		for (const block of scripts) {
			const offset = source.indexOf(block);
			for (const m of block.matchAll(STRING_LITERAL_RE)) {
				const text = m[1] ?? m[2] ?? m[3] ?? '';
				const tokens = text.trim().split(/\s+/);
				if (tokens.length < 2 || !tokens.every((t) => classes.has(t))) continue;
				const line = lineAt(source, offset + (m.index ?? 0));
				hits.push(`${rel}:${line}: '${text.trim()}' — классы пишите в разметке, не в переменной`);
			}
		}
	}
	return hits;
}

/**
 * Классы ui.css (выше служебных), которые встречаются в одном экране и ни в
 * одном компоненте (план 47, сторож п. 5): вёрстка экрана, живущая в
 * глобальном файле. Новые не заводятся — список только вниз.
 */
// План 47 закрыт: классов одного экрана не осталось — новые сразу в $ui.
export const SINGLE_SCREEN_CLASSES = new Set(/** @type {string[]} */ ([]));

const UTILITY_MARKER = '/* Служебные классы:';

function markupClassTokens(source) {
	const out = new Set();
	for (const el of markupElements(markupOf(source))) for (const c of el.classes) out.add(c);
	return out;
}

/**
 * @returns {string[]}
 */
export function checkSingleScreenClasses(webRoot, allowed = SINGLE_SCREEN_CLASSES) {
	const css = fs.readFileSync(path.join(webRoot, 'src', 'lib', 'styles', 'ui.css'), 'utf8');
	const cut = css.indexOf(UTILITY_MARKER);
	const defined = new Set();
	for (const rule of parseCssRules(cut >= 0 ? css.slice(0, cut) : css)) {
		for (const sel of rule.selectors) {
			for (const m of sel.matchAll(/\.([A-Za-z_][\w-]*)/g)) defined.add(m[1]);
		}
	}
	const libTokens = new Set();
	const libFiles = [
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'components')),
		...walkSvelte(path.join(webRoot, 'src', 'lib', 'layouts'))
	];
	for (const file of libFiles) {
		const source = fs.readFileSync(file, 'utf8');
		for (const c of markupClassTokens(source)) libTokens.add(c);
		for (const m of source.matchAll(STRING_LITERAL_RE)) {
			for (const t of (m[1] ?? m[2] ?? m[3] ?? '').split(/\s+/)) if (t) libTokens.add(t);
		}
	}
	/** @type {Map<string, Set<string>>} */
	const usage = new Map();
	for (const { file, rel } of prodScreens(webRoot)) {
		for (const c of markupClassTokens(fs.readFileSync(file, 'utf8'))) {
			if (!defined.has(c) || libTokens.has(c)) continue;
			if (!usage.has(c)) usage.set(c, new Set());
			usage.get(c).add(rel);
		}
	}
	const hits = [];
	const single = new Set();
	for (const [cls, screens] of usage) {
		if (screens.size !== 1) continue;
		single.add(cls);
		if (!allowed.has(cls)) {
			hits.push(
				`${[...screens][0]}: .${cls} в ui.css нужен одному экрану — компонент $ui или служебный класс`
			);
		}
	}
	for (const cls of allowed) {
		if (!single.has(cls)) hits.push(`.${cls} уже не одноэкранный — уберите из SINGLE_SCREEN_CLASSES`);
	}
	return hits;
}

/**
 * Full check used by `npm run check:ui` and the Cursor stop hook.
 * @returns {{ ok: boolean, groups: Array<{ message: string, hits: string[] }> }}
 */
export function checkProject(webRoot) {
	const routes = path.join(webRoot, 'src', 'routes');
	const srcRoot = path.join(webRoot, 'src');
	if (!fs.existsSync(routes)) {
		return {
			ok: false,
			groups: [{ message: `check-ui: routes dir missing: ${routes}`, hits: [routes] }]
		};
	}

	const srcFiles = walkSvelte(srcRoot);
	const inventory = libraryInventory(webRoot);
	const extraRoute = [];
	const composition = [];
	const roleButton = [];
	const rawBtn = [];
	const rawFldInput = [];
	const rawTextarea = [];
	const rawLab = [];
	const legacyImports = [];
	const rawRouteButtons = [];
	const rawRouteDivRow2 = [];
	const rawRouteComposeText = [];
	const unknownUiButtons = checkUnknownUiButtonClasses(webRoot);
	const inlineStyles = checkInlineStyleBudget(webRoot);
	const styleBlocks = checkStyleBlocks(webRoot);

	for (const file of srcFiles) {
		const rel = posixRel(webRoot, file);
		if (!rel) continue;
		const source = fs.readFileSync(file, 'utf8');
		if (rel.startsWith('src/routes/')) {
			const base = path.posix.basename(rel);
			if (!ROUTE_SVELTE.has(base)) {
				extraRoute.push(hit(rel, 1, `route .svelte must be ${[...ROUTE_SVELTE].join(' / ')}`));
			}
			composition.push(...analyzeScreenSource(rel, source, inventory));
			roleButton.push(...searchSource(rel, source, /role="button"/));
			const raw = rawClassHits(rel, source);
			rawBtn.push(...raw.rawBtn);
			rawFldInput.push(...raw.rawFldInput);
			rawTextarea.push(...raw.rawTextarea);
			rawLab.push(...raw.rawLab);
			rawRouteButtons.push(...raw.rawRouteButtons);
			rawRouteDivRow2.push(...raw.rawRouteDivRow2);
			rawRouteComposeText.push(...raw.rawRouteComposeText);
		}
		legacyImports.push(...searchSource(rel, source, /\$lib\/components/));
	}

	const groups = [
		{
			message:
				'check-ui: extra .svelte under routes — screens are +page/+layout/+error assembled from $ui + layouts.',
			hits: extraRoute
		},
		{
			message:
				'check-ui: screen is not assembled from Wynd UI / layouts only — use existing $ui components.',
			hits: composition
		},
		{
			message: 'check-ui: found role="button" in routes — use Button/Chip/IconButton/etc.',
			hits: roleButton
		},
		{
			message: 'check-ui: found raw class="btn in routes — use Button component.',
			hits: rawBtn
		},
		{
			message: 'check-ui: found raw <input class="fld in routes — use Input component.',
			hits: rawFldInput
		},
		{
			message: 'check-ui: found raw <textarea class="fld|ta in routes — use TextArea component.',
			hits: rawTextarea
		},
		{
			message: 'check-ui: found raw class="lab" in routes — use Label component.',
			hits: rawLab
		},
		{
			message: 'check-ui: use $ui instead of $lib/components',
			hits: legacyImports
		},
		{
			message:
				'check-ui: open Wynd UI gap plan must justify the hole and list files to add (separate library task).',
			hits: checkOpenGapPlans(path.resolve(webRoot, '..'))
		},
		{
			message:
				'check-ui: new <button class="…"> in $ui must be listed in ui-guard.mjs (BUTTON_LAYOUT_CLASSES or BUTTON_TEXT_CLASSES).',
			hits: unknownUiButtons
		},
		{
			message:
				'check-ui: инлайн-стили в экранах — храповик: только вниз (GUI-4, см. служебные классы в конце ui.css).',
			hits: inlineStyles
		},
		{
			message:
				'check-ui: свои <style> в экранах — только у экранов из STYLE_BLOCK_SCREENS (GUARD-3).',
			hits: styleBlocks
		},
		{
			message:
				'check-ui: raw semantic <button class="row2|act|…"> in prod routes — use $ui row/button components.',
			hits: rawRouteButtons
		},
		{
			message:
				'check-ui: raw <div class="row2"> in prod routes — use SettingsRow or other $ui row components.',
			hits: rawRouteDivRow2
		},
		{
			message:
				'check-ui: класс компонента Wynd UI на голом теге или {@html} в экране (план 47, сторож 1).',
			hits: checkLibraryClasses(webRoot)
		},
		{
			message:
				'check-ui: файл библиотеки не назван в справочнике ui-components.md (план 47, сторож 2).',
			hits: checkLibraryRegistry(webRoot)
		},
		{
			message: 'check-ui: строка классов в <script> экрана (план 47, сторож 4).',
			hits: checkScriptClassStrings(webRoot)
		},
		{
			message: 'check-ui: одноэкранный класс в ui.css (план 47, сторож 5).',
			hits: checkSingleScreenClasses(webRoot)
		},
		{
			message:
				'check-ui: raw class="compose-text" in prod routes — use TextArea variant="compose".',
			hits: rawRouteComposeText
		}
	];

	const failed = groups.filter((g) => g.hits.length > 0);
	return { ok: failed.length === 0, groups: failed.length ? failed : groups };
}

export function formatReport(result) {
	const failed = result.groups.filter((g) => g.hits.length > 0);
	if (!failed.length) return 'check-ui: OK';
	return failed.map((g) => `${g.message}\n${g.hits.join('\n')}`).join('\n\n');
}

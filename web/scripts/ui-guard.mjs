// @ts-nocheck — Node-скрипт сторожа; типы не тянем в svelte-check.
import fs from 'node:fs';
import path from 'node:path';

const ROUTE_SVELTE = new Set(['+page.svelte', '+layout.svelte', '+error.svelte']);
const IMPORT_RE = /import\s+(?:type\s+)?([\s\S]*?)\s+from\s+['"]([^'"]+)['"]/g;
const TAG_RE = /<([A-Z][A-Za-z0-9]*)(?:\.[A-Z][A-Za-z0-9]*)*\b/g;

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
	return (
		rel.startsWith('src/routes/') &&
		rel.endsWith('.svelte') &&
		!rel.includes('/dev/spike/')
	);
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

	const markup = markupOf(source);
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

/** Семантические классы на <button>: в ui.css нужен явный button.* (см. ui-components.md). */
export const BUTTON_LAYOUT_SPECS = [
	{ class: 'btn', props: ['width'] },
	{ class: 'row2', props: ['padding'] },
	{ class: 'r', props: ['padding'] },
	{ class: 'circle-row-action', props: ['padding', 'border', 'background'] },
	{ class: 'fold', props: ['padding', 'border', 'background'] },
	{ class: 'att', props: ['padding', 'border'] },
	{ class: 'cm', props: ['padding'] },
	{ class: 'rcho', props: ['border'] },
	{ class: 'addph', props: ['width', 'height'] },
	{ class: 'send', props: ['width'] },
	{ class: 'chip', props: ['padding'] },
	{ class: 'inp', props: ['padding'] },
	{ class: 'one', props: ['padding'], selectorIncludes: '.rx' },
	{ class: 'add', props: ['padding'], selectorIncludes: '.rx' },
	{ class: 'di', props: ['padding'], selectorIncludes: '.danger' }
];

/** Классы TextButton / compose — padding:0 намеренно, не требуют зеркала .класс. */
export const BUTTON_TEXT_CLASSES = new Set(['act', 't', 'rt', 'under', 'done']);

const BUTTON_LAYOUT_CLASS_NAMES = new Set(BUTTON_LAYOUT_SPECS.map((s) => s.class));

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

function selectorUsesButtonClass(selector, className) {
	if (!selector.includes('button')) return false;
	const re = new RegExp(`\\.${className}(?:\\b|[.:{,\\s])`);
	return re.test(selector);
}

function ruleZerosPadding(declarations) {
	return declarations.some((d) => /^padding\s*:\s*0\b/.test(d));
}

/** padding:0 в полноценном button.X (есть width/border/display…) — норма, не сброс. */
function isBareButtonReset(declarations) {
	if (!ruleZerosPadding(declarations)) return false;
	const props = declProps(declarations);
	const layoutAnchors = ['display', 'width', 'height', 'gap', 'border-radius', 'flex'];
	return !layoutAnchors.some((p) => props.has(p));
}

function declProps(declarations) {
	const props = new Set();
	for (const d of declarations) {
		const m = d.match(/^([\w-]+)\s*:/);
		if (m) props.add(m[1]);
	}
	return props;
}

/**
 * Wynd UI фаза 1–3: интерактив на <button> + сброс UA в ui.css. Селектор button.X сильнее .X —
 * у layout-классов вёрстку дублируют в button.X (или .контекст button.X).
 * @returns {string[]}
 */
export function checkButtonCssSync(css) {
	const rules = parseCssRules(css);
	const hits = [];

	for (const spec of BUTTON_LAYOUT_SPECS) {
		const { class: className, props, selectorIncludes } = spec;

		for (const rule of rules) {
			if (!isBareButtonReset(rule.declarations)) continue;
			const matchesClass = rule.selectors.some((sel) => selectorUsesButtonClass(sel, className));
			if (!matchesClass) continue;
			hits.push(
				`${className}: in bare padding:0 reset (${rule.selectors.join(', ')}) — add explicit button.${className} layout`
			);
		}

		const matching = rules.filter((rule) =>
			rule.selectors.some((sel) => {
				if (!selectorUsesButtonClass(sel, className)) return false;
				if (selectorIncludes && !sel.includes(selectorIncludes)) return false;
				return true;
			})
		);
		if (!matching.length) {
			const ctx = selectorIncludes ? ` (expected selector with ${selectorIncludes})` : '';
			hits.push(`${className}: no button.${className} layout rule in ui.css${ctx}`);
			continue;
		}

		const merged = new Set();
		for (const rule of matching) {
			for (const p of declProps(rule.declarations)) merged.add(p);
		}
		for (const prop of props) {
			if (!merged.has(prop)) {
				hits.push(`${className}: button.${className} rules missing "${prop}" (have: ${[...merged].join(', ')})`);
			}
		}
	}

	return hits;
}

const ROUTE_RAW_BUTTON_CLASS_RE =
	/<button\b[^>]*\bclass="[^"]*\b(row2|rcho|one|add|cm|att|act|compose-text)\b/;

export const ROUTE_RAW_DIV_ROW2_RE = /<div\b[^>]*\bclass="[^"]*\brow2\b/;

export const ROUTE_RAW_COMPOSE_TEXT_RE = /class="[^"]*\bcompose-text\b/;

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
		...BUTTON_LAYOUT_CLASS_NAMES,
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
			`${className}: <button class="${className}"> in $ui — add BUTTON_LAYOUT_SPECS entry or BUTTON_TEXT_CLASSES in ui-guard.mjs`
		);
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
	const uiCssPath = path.join(webRoot, 'src', 'lib', 'styles', 'ui.css');
	const uiCss = fs.existsSync(uiCssPath) ? fs.readFileSync(uiCssPath, 'utf8') : '';
	const buttonCssSync = uiCss ? checkButtonCssSync(uiCss) : ['ui.css missing'];
	const unknownUiButtons = checkUnknownUiButtonClasses(webRoot);

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
			rawBtn.push(...searchSource(rel, source, /class="btn/));
			rawFldInput.push(...searchSource(rel, source, /<input[^>]*class="[^"]*fld/));
			rawTextarea.push(...searchSource(rel, source, /<textarea[^>]*class="[^"]*(fld|ta)/));
			rawLab.push(...searchSource(rel, source, /class="lab"/));
			if (!rel.includes('/dev/')) {
				rawRouteButtons.push(...searchSource(rel, source, ROUTE_RAW_BUTTON_CLASS_RE));
				rawRouteDivRow2.push(...searchSource(rel, source, ROUTE_RAW_DIV_ROW2_RE));
				rawRouteComposeText.push(...searchSource(rel, source, ROUTE_RAW_COMPOSE_TEXT_RE));
			}
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
				'check-ui: button layout classes in ui.css — button.X must mirror .X (Wynd UI phase 1–3); see ui-components.md.',
			hits: buttonCssSync
		},
		{
			message:
				'check-ui: new <button class="…"> in $ui must be listed in ui-guard.mjs (BUTTON_LAYOUT_SPECS or BUTTON_TEXT_CLASSES).',
			hits: unknownUiButtons
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

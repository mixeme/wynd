// Сторож вертикального ритма C120: зазоры между блоками — только ступени
// лесенки 4/8/16 из tokens.css. Оптические сдвиги до 2 px, нахлёсты рамок
// (−1 px) и явные исключения допускаются: это не межблочный ритм.
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

// Мастерская и «Подпись» — общие стили окна; клиент — ui.css; Настольный —
// app.css окна для компьютера (его утверждённые экраны с инлайн-отступами
// не сканируются, пока те кадры не пересмотрены).
const FILES = [
	'web/src/lib/styles/tokens.css',
	'web/src/lib/styles/ui.css',
	'web/src/lib/styles/workshop.css',
	'cmd/wynd-workshop/frontend/src/app.css',
	'cmd/wynd-desktop/frontend/src/app.css',
	...[
		'Notice.svelte',
		'FootBar.svelte',
		'MarkRows.svelte',
		'MarkRow.svelte',
		'StepNav.svelte',
		'StepList.svelte',
		'FieldLine.svelte',
		'CopyLine.svelte',
		'SelfBar.svelte',
		'Checkbox.svelte',
		'RawSheet.svelte',
		'TitleBar.svelte',
		'WorkshopTabs.svelte',
		'Select.svelte'
	].map((f) => `web/src/lib/components/workshop/${f}`)
];

const STEPS = new Set(['4px', '8px', '16px', 'var(--sp-1)', 'var(--sp-2)', 'var(--sp-3)']);
const ALLOWED_LITERAL = new Set(['0', '0px', 'auto', '-1px']);
const OPTICAL_MAX = 2;

// Где ступень не межблочный ритм: центровка приветствия по окну, пустые
// состояния (крупный отступ — центровка экрана) и абзац сырого вывода.
// Сверка по файлу, правилу и значению: значение 24/28 может быть законным
// в другом правиле.
const EXCEPTIONS = [
	{ file: 'cmd/wynd-workshop/frontend/src/app.css', selector: '.ins-welcome', value: '70px', why: 'центровка приветствия' },
	{ file: 'cmd/wynd-workshop/frontend/src/app.css', selector: '.ins-welcome', value: '40px', why: 'центровка приветствия' },
	{ file: 'web/src/lib/components/workshop/RawSheet.svelte', selector: '.ins-cause-part', value: '1.45em', why: 'абзац сырого вывода' },
	{ file: 'web/src/lib/styles/ui.css', selector: '.empty', value: '80px', why: 'центровка пустого состояния' },
	{ file: 'web/src/lib/styles/ui.css', selector: '.empty-state > .h1s', value: '48px', why: 'центровка пустого состояния' },
	{ file: 'web/src/lib/styles/ui.css', selector: '.empty-state.is-list > .h1s', value: '24px', why: 'центровка пустого состояния' },
	{ file: 'web/src/lib/styles/ui.css', selector: '.empty-state.is-tab > .h1s', value: '220px', why: 'центровка пустого состояния' },
	{ file: 'web/src/lib/styles/ui.css', selector: '.empty-state.is-feed > .h1s', value: '28px', why: 'центровка пустого состояния' }
];

const ignored = new Set(EXCEPTIONS.map((e) => `${e.file}|${e.selector}|${e.value}`));

function verticalValues(line) {
	const out = [];
	const m = line.match(/^\s*(margin-top|margin-bottom|row-gap)\s*:\s*([^;]+);/);
	if (m) {
		out.push({ value: m[2].trim() });
		return out;
	}
	const margin = line.match(/^\s*margin\s*:\s*([^;]+);/);
	if (margin) {
		const parts = margin[1].trim().split(/\s+/);
		if (parts.length === 1) out.push({ value: parts[0] });
		else if (parts.length === 2) out.push({ value: parts[0] });
		else {
			out.push({ value: parts[0] });
			out.push({ value: parts[2] });
		}
		return out;
	}
	const gap = line.match(/^\s*gap\s*:\s*([^;]+);/);
	if (gap) {
		const parts = gap[1].trim().split(/\s+/);
		if (parts.length === 2) out.push({ value: parts[0] });
	}
	return out;
}

function px(value) {
	const m = value.match(/^(-?\d+(?:\.\d+)?)px$/);
	return m ? Number(m[1]) : null;
}

const problems = [];
for (const rel of FILES) {
	const text = readFileSync(path.join(root, rel), 'utf8');
	let selector = '';
	text.split(/\r?\n/).forEach((line, i) => {
		const lineNo = i + 1;
		const trimmed = line.trim();
		const open = line.indexOf('{');
		if (open >= 0) {
			const cand = line.slice(0, open).trim();
			if (cand && !cand.startsWith('@')) selector = cand;
		}
		if (trimmed.startsWith('/*') || trimmed.startsWith('*') || trimmed.includes('{')) {
			if (trimmed === '}') selector = '';
			return;
		}
		for (const { value } of verticalValues(line)) {
			if (ignored.has(`${rel}|${selector}|${value}`)) continue;
			if (STEPS.has(value) || ALLOWED_LITERAL.has(value)) continue;
			if (/^(min|max|clamp|calc)\(/.test(value)) continue;
			const n = px(value);
			if (n !== null && Math.abs(n) <= OPTICAL_MAX) continue;
			problems.push({ file: rel, line: lineNo, value, selector });
		}
		if (trimmed === '}') selector = '';
	});
}

const tokens = readFileSync(path.join(root, 'web/src/lib/styles/tokens.css'), 'utf8');
for (const step of [1, 2, 3]) {
	if (!new RegExp(`--sp-${step}:\\s*\\d+px;`).test(tokens)) {
		problems.push({ file: 'web/src/lib/styles/tokens.css', line: 0, value: `нет --sp-${step}`, selector: '' });
	}
}

if (problems.length) {
	console.error('check-spacing: значения вне лесенки 4/8/16');
	for (const p of problems) console.error(`  ${p.file}:${p.line}  ${p.selector}  ${p.value}`);
	console.error('Зазоры задаются var(--sp-1…3) из tokens.css; см. C120 плана 46.');
	process.exit(1);
}
console.log(`check-spacing: OK (${FILES.length} файлов, лесенка 4/8/16)`);

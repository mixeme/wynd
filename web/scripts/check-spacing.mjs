// Сторож вертикального ритма C120: зазоры между блоками — только ступени
// лесенки 4/8/16 из tokens.css. Оптические сдвиги до 2 px и нахлёсты
// рамок (−1 px) допускаются: это не межблочный ритм.
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

// Пока лесенка внедрена в Мастерской и «Подписи» (общие стили окна); клиент
// и окно для компьютера добавляются следующими шагами C120.
const FILES = [
	'web/src/lib/styles/tokens.css',
	'web/src/lib/styles/workshop.css',
	'cmd/wynd-workshop/frontend/src/app.css',
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

const STEPS = new Set(['var(--sp-1)', 'var(--sp-2)', 'var(--sp-3)']);
const ALLOWED_LITERAL = new Set(['0', '0px', 'auto', '-1px']);
const OPTICAL_MAX = 2;

// Где ступень не межблочный ритм: центровка приветствия по окну и абзац
// сырого вывода в его собственных единицах. Сверка по файлу и значению,
// чтобы сторожа не ломали сдвиги строк.
const EXCEPTIONS = [
	{ file: 'cmd/wynd-workshop/frontend/src/app.css', value: '70px', why: 'центровка приветствия по окну' },
	{ file: 'cmd/wynd-workshop/frontend/src/app.css', value: '40px', why: 'центровка приветствия по окну' },
	{ file: 'web/src/lib/components/workshop/RawSheet.svelte', value: '1.45em', why: 'абзац сырого вывода по тексту' }
];

const ignoredValue = new Set(EXCEPTIONS.map((e) => `${e.file}:${e.value}`));

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
	text.split(/\r?\n/).forEach((line, i) => {
		if (line.trim().startsWith('/*') || line.trim().startsWith('*')) return;
		const lineNo = i + 1;
		for (const { value } of verticalValues(line)) {
			if (ignoredValue.has(`${rel}:${value}`)) continue;
			if (STEPS.has(value) || ALLOWED_LITERAL.has(value)) continue;
			const n = px(value);
			if (n !== null && Math.abs(n) <= OPTICAL_MAX) continue;
			problems.push({ file: rel, line: lineNo, value });
		}
	});
}

const tokens = readFileSync(path.join(root, 'web/src/lib/styles/tokens.css'), 'utf8');
for (const step of [1, 2, 3]) {
	if (!new RegExp(`--sp-${step}:\\s*\\d+px;`).test(tokens)) {
		problems.push({ file: 'web/src/lib/styles/tokens.css', line: 0, value: `нет --sp-${step}` });
	}
}

if (problems.length) {
	console.error('check-spacing: значения вне лесенки 4/8/12/16/24');
	for (const p of problems) console.error(`  ${p.file}:${p.line}  ${p.value}`);
	console.error('Зазоры задаются var(--sp-1…3) из tokens.css; см. C120 плана 46.');
	process.exit(1);
}
console.log(`check-spacing: OK (${FILES.length} файлов, лесенка 4/8/16)`);

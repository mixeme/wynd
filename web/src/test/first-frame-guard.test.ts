import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { checkFirstFrameReturn } from '../../scripts/first-frame-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

function tree(files: Record<string, string>): string {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'wynd-first-frame-'));
	const web = path.join(root, 'web');
	for (const [rel, text] of Object.entries(files)) {
		const abs = path.join(web, rel);
		fs.mkdirSync(path.dirname(abs), { recursive: true });
		fs.writeFileSync(abs, text);
	}
	return web;
}

const PAGE = 'src/routes/circles/[id]/search/+page.svelte';
const SCOPE = 'src/lib/journal/search-scope.ts';
const FIELD = 'src/lib/components/forms/SearchField.svelte';

const healthy = {
	[PAGE]: `const remembered = recallSearchScreen(memoryKey());
let loading = $state(false);
let hits = $state(remembered?.hits ?? []);
$effect(() => { rememberSearchScreen(memoryKey(), { hits, loading }); });
`,
	[SCOPE]: `export function noteArrival(type) { arrival = type === 'popstate' ? 'popstate' : 'other'; }
export function searchFocusAfterNavigation(path) { if (arrival === 'popstate') return false; return true; }
`,
	[FIELD]: `onMount(() => { if (autofocus && searchFocusAfterNavigation(window.location.pathname)) input?.focus(); });
`
};

// C160: без памяти кадра возврат с записи показывал «Ищем…» и пустое место,
// а повторный фокус поля в PWA Firefox поднимал клавиатуру.
describe('сторож первого кадра возврата', () => {
	it('ловит пропавшую память кадра и фокус без гейта', () => {
		const web = tree({
			[PAGE]: `let hits = $state([]);
$effect(() => { void reload(); });
`,
			[SCOPE]: 'export function searchFocusAfterNavigation() { return true; }\n',
			[FIELD]: 'onMount(() => { input?.focus(); });\n'
		});
		const hits = checkFirstFrameReturn(web);
		expect(hits.some((h) => h.includes('recallSearchScreen'))).toBe(true);
		expect(hits.some((h) => h.includes('rememberSearchScreen'))).toBe(true);
		expect(hits.some((h) => h.includes('noteArrival'))).toBe(true);
		expect(hits.some((h) => h.includes('popstate'))).toBe(true);
		expect(hits.some((h) => h.includes('searchFocusAfterNavigation'))).toBe(true);
	});

	it('ловит память, вспомянутую после инициализации строк', () => {
		const web = tree({
			...healthy,
			[PAGE]: `let hits = $state([]);
const remembered = recallSearchScreen(memoryKey());
$effect(() => { rememberSearchScreen(memoryKey(), { hits }); });
`
		});
		const hits = checkFirstFrameReturn(web);
		expect(hits.some((h) => h.includes('после инициализации'))).toBe(true);
	});

	it('на нынешнем дереве возврат в поиск помнит кадр', () => {
		expect(checkFirstFrameReturn(webRoot)).toEqual([]);
	});
});

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
	analyzeScreenSource,
	checkLibraryClasses,
	checkLibraryRegistry,
	checkProject,
	checkScriptClassStrings,
	checkSingleScreenClasses,
	classTokens,
	hasStyleBlock,
	markupElements,
	parseCssRules,
	rawClassHits
} from '../../scripts/ui-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const uiCss = fs.readFileSync(path.join(webRoot, 'src/lib/styles/ui.css'), 'utf8');

// Сброс :where(button) в ui.css накрывает браузерные стили кнопки, поэтому
// зеркал button.X больше нет — и сторожа их синхронности тоже (REF-8).
describe('ui.css', () => {
	it('не содержит зеркал button.X', () => {
		const mirrors = parseCssRules(uiCss).filter((rule) =>
			rule.selectors.some((sel) => /button\./.test(sel))
		);
		expect(mirrors.map((m) => m.selectors.join(', '))).toEqual([]);
	});

	it('сбрасывает кнопку с нулевой весомостью', () => {
		const reset = parseCssRules(uiCss).find((rule) =>
			rule.selectors.some((sel) => sel.replace(/\s/g, '') === ':where(button)')
		);
		expect(reset).toBeTruthy();
		const props = (reset?.declarations ?? []).map((d) => d.split(':')[0].trim());
		for (const p of ['font', 'color', 'background', 'border', 'padding', 'cursor']) {
			expect(props).toContain(p);
		}
	});
});

describe('prod route raw markup guards', () => {
	// Инвариант (план 42, GUARD-1): сырой класс ловится в любом написании —
	// не первым в строке, в выражении, в шаблонной строке, в интерполяции
	// значения и директивой class:; тег читается целиком, стрелка внутри
	// {…} его не обрывает; скрипт экрана не проверяется.
	it('catches every spelling of a raw class', () => {
		const rel = 'src/routes/x/+page.svelte';
		const source = [
			`<script>const s = '<div class="btn">';</script>`,
			'<div class="x btn">a</div>',
			"<div class={'btn'}>b</div>",
			'<div class={`pad ${big ? "btn" : ""}`}>c</div>',
			`<div class="x {on ? 'btn' : ''}">d</div>`,
			'<div class:btn={on}>e</div>',
			'<button onclick={() => a > b} class="row2">f</button>',
			'<div class="row2">g</div>',
			'<div class="btnx">h</div>'
		].join(String.fromCharCode(10));
		const hits = rawClassHits(rel, source);
		expect(hits.rawBtn.map((h: string) => h.split(':')[1])).toEqual(['2', '3', '4', '5', '6']);
		expect(hits.rawRouteButtons.map((h: string) => h.split(':')[1])).toEqual(['7']);
		expect(hits.rawRouteDivRow2.map((h: string) => h.split(':')[1])).toEqual(['8']);
	});

	it('skips prod-only rules on /dev/ screens', () => {
		const hits = rawClassHits('src/routes/dev/x/+page.svelte', '<div class="row2">a</div>');
		expect(hits.rawRouteDivRow2).toEqual([]);
	});

	it('splits class values into tokens', () => {
		expect(classTokens("a {x ? 'b c' : `d ${e}`} f")).toEqual(['b', 'c', 'd', 'a', 'f']);
	});

	// Инвариант (GUARD-2): храповик считает все написания инлайн-стиля.
	it('counts style=, style={} and style: directives', () => {
		const els = markupElements('<div style="a:1" style:color={c}></div><p style={s}></p>');
		expect(els.reduce((n: number, el: { styles: number }) => n + el.styles, 0)).toBe(3);
	});

	// Инвариант (GUARD-4): компонент из переменной не обходит проверку импортов.
	it('flags dynamic .svelte imports and <svelte:component>', () => {
		const inventory = { ui: new Set<string>(), layouts: new Set<string>() };
		const source = [
			'<script>',
			"const A = import('./Local.svelte');",
			'const B = import(`$ui/x/Y.svelte`);',
			"const C = import.meta.glob('./*.svelte');",
			"const D = import('leaflet');",
			'</script>',
			'<svelte:component this={A} />'
		].join(String.fromCharCode(10));
		const hits = analyzeScreenSource('src/routes/x/+page.svelte', source, inventory);
		expect(hits.map((h: string) => h.split(':')[1])).toEqual(['2', '3', '4', '7']);
	});

	// Инвариант (GUARD-3): свой <style> ищется в разметке, не в скрипте и не в комментарии.
	it('finds a style block only in markup', () => {
		expect(hasStyleBlock('<div></div>\n<style>.a{}</style>')).toBe(true);
		expect(hasStyleBlock("<script>const s = '<style>';</script><div></div>")).toBe(false);
		expect(hasStyleBlock('<!-- <style> --><div></div>')).toBe(false);
	});

	it('passes checkProject on current tree (prod routes clean)', () => {
		const { ok, groups } = checkProject(webRoot);
		const failed = groups.filter((g) => g.hits.length > 0).map((g) => g.message);
		expect(failed).toEqual([]);
		expect(ok).toBe(true);
	});
});

describe('parseCssRules', () => {
	it('skips comments', () => {
		const rules = parseCssRules('/* .x { padding: 0; } */ .a { color: red; }');
		expect(rules).toHaveLength(1);
		expect(rules[0].selectors).toEqual(['.a']);
	});
});

// План 47, сторож: правила проверяются на временном дереве проекта.
describe('plan 47 guards', () => {
	function tree(files: Record<string, string>): string {
		const root = fs.mkdtempSync(path.join(os.tmpdir(), 'wynd-guard-'));
		const web = path.join(root, 'web');
		for (const [rel, text] of Object.entries(files)) {
			const abs = path.join(rel.startsWith('docs/') ? root : web, rel);
			fs.mkdirSync(path.dirname(abs), { recursive: true });
			fs.writeFileSync(abs, text);
		}
		return web;
	}

	it('catches library classes on raw tags and stale ratchet entries', () => {
		const web = tree({
			'src/routes/a/+page.svelte':
				'<div class="panel">x</div>\n<span class="tm">1</span>\n{@html q}\n<input type="file" hidden />',
			'src/routes/b/+page.svelte': '<Panel class="panel" />',
			'src/routes/dev/c/+page.svelte': '<div class="panel"></div>'
		});
		const hits = checkLibraryClasses(
			web,
			['panel'],
			{ 'src/routes/a/+page.svelte': ['tm'], 'src/routes/b/+page.svelte': ['qr'] },
			new Set()
		);
		expect(hits).toHaveLength(4);
		expect(hits[0]).toMatch(/^src\/routes\/a\/\+page\.svelte:1:.*\.panel на <div>/);
		expect(hits[1]).toMatch(/^src\/routes\/a\/\+page\.svelte:4:.*есть FilePicker/);
		expect(hits[2]).toContain('{@html}');
		expect(hits[3]).toContain('.qr больше нет');
	});

	it('requires every library file in the reference', () => {
		const web = tree({
			'src/lib/components/forms/Known.svelte': '<div></div>',
			'src/lib/components/forms/Stray.svelte': '<div></div>',
			'docs/reference/ui-components.md': 'Компоненты: **Known**.'
		});
		const hits = checkLibraryRegistry(web);
		expect(hits).toHaveLength(1);
		expect(hits[0]).toContain('Stray');
	});

	it('catches class strings in screen scripts', () => {
		const web = tree({
			'src/lib/styles/ui.css': '.flex-mid { display: flex; } .gap-10 { gap: 10px; }',
			'src/routes/a/+page.svelte':
				"<script>\n\tconst row = 'flex-mid gap-10';\n\tconst word = 'flex-mid';\n\tconst text = 'flex-mid и всё';\n</script>\n<div class={row}></div>"
		});
		const hits = checkScriptClassStrings(web);
		expect(hits).toHaveLength(1);
		expect(hits[0]).toContain('src/routes/a/+page.svelte:2:');
	});

	it('catches new single-screen classes and stale entries', () => {
		const web = tree({
			'src/lib/styles/ui.css':
				'.own { color: red; } .shared { color: red; } .lib { color: red; }\n/* Служебные классы: */\n.mt-8 { margin-top: 8px; }',
			'src/lib/components/forms/X.svelte': '<div class="lib"></div>',
			'src/routes/a/+page.svelte': '<div class="own shared lib mt-8"></div>',
			'src/routes/b/+page.svelte': '<div class="shared"></div>'
		});
		expect(checkSingleScreenClasses(web, new Set())).toEqual([
			'src/routes/a/+page.svelte: .own в ui.css нужен одному экрану — компонент $ui или служебный класс'
		]);
		expect(checkSingleScreenClasses(web, new Set(['own', 'gone']))).toEqual([
			'.gone уже не одноэкранный — уберите из SINGLE_SCREEN_CLASSES'
		]);
	});
});

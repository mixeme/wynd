import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
	analyzeScreenSource,
	checkProject,
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

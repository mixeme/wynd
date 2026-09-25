import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { checkButtonCssSync, parseCssRules } from '../../scripts/ui-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const uiCss = fs.readFileSync(path.join(webRoot, 'src/lib/styles/ui.css'), 'utf8');

describe('checkButtonCssSync', () => {
	it('passes on current ui.css', () => {
		expect(checkButtonCssSync(uiCss)).toEqual([]);
	});

	it('flags layout class in bare padding:0 reset', () => {
		const bad = `
			button.row2, button.done { padding: 0; border: none; background: none; }
			button.row2 { display:flex; padding:13px 16px; }
		`;
		expect(checkButtonCssSync(bad).some((h) => h.includes('row2') && h.includes('bare'))).toBe(true);
	});

	it('allows padding:0 inside full layout rule', () => {
		const ok = `
			button.rcho { width:26px; height:26px; display:grid; padding:0; border:1px solid var(--line); }
		`;
		expect(checkButtonCssSync(ok).some((h) => h.includes('rcho') && h.includes('bare'))).toBe(false);
	});

	it('requires .rx context for button.one', () => {
		const bad = `
			.rx button.one { padding:4px 11px; display:flex; border:none; }
			button.one { padding:0; border:none; background:none; }
		`;
		expect(checkButtonCssSync(bad).some((h) => h.includes('one') && h.includes('bare'))).toBe(true);
	});
});

describe('parseCssRules', () => {
	it('skips comments', () => {
		const rules = parseCssRules('/* .x { padding: 0; } */ .a { color: red; }');
		expect(rules).toHaveLength(1);
		expect(rules[0].selectors).toEqual(['.a']);
	});
});

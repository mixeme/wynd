import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { checkChromeOwners } from '../../scripts/chrome-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

function tree(files: Record<string, string>): string {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'wynd-chrome-'));
	const web = path.join(root, 'web');
	for (const [rel, text] of Object.entries(files)) {
		const abs = path.join(web, rel);
		fs.mkdirSync(path.dirname(abs), { recursive: true });
		fs.writeFileSync(abs, text);
	}
	return web;
}

// C140: копия шапки в чужом компоненте разошлась с принятой геометрией
// («Кадр» и редактор записи), а сторож экранов её не видел — он про routes.
describe('сторож шапок', () => {
	it('ловит копию шапки в чужом компоненте и на экране', () => {
		const web = tree({
			'src/lib/components/chrome/ComposeHeader.svelte':
				'<div class="cbar"><div class="top compose-top"></div></div>',
			'src/lib/layouts/FormLayout.svelte': '<div class="cbar"></div>',
			'src/lib/components/chrome/CircleBar.svelte':
				'<span class="idn"><span class="idn-t"></span></span>',
			'src/lib/components/overlays/Copy.svelte':
				'<div class="cbar"><div class="top compose-top"></div></div>',
			'src/routes/a/+page.svelte': '<div class="cbar"></div>',
			'src/routes/dev/b/+page.svelte': '<div class="cbar"></div>'
		});
		const hits = checkChromeOwners(web);
		expect(hits.map((h: string) => h.split(':')[0])).toEqual([
			'src/lib/components/overlays/Copy.svelte',
			'src/lib/components/overlays/Copy.svelte',
			'src/routes/a/+page.svelte'
		]);
		expect(hits[0]).toContain('.cbar');
		expect(hits[1]).toContain('.compose-top');
	});

	it('на нынешнем дереве шапки у своих компонентов', () => {
		expect(checkChromeOwners(webRoot)).toEqual([]);
	});
});

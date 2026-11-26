import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { checkViewportMeta } from '../../scripts/viewport-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

function tree(files: Record<string, string>): string {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'wynd-viewport-'));
	const web = path.join(root, 'web');
	for (const [rel, text] of Object.entries(files)) {
		const abs = path.join(web, rel);
		fs.mkdirSync(path.dirname(abs), { recursive: true });
		fs.writeFileSync(abs, text);
	}
	return web;
}

// C144: resizes-content сжимал layout viewport под клавиатуру, и её место при
// закрытии закрашивалось фоном оболочки — белая вспышка на пол-экрана.
describe('сторож клавиатуры', () => {
	it('ловит resizes-content и не трогает обычную метку', () => {
		const web = tree({
			'src/app.html':
				'<meta name="viewport" content="width=device-width, initial-scale=1, interactive-widget=resizes-content" />',
			'src/lib/ok.html': '<meta name="viewport" content="width=device-width" />',
			'src/x/index.html':
				'<head>\n<meta\n  name="viewport"\n  content="width=device-width, interactive-widget=resizes-content"\n/>\n</head>'
		});
		const hits = checkViewportMeta(web);
		expect(hits.map((h: string) => h.split(':')[0])).toEqual(['src/app.html', 'src/x/index.html']);
		expect(hits[0]).toContain('src/app.html:1');
		expect(hits[1]).toContain('resizes-content');
	});

	it('на нынешнем дереве метка без resizes-content', () => {
		expect(checkViewportMeta(webRoot)).toEqual([]);
	});
});

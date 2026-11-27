import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { checkDialogFit } from '../../scripts/dialog-guard.mjs';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

function tree(files: Record<string, string>): string {
	const root = fs.mkdtempSync(path.join(os.tmpdir(), 'wynd-dialog-'));
	const web = path.join(root, 'web');
	for (const [rel, text] of Object.entries(files)) {
		const abs = path.join(web, rel);
		fs.mkdirSync(path.dirname(abs), { recursive: true });
		fs.writeFileSync(abs, text);
	}
	return web;
}

// C156: с фиксированным верхом без --app-h диалог удаления круга уходил
// кнопками под клавиатуру, и владелец видел мигание диалога.
describe('сторож диалогов', () => {
	it('ловит правило без --app-h и старое исключение fitViewport', () => {
		const web = tree({
			'src/lib/styles/ui.css':
				'body .dlg {\n\ttop: 250px;\n}\nbody .dlg.dlg-fit-viewport {\n' +
				'\t--dialog-top: clamp(12px, calc(var(--app-h, 100dvh) - 400px), 250px);\n' +
				'\tmax-height: calc(var(--app-h, 100dvh) - var(--dialog-top) - 12px);\n}\n'
		});
		const hits = checkDialogFit(web);
		expect(hits).toHaveLength(2);
		expect(hits[0]).toContain('top');
		expect(hits[1]).toContain('max-height');
	});

	it('на нынешнем дереве диалог вписан в видимую область', () => {
		expect(checkDialogFit(webRoot)).toEqual([]);
	});
});

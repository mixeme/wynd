import { describe, expect, it } from 'vitest';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

// web/static/THIRD_PARTY_LICENSES.txt лежит в репозитории, потому что стадия
// web в образе собирается без Go и сгенерировать файл там нечем. Значит, он
// может устареть молча — этот тест и есть ворота: при смене зависимостей
// npm run licenses обязателен (LIC-3).
describe('third-party licenses', () => {
	const webRoot = process.cwd();
	const committedPath = path.join(webRoot, 'static', 'THIRD_PARTY_LICENSES.txt');
	const lf = (s: string) => s.replace(/\r\n/g, '\n');

	it('файл собран из текущих зависимостей', { timeout: 180_000 }, () => {
		const out = path.join(os.tmpdir(), `wynd-licenses-${process.pid}.txt`);
		try {
			execFileSync(process.execPath, ['scripts/third-party-licenses.mjs'], {
				cwd: webRoot,
				env: { ...process.env, WYND_LICENSES_OUT: out },
				encoding: 'utf8'
			});
			expect(lf(fs.readFileSync(out, 'utf8')), 'запустите npm run licenses').toBe(
				lf(fs.readFileSync(committedPath, 'utf8'))
			);
		} finally {
			fs.rmSync(out, { force: true });
		}
	});

	it('в файле есть и клиентские пакеты, и модули сервера', () => {
		const text = lf(fs.readFileSync(committedPath, 'utf8'));
		expect(text).toContain('Клиент (npm, боевые зависимости)');
		expect(text).toContain('Сервер (модули Go)');
		expect(text).toContain('modernc.org/sqlite@');
		expect(text).toContain('leaflet@');
	});
});

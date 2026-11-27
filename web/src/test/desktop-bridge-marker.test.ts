import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const appHtml = fs.readFileSync(path.join(webRoot, 'src/app.html'), 'utf8');

// C155 и старые окна: сборки Настольного до 0.5.7 решали, вставлять ли мост,
// по вхождению имени в HTML — и принимали проверку в app.html за готовый мост.
// Голое имя в странице снова сломает такое окно: без адреса сервера и порта API.
describe('мост окна в app.html', () => {
	it('ставит класс по мосту, не называя его в HTML целиком', () => {
		expect(appHtml).toContain("window['__wynd' + 'Desktop']");
		expect(appHtml).toContain("classList.add('desktop-window')");
		expect(appHtml).not.toContain('__wyndDesktop');
	});
});

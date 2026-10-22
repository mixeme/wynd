import { beforeEach, describe, expect, it } from 'vitest';
import { closeBrowserHint, showBrowserHint } from './browserHint';

const FIREFOX_ANDROID = 'Mozilla/5.0 (Android 13; Mobile; rv:158.0) Gecko/158.0 Firefox/158.0';
const CHROME_ANDROID =
	'Mozilla/5.0 (Linux; Android 13; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36';
const MI_BROWSER =
	'Mozilla/5.0 (Linux; U; Android 13; ru-ru; M2101K7AG Build/TKQ1) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/112.0.5615.136 Mobile Safari/537.36 XiaoMi/MiuiBrowser/14.5.6-gn';
const SAMSUNG =
	'Mozilla/5.0 (Linux; Android 13; SM-A525F) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/26.0 Chrome/122.0.0.0 Mobile Safari/537.36';
const FIREFOX_DESKTOP = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0';

// Подсказка (7.7а) — на Android в любом браузере, кроме Chrome и Vivaldi, и до крестика.
describe('подсказка про браузер', () => {
	beforeEach(() => localStorage.clear());

	it('видна в Firefox, Mi Browser и Samsung Internet на Android', () => {
		expect(showBrowserHint(FIREFOX_ANDROID)).toBe(true);
		expect(showBrowserHint(MI_BROWSER)).toBe(true);
		expect(showBrowserHint(SAMSUNG)).toBe(true);
	});

	it('не видна в Chrome на Android и на компьютере', () => {
		expect(showBrowserHint(CHROME_ANDROID)).toBe(false);
		expect(showBrowserHint(FIREFOX_DESKTOP)).toBe(false);
	});

	it('после крестика не показывается', () => {
		closeBrowserHint();
		expect(showBrowserHint(FIREFOX_ANDROID)).toBe(false);
	});
});

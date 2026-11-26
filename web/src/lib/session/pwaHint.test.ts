import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { closePwaHint, pwaInstallSteps, showPwaHint } from './pwaHint';

const CHROME_ANDROID =
	'Mozilla/5.0 (Linux; Android 13; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36';
const FIREFOX_ANDROID = 'Mozilla/5.0 (Android 13; Mobile; rv:158.0) Gecko/158.0 Firefox/158.0';
const FIREFOX_DESKTOP =
	'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0';
const CHROME_DESKTOP =
	'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';
const IPHONE =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 Version/18.5 Mobile/15E148 Safari/604.1';
const SAFARI_MAC =
	'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18.5 Safari/605.1.15';

afterEach(() => {
	delete (window as unknown as { __wyndAndroid?: unknown }).__wyndAndroid;
	delete (window as unknown as { __wyndDesktop?: unknown }).__wyndDesktop;
	delete (navigator as unknown as { standalone?: boolean }).standalone;
});

// План 74: баннер «Wynd на главный экран» — в браузере, не в приложении.
describe('баннер про веб-приложение', () => {
	beforeEach(() => localStorage.clear());

	it('виден в браузерах на телефоне и компьютере', () => {
		expect(showPwaHint(CHROME_ANDROID, false)).toBe(true);
		expect(showPwaHint(FIREFOX_ANDROID, false)).toBe(true);
		expect(showPwaHint(IPHONE, false)).toBe(true);
		expect(showPwaHint(CHROME_DESKTOP, false)).toBe(true);
	});

	it('не виден в установленной PWA', () => {
		expect(showPwaHint(CHROME_ANDROID, true)).toBe(false);
	});

	it('не виден в приложении Wynd и в окне Настольного', () => {
		(window as unknown as { __wyndAndroid: unknown }).__wyndAndroid = { postMessage() {} };
		expect(showPwaHint(CHROME_ANDROID, false)).toBe(false);
		delete (window as unknown as { __wyndAndroid?: unknown }).__wyndAndroid;
		(window as unknown as { __wyndDesktop: unknown }).__wyndDesktop = {
			origin: 'https://wynd.example'
		};
		expect(showPwaHint(CHROME_DESKTOP, false)).toBe(false);
	});

	it('в Firefox на компьютере установки нет — не показываем', () => {
		expect(showPwaHint(FIREFOX_DESKTOP, false)).toBe(false);
	});

	it('после крестика не показывается', () => {
		closePwaHint();
		expect(showPwaHint(CHROME_ANDROID, false)).toBe(false);
	});
});

describe('шаги установки по браузеру', () => {
	it('Safari на телефоне — через «Поделиться»', () => {
		expect(pwaInstallSteps(IPHONE)).toContain('Поделиться');
		expect(pwaInstallSteps(IPHONE)).toContain('Домой');
	});

	it('Firefox — через меню', () => {
		expect(pwaInstallSteps(FIREFOX_ANDROID)).toContain('меню Firefox');
	});

	it('Safari на компьютере — «Добавить в Dock»', () => {
		expect(pwaInstallSteps(SAFARI_MAC)).toContain('Dock');
	});

	it('Chrome на Android — «Установить приложение»', () => {
		expect(pwaInstallSteps(CHROME_ANDROID)).toContain('Установить приложение');
	});

	it('Chrome на компьютере — значок в адресной строке', () => {
		expect(pwaInstallSteps(CHROME_DESKTOP)).toContain('адресной строке');
	});
});

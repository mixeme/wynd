import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { closeAndroidAppHint, showAndroidAppHint } from './appHint';

const CHROME_ANDROID =
	'Mozilla/5.0 (Linux; Android 13; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36';
const FIREFOX_ANDROID = 'Mozilla/5.0 (Android 13; Mobile; rv:158.0) Gecko/158.0 Firefox/158.0';
const FIREFOX_DESKTOP =
	'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0';
const IPHONE =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148';

afterEach(() => {
	delete (window as unknown as { __wyndAndroid?: unknown }).__wyndAndroid;
});

// Баннер про приложение (C139): в любом вебе на Android, кроме собственного
// окна Wynd, и до крестика.
describe('баннер про приложение для Android', () => {
	beforeEach(() => localStorage.clear());

	it('виден в браузере и в PWA на Android', () => {
		expect(showAndroidAppHint(CHROME_ANDROID)).toBe(true);
		expect(showAndroidAppHint(FIREFOX_ANDROID)).toBe(true);
	});

	it('не виден внутри приложения Wynd — человек уже в нём', () => {
		(window as unknown as { __wyndAndroid: unknown }).__wyndAndroid = {
			postMessage() {
				throw new Error('Detection must not send messages');
			}
		};
		expect(showAndroidAppHint(CHROME_ANDROID)).toBe(false);
	});

	it('не виден на компьютере и iPhone', () => {
		expect(showAndroidAppHint(FIREFOX_DESKTOP)).toBe(false);
		expect(showAndroidAppHint(IPHONE)).toBe(false);
	});

	it('после крестика не показывается', () => {
		closeAndroidAppHint();
		expect(showAndroidAppHint(CHROME_ANDROID)).toBe(false);
	});
});

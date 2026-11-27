import { describe, expect, it } from 'vitest';
import { canOpenInApp } from './openInApp';

const CHROME_WINDOWS =
	'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';
const FIREFOX_WINDOWS =
	'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:158.0) Gecko/20100101 Firefox/158.0';
const FIREFOX_LINUX = 'Mozilla/5.0 (X11; Linux x86_64; rv:158.0) Gecko/20100101 Firefox/158.0';
const CHROME_UBUNTU =
	'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';
const CHROME_ANDROID =
	'Mozilla/5.0 (Linux; Android 13; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36';
const FIREFOX_ANDROID = 'Mozilla/5.0 (Android 13; Mobile; rv:158.0) Gecko/158.0 Firefox/158.0';
const SAFARI_MAC =
	'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18.5 Safari/605.1.15';
const SAFARI_IPHONE =
	'Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 Version/18.5 Mobile/15E148 Safari/604.1';
const CHROME_CHROMEOS =
	'Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';

describe('«Открыть в приложении» на странице приглашения', () => {
	it('показывается в браузере Windows и Linux', () => {
		expect(canOpenInApp(CHROME_WINDOWS)).toBe(true);
		expect(canOpenInApp(FIREFOX_WINDOWS)).toBe(true);
		expect(canOpenInApp(FIREFOX_LINUX)).toBe(true);
		expect(canOpenInApp(CHROME_UBUNTU)).toBe(true);
	});

	it('не показывается на телефонах и в ChromeOS', () => {
		expect(canOpenInApp(CHROME_ANDROID)).toBe(false);
		expect(canOpenInApp(FIREFOX_ANDROID)).toBe(false);
		expect(canOpenInApp(SAFARI_IPHONE)).toBe(false);
		expect(canOpenInApp(CHROME_CHROMEOS)).toBe(false);
	});

	it('не показывается на macOS и без User-Agent', () => {
		expect(canOpenInApp(SAFARI_MAC)).toBe(false);
		expect(canOpenInApp(undefined)).toBe(false);
		expect(canOpenInApp('')).toBe(false);
	});
});

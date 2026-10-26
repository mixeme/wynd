import { afterEach, describe, expect, it, vi } from 'vitest';
import { openDownload } from './openDownload';

describe('openDownload', () => {
	afterEach(() => {
		vi.restoreAllMocks();
		delete (navigator as Navigator & { standalone?: boolean }).standalone;
	});

	function clicked(): HTMLAnchorElement {
		let anchor: HTMLAnchorElement | undefined;
		vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
			this: HTMLAnchorElement
		) {
			// eslint-disable-next-line @typescript-eslint/no-this-alias -- ловим ссылку, по которой щёлкнули
			anchor = this;
		});
		openDownload('https://wynd.example/api/v1/archive/tok');
		if (!anchor) throw new Error('no click');
		return anchor;
	}

	it('opens in the same window in a browser and on Android', () => {
		const a = clicked();
		expect(a.href).toBe('https://wynd.example/api/v1/archive/tok');
		expect(a.target).toBe('');
	});

	it('opens a new window in the iPhone home-screen app, so the file does not replace Wynd', () => {
		Object.defineProperty(navigator, 'standalone', { value: true, configurable: true });
		expect(clicked().target).toBe('_blank');
	});
});

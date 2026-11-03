import { afterEach, describe, expect, it, vi } from 'vitest';
import { openDownload, openDownloadAfter } from './openDownload';

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

	function iosWindow() {
		Object.defineProperty(navigator, 'standalone', { value: true, configurable: true });
		const child = {
			closed: false,
			opener: window as Window | null,
			location: { replace: vi.fn() },
			close: vi.fn()
		};
		const open = vi.spyOn(window, 'open').mockReturnValue(child as unknown as Window);
		return { child, open };
	}

	it('opens the iPhone window before requesting a link and hands off after the response', async () => {
		const { child, open } = iosWindow();
		let finish!: (href: string) => void;
		const load = vi.fn(() => {
			expect(open).toHaveBeenCalledWith('about:blank', '_blank');
			expect(child.opener).toBeNull();
			return new Promise<string>((resolve) => {
				finish = resolve;
			});
		});
		const pending = openDownloadAfter(load);
		expect(child.location.replace).not.toHaveBeenCalled();
		finish('https://wynd.example/api/v1/archive/one-time');
		await pending;
		expect(child.location.replace).toHaveBeenCalledWith(
			'https://wynd.example/api/v1/archive/one-time'
		);
		expect(open).toHaveBeenCalledTimes(1);
		expect(child.close).not.toHaveBeenCalled();
	});

	it('closes the reserved window and propagates a busy or failed link request', async () => {
		const { child } = iosWindow();
		const error = new Error('busy');
		await expect(openDownloadAfter(() => Promise.reject(error))).rejects.toBe(error);
		expect(child.close).toHaveBeenCalledOnce();
		expect(child.location.replace).not.toHaveBeenCalled();
	});

	it('does not request a token when the browser blocks the window', async () => {
		iosWindow();
		vi.mocked(window.open).mockReturnValue(null);
		const load = vi.fn();
		await expect(openDownloadAfter(load)).rejects.toThrow('download_window_blocked');
		expect(load).not.toHaveBeenCalled();
	});

	it('respects closing the window while the one-time link is pending', async () => {
		const { child, open } = iosWindow();
		let finish!: (href: string) => void;
		const pending = openDownloadAfter(
			() =>
				new Promise<string>((resolve) => {
					finish = resolve;
				})
		);
		child.closed = true;
		finish('https://wynd.example/api/v1/archive/one-time');
		await pending;
		expect(child.location.replace).not.toHaveBeenCalled();
		expect(open).toHaveBeenCalledTimes(1);
	});

	it('keeps browser and Android downloads native without opening a window', async () => {
		const open = vi.spyOn(window, 'open');
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
		await openDownloadAfter(async () => 'https://wynd.example/api/v1/archive/one-time');
		expect(open).not.toHaveBeenCalled();
		expect(click).toHaveBeenCalledOnce();
	});
});

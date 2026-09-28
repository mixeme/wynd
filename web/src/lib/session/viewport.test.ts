import { afterEach, describe, expect, it, vi } from 'vitest';
import { initViewportHeight } from './viewport';

function fakeViewport(init: { height: number; offsetTop?: number; scale?: number }) {
	const listeners: Record<string, (() => void)[]> = {};
	const vv = {
		height: init.height,
		offsetTop: init.offsetTop ?? 0,
		scale: init.scale ?? 1,
		addEventListener: (type: string, fn: () => void) => {
			(listeners[type] ??= []).push(fn);
		},
		removeEventListener: () => {},
		fire(type: string) {
			for (const fn of listeners[type] ?? []) fn();
		}
	};
	Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true });
	return vv;
}

describe('высота окна по видимой области', () => {
	afterEach(() => {
		document.documentElement.style.removeProperty('--app-h');
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});

	it('клавиатура сжимает окно, сдвиг страницы сбрасывается', () => {
		vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
			fn(0);
			return 1;
		});
		const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
		const vv = fakeViewport({ height: 800 });
		const stop = initViewportHeight();
		expect(document.documentElement.style.getPropertyValue('--app-h')).toBe('800px');

		vv.height = 480; // открылась клавиатура
		vv.offsetTop = 320; // браузер сдвинул страницу к полю ввода
		vv.fire('resize');
		expect(document.documentElement.style.getPropertyValue('--app-h')).toBe('480px');
		expect(scrollTo).toHaveBeenCalledWith(0, 0);
		stop();
		expect(document.documentElement.style.getPropertyValue('--app-h')).toBe('');
	});

	it('увеличение щипком — не клавиатура, высоту не трогаем', () => {
		fakeViewport({ height: 400, scale: 2 });
		const stop = initViewportHeight();
		expect(document.documentElement.style.getPropertyValue('--app-h')).toBe('');
		stop();
	});
});

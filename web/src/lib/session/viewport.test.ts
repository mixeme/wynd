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
			// 0, не 1: иначе «кадр уже заказан», и следующие события не дойдут.
			return 0;
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

	// Поле имени на экране входа в круг оставалось под клавиатурой: окно
	// сжалось, а экран внутри него никто не прокрутил.
	it('поле под клавиатурой доводится до видимого, видимое не трогается', () => {
		vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
			fn(0);
			// 0, не 1: иначе «кадр уже заказан», и следующие события не дойдут.
			return 0;
		});
		vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
		const vv = fakeViewport({ height: 800 });
		const stop = initViewportHeight();

		const input = document.createElement('input');
		document.body.append(input);
		input.focus();
		const reveal = vi.fn();
		input.scrollIntoView = reveal;
		const place = (top: number) =>
			vi.spyOn(input, 'getBoundingClientRect').mockReturnValue({ top, bottom: top + 44 } as DOMRect);

		place(600);
		vv.height = 480;
		vv.fire('resize');
		expect(reveal).toHaveBeenCalledWith({ block: 'center' });

		reveal.mockClear();
		vv.height = 800;
		vv.fire('resize');
		place(200);
		vv.height = 480;
		vv.fire('resize');
		expect(reveal).not.toHaveBeenCalled();

		// Панель браузера спряталась и вернулась — не клавиатура.
		place(790);
		vv.height = 800;
		vv.fire('resize');
		vv.height = 744;
		vv.fire('resize');
		expect(reveal).not.toHaveBeenCalled();

		input.remove();
		stop();
	});

	// Vivaldi: клавиатуру спрятали, нажали другое поле — оно под клавиатурой.
	// Поле проверяется и при входе в него, и после: экран мог уехать позже.
	it('поле перепроверяется при входе в него и пока клавиатура встаёт', () => {
		vi.useFakeTimers();
		vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
			fn(0);
			// 0, не 1: иначе «кадр уже заказан», и следующие события не дойдут.
			return 0;
		});
		vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
		const vv = fakeViewport({ height: 800 });
		const stop = initViewportHeight();

		// Поле — внутри экрана, который прокручивается: его есть куда подвести.
		const scroller = document.createElement('div');
		scroller.style.overflowY = 'auto';
		vi.spyOn(scroller, 'scrollHeight', 'get').mockReturnValue(1200);
		vi.spyOn(scroller, 'clientHeight', 'get').mockReturnValue(400);
		const input = document.createElement('input');
		scroller.append(input);
		document.body.append(scroller);
		const reveal = vi.fn();
		input.scrollIntoView = reveal;
		const rect = vi.spyOn(input, 'getBoundingClientRect');
		const place = (top: number) => rect.mockReturnValue({ top, bottom: top + 44 } as DOMRect);

		// Клавиатура закрыта: вход в поле сам ничего не двигает.
		place(600);
		input.focus();
		expect(reveal).not.toHaveBeenCalled();

		// Клавиатура встала, поле на виду — а потом экран уехал.
		place(200);
		vv.height = 480;
		vv.fire('resize');
		expect(reveal).not.toHaveBeenCalled();
		place(600);
		vi.advanceTimersByTime(1000);
		expect(reveal).toHaveBeenCalledWith({ block: 'center' });

		// Поле вплотную к клавиатуре: панель автозаполнения ложится поверх
		// страницы, в видимую область не входит — и закрывает его.
		reveal.mockClear();
		place(429);
		vi.advanceTimersByTime(1000);
		input.blur();
		input.focus();
		expect(reveal).toHaveBeenCalledTimes(1);

		// Клавиатура открыта, перешли в поле ниже края.
		place(600);
		reveal.mockClear();
		input.blur();
		input.focus();
		expect(reveal).toHaveBeenCalledTimes(1);

		// Клавиатуру убрали: отложенные проверки поле не трогают.
		reveal.mockClear();
		vv.height = 800;
		vv.fire('resize');
		vi.advanceTimersByTime(1000);
		expect(reveal).not.toHaveBeenCalled();

		scroller.remove();
		stop();
		vi.useRealTimers();
	});

	// iPhone: строка ввода внизу окна всегда вплотную к клавиатуре. Двигать её
	// некуда — попытки сдвигали страницу, экран мигал.
	it('поле у клавиатуры не трогается, если прокручивать нечего', () => {
		vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
			fn(0);
			return 0;
		});
		vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
		const vv = fakeViewport({ height: 800 });
		const stop = initViewportHeight();

		const scroller = document.createElement('div');
		scroller.style.overflowY = 'auto';
		const inside = document.createElement('input');
		const pinned = document.createElement('input');
		scroller.append(inside);
		document.body.append(scroller, pinned);
		const revealPinned = vi.fn();
		const revealInside = vi.fn();
		pinned.scrollIntoView = revealPinned;
		inside.scrollIntoView = revealInside;
		// Оба поля видны, но ближе 80 px к клавиатуре (область 480).
		for (const el of [pinned, inside]) {
			vi.spyOn(el, 'getBoundingClientRect').mockReturnValue({ top: 420, bottom: 464 } as DOMRect);
		}

		pinned.focus();
		vv.height = 480;
		vv.fire('resize');
		expect(revealPinned).not.toHaveBeenCalled();

		// Поле внутри экрана, который ещё прокручивается, — подводим.
		vi.spyOn(scroller, 'scrollHeight', 'get').mockReturnValue(1200);
		vi.spyOn(scroller, 'clientHeight', 'get').mockReturnValue(400);
		inside.focus();
		expect(revealInside).toHaveBeenCalledWith({ block: 'center' });

		scroller.remove();
		pinned.remove();
		stop();
	});

	// iPhone, приложение с экрана «Домой»: окно короче экрана, под строкой
	// ввода пустая полоса — высота названа рано, события visualViewport нет.
	it('высота перечитывается по событию окна и после запуска', () => {
		vi.useFakeTimers();
		vi.stubGlobal('requestAnimationFrame', (fn: FrameRequestCallback) => {
			fn(0);
			return 0;
		});
		vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
		const vv = fakeViewport({ height: 726 });
		const stop = initViewportHeight();
		const appH = () => document.documentElement.style.getPropertyValue('--app-h');
		expect(appH()).toBe('726px');

		// Окно выросло, visualViewport промолчал: хватает события окна…
		vv.height = 848;
		window.dispatchEvent(new Event('resize'));
		expect(appH()).toBe('848px');
		// …а без событий — проверки после запуска.
		vv.height = 880;
		vi.advanceTimersByTime(3000);
		expect(appH()).toBe('880px');

		stop();
		vi.useRealTimers();
	});

	it('увеличение щипком — не клавиатура, высоту не трогаем', () => {
		fakeViewport({ height: 400, scale: 2 });
		const stop = initViewportHeight();
		expect(document.documentElement.style.getPropertyValue('--app-h')).toBe('');
		stop();
	});
});

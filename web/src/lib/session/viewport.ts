// Высота окна приложения = видимая область, а не весь экран.
//
// Окно приложения (.ph.app) — во всю высоту и само не прокручивается. Когда
// открывается клавиатура, браузер, который не уменьшает страницу под неё
// (Firefox на Android, Safari), сдвигает страницу вверх целиком, чтобы поле
// ввода встало над клавиатурой: шапка и запись уезжали за верхний край, под
// короткой записью оставалась пустота. Здесь высота окна берётся из
// visualViewport (--app-h), а сдвиг документа сбрасывается — клавиатура
// сжимает окно, как в приложении. Браузеры, которые понимают
// interactive-widget=resizes-content (app.html), сжимают страницу сами.
//
// При увеличении щипком видимая область тоже меньше — тогда не трогаем:
// это масштаб, а не клавиатура.
//
// Окно сжалось — поле, в которое пишут, могло остаться под клавиатурой:
// браузер подводит к нему страницу, а страница здесь не едет, прокручивается
// экран внутри окна. Поэтому поле под нижним краем доводим до видимого сами.

/** Клавиатура заметно ниже любой панели браузера: меньшее сжатие — не она. */
const KEYBOARD_MIN_PX = 120;

function isTextField(el: Element | null): el is HTMLElement {
	if (!(el instanceof HTMLElement)) return false;
	if (el instanceof HTMLTextAreaElement || el.isContentEditable) return true;
	if (!(el instanceof HTMLInputElement)) return false;
	return !['button', 'checkbox', 'color', 'file', 'image', 'radio', 'range', 'reset', 'submit'].includes(
		el.type
	);
}

function revealFocusedField(visibleHeight: number): void {
	const el = document.activeElement;
	if (!isTextField(el)) return;
	const rect = el.getBoundingClientRect();
	if (rect.top >= 0 && rect.bottom <= visibleHeight) return;
	el.scrollIntoView({ block: 'center' });
}

export function initViewportHeight(): () => void {
	const vv = typeof window !== 'undefined' ? window.visualViewport : null;
	if (!vv) return () => {};
	const root = document.documentElement;
	let frame = 0;
	let lastHeight = 0;
	let fullHeight = 0;

	const apply = () => {
		frame = 0;
		if (Math.abs(vv.scale - 1) > 0.01) {
			root.style.removeProperty('--app-h');
			return;
		}
		const height = Math.round(vv.height);
		// Клавиатура может выезжать в несколько шагов: сравниваем с полной
		// высотой окна, а не с предыдущим шагом.
		fullHeight = Math.max(fullHeight, height);
		const shrunk = height < lastHeight && fullHeight - height >= KEYBOARD_MIN_PX;
		lastHeight = height;
		root.style.setProperty('--app-h', `${height}px`);
		if (window.scrollY !== 0 || vv.offsetTop !== 0) window.scrollTo(0, 0);
		if (shrunk) revealFocusedField(height);
	};
	const schedule = () => {
		if (!frame) frame = requestAnimationFrame(apply);
	};

	vv.addEventListener('resize', schedule);
	vv.addEventListener('scroll', schedule);
	apply();
	return () => {
		vv.removeEventListener('resize', schedule);
		vv.removeEventListener('scroll', schedule);
		if (frame) cancelAnimationFrame(frame);
		root.style.removeProperty('--app-h');
	};
}

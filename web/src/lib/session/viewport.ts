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

export function initViewportHeight(): () => void {
	const vv = typeof window !== 'undefined' ? window.visualViewport : null;
	if (!vv) return () => {};
	const root = document.documentElement;
	let frame = 0;

	const apply = () => {
		frame = 0;
		if (Math.abs(vv.scale - 1) > 0.01) {
			root.style.removeProperty('--app-h');
			return;
		}
		root.style.setProperty('--app-h', `${Math.round(vv.height)}px`);
		if (window.scrollY !== 0 || vv.offsetTop !== 0) window.scrollTo(0, 0);
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

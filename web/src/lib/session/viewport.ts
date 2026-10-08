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
//
// Одной проверки в момент сжатия мало (Vivaldi, 0.25.4): клавиатуру спрятали,
// фокус остался, нажали другое поле — оно оказалось под клавиатурой. Вслед за
// клавиатурой выезжает панель автозаполнения, браузер двигает экран сам, и
// порядок событий у браузеров разный. Поэтому проверяем ещё и при входе в
// поле, и несколько раз после — пока клавиатура встаёт.
//
// Одного события visualViewport мало (iPhone, приложение с экрана «Домой»,
// iOS 18.5, 0.28.2): окно приложения оказалось на 120 пунктов короче экрана,
// под строкой ввода — пустая полоса. Причина не установлена; высоту
// перечитываем ещё и по событиям окна и несколько раз после запуска — на
// случай, если Safari назвал её до того, как окно встало, и промолчал.
//
// iPhone: при касании поля Safari сам везёт страницу к нему, пока выезжает
// клавиатура, а мы в это же время сжимаем окно и возвращаем страницу — шапка
// на треть секунды оказывается посреди экрана и съезжает наверх (запись
// экрана, 0.28.4). Поэтому на iPhone поле получает фокус от нас, с
// preventScroll: страницу никто не везёт, окно просто сжимается.

/** Клавиатура заметно ниже любой панели браузера: меньшее сжатие — не она. */
const KEYBOARD_MIN_PX = 120;
/**
 * Запас над клавиатурой. Панель автозаполнения (ключ, карта, метка) в
 * установленном приложении на Chromium ложится поверх страницы и в видимую
 * область не входит: браузер ставил поле вплотную к клавиатуре, панель его
 * закрывала, а по числам поле было «видно» (Vivaldi, 0.25.7: область 460,
 * поле 409–458, видно на деле до 395).
 */
const KEYBOARD_BAR_PX = 80;
/** Когда перепроверить поле после сжатия окна или входа в поле. */
const RECHECK_MS = [150, 400, 900];
/** Когда перечитать высоту после запуска: событие об исправлении может не прийти. */
const STARTUP_RECHECK_MS = [300, 1000, 3000];

function isTextField(el: Element | null): el is HTMLElement {
	if (!(el instanceof HTMLElement)) return false;
	if (el instanceof HTMLTextAreaElement || el.isContentEditable) return true;
	if (!(el instanceof HTMLInputElement)) return false;
	return !['button', 'checkbox', 'color', 'file', 'image', 'radio', 'range', 'reset', 'submit'].includes(
		el.type
	);
}

/** Есть ли над полем экран, который ещё можно прокрутить вниз. */
function canScrollDown(el: HTMLElement): boolean {
	for (let node = el.parentElement; node && node !== document.body; node = node.parentElement) {
		const overflow = getComputedStyle(node).overflowY;
		if (overflow !== 'auto' && overflow !== 'scroll') continue;
		if (node.scrollHeight - node.clientHeight - node.scrollTop > 1) return true;
	}
	return false;
}

const TYPED_INPUTS = ['text', 'search', 'email', 'url', 'tel', 'password', 'number'];

/** Поле, в которое печатают с клавиатуры: у даты и файла свой выбор, их не трогаем. */
function isTypedField(el: Element | null): el is HTMLElement {
	if (el instanceof HTMLTextAreaElement) return !el.disabled && !el.readOnly;
	if (el instanceof HTMLInputElement) return TYPED_INPUTS.includes(el.type) && !el.disabled && !el.readOnly;
	return el instanceof HTMLElement && el.isContentEditable;
}

function isIOS(): boolean {
	return (
		/iP(hone|ad|od)/.test(navigator.userAgent) ||
		(navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
	);
}

/** Палец сдвинулся дальше — это прокрутка, а не касание поля. */
const TAP_SLOP_PX = 10;

/**
 * Касание поля не в фокусе — фокус ставим сами, без подвоза страницы.
 * Курсор при этом встаёт, куда его ставит браузер по умолчанию; место в
 * тексте выбирается вторым касанием.
 */
function focusWithoutScroll(): () => void {
	let start: { x: number; y: number } | undefined;
	const onStart = (event: TouchEvent) => {
		const touch = event.touches.length === 1 ? event.touches[0] : undefined;
		start = touch ? { x: touch.clientX, y: touch.clientY } : undefined;
	};
	const onEnd = (event: TouchEvent) => {
		const from = start;
		start = undefined;
		const touch = event.changedTouches[0];
		const target = event.target as Element | null;
		if (!from || !touch || !isTypedField(target) || document.activeElement === target) return;
		if (Math.abs(touch.clientX - from.x) > TAP_SLOP_PX || Math.abs(touch.clientY - from.y) > TAP_SLOP_PX) return;
		event.preventDefault();
		target.focus({ preventScroll: true });
	};
	document.addEventListener('touchstart', onStart, { passive: true });
	document.addEventListener('touchend', onEnd, { passive: false });
	return () => {
		document.removeEventListener('touchstart', onStart);
		document.removeEventListener('touchend', onEnd);
	};
}

function revealFocusedField(visibleHeight: number): boolean {
	const el = document.activeElement;
	if (!isTextField(el)) return false;
	const rect = el.getBoundingClientRect();
	if (rect.top >= 0 && rect.bottom <= visibleHeight - KEYBOARD_BAR_PX) return false;
	// Поле видно, но вплотную к клавиатуре, и прокручивать нечего — строка
	// ввода внизу окна стоит так всегда. Двигать её некуда: scrollIntoView
	// сдвигал саму страницу, мы возвращали её назад, и экран мигал на каждой
	// перепроверке (iPhone, 0.28.3: два касания поля — восемь попыток).
	if (rect.top >= 0 && rect.bottom <= visibleHeight && !canScrollDown(el)) return false;
	el.scrollIntoView({ block: 'center' });
	return true;
}

// Отладка на телефоне, где инструментов разработчика нет: «/?debug=viewport»
// включает строку с числами поверх экрана, «/?debug=off» убирает. Что браузер
// сообщает о клавиатуре, иначе не узнать — у каждого своё.
const DEBUG_KEY = 'wynd:debug-viewport';

function debugWanted(): boolean {
	try {
		const flag = new URLSearchParams(location.search).get('debug');
		if (flag === 'viewport') localStorage.setItem(DEBUG_KEY, '1');
		else if (flag === 'off') localStorage.removeItem(DEBUG_KEY);
		return localStorage.getItem(DEBUG_KEY) === '1';
	} catch {
		return false;
	}
}

function debugLine(): HTMLElement {
	const el = document.createElement('div');
	el.style.cssText =
		'position:fixed;left:0;right:0;top:0;z-index:99999;padding:3px 6px;background:#000;color:#0f0;font:10px/1.3 monospace;white-space:pre-wrap;pointer-events:none;';
	document.body.appendChild(el);
	return el;
}

export function initViewportHeight(): () => void {
	const vv = typeof window !== 'undefined' ? window.visualViewport : null;
	if (!vv) return () => {};
	const root = document.documentElement;
	let frame = 0;
	let lastHeight = 0;
	let fullHeight = 0;
	let timers: ReturnType<typeof setTimeout>[] = [];

	const stats = { resize: 0, scroll: 0, focus: 0, reveal: 0 };
	const debug = debugWanted() ? debugLine() : undefined;
	const report = () => {
		if (!debug) return;
		const el = document.activeElement;
		const rect = el?.getBoundingClientRect();
		debug.textContent =
			`vv ${Math.round(vv.height)} top ${Math.round(vv.offsetTop)} scale ${vv.scale.toFixed(3)}` +
			` · inner ${window.innerHeight} · doc ${root.clientHeight} · screen ${screen.height}` +
			` · app ${matchMedia('(display-mode: standalone)').matches ? 1 : 0} · scrollY ${Math.round(window.scrollY)}
` +
			`full ${fullHeight} last ${lastHeight} app-h ${root.style.getPropertyValue('--app-h') || '—'}` +
			` · kb ${fullHeight - lastHeight >= KEYBOARD_MIN_PX ? 'open' : 'no'}
` +
			`ev resize ${stats.resize} scroll ${stats.scroll} focus ${stats.focus} reveal ${stats.reveal}` +
			` · ${el?.tagName ?? '—'} ${rect ? `${Math.round(rect.top)}–${Math.round(rect.bottom)}` : ''}`;
	};
	const debugTimer = debug ? setInterval(report, 300) : undefined;

	const keyboardOpen = () => fullHeight - lastHeight >= KEYBOARD_MIN_PX;
	const reveal = () => {
		if (keyboardOpen() && revealFocusedField(lastHeight)) stats.reveal++;
	};
	const recheck = () => {
		timers.forEach(clearTimeout);
		timers = RECHECK_MS.map((ms) => setTimeout(reveal, ms));
	};
	const onFocusIn = (event: FocusEvent) => {
		if (!isTextField(event.target as Element | null)) return;
		stats.focus++;
		reveal();
		recheck();
	};

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
		if (shrunk) {
			if (revealFocusedField(height)) stats.reveal++;
			recheck();
		}
	};
	const schedule = (event?: Event) => {
		if (event?.type === 'resize') stats.resize++;
		else if (event) stats.scroll++;
		if (!frame) frame = requestAnimationFrame(apply);
	};

	const reread = () => schedule();
	const WINDOW_EVENTS = ['resize', 'orientationchange', 'pageshow'];
	vv.addEventListener('resize', schedule);
	vv.addEventListener('scroll', schedule);
	WINDOW_EVENTS.forEach((type) => window.addEventListener(type, reread));
	document.addEventListener('visibilitychange', reread);
	document.addEventListener('focusin', onFocusIn);
	const startup = STARTUP_RECHECK_MS.map((ms) => setTimeout(reread, ms));
	const stopFocus = isIOS() ? focusWithoutScroll() : undefined;
	apply();
	return () => {
		document.removeEventListener('focusin', onFocusIn);
		document.removeEventListener('visibilitychange', reread);
		WINDOW_EVENTS.forEach((type) => window.removeEventListener(type, reread));
		startup.forEach(clearTimeout);
		stopFocus?.();
		timers.forEach(clearTimeout);
		clearInterval(debugTimer);
		debug?.remove();
		vv.removeEventListener('resize', schedule);
		vv.removeEventListener('scroll', schedule);
		if (frame) cancelAnimationFrame(frame);
		root.style.removeProperty('--app-h');
	};
}

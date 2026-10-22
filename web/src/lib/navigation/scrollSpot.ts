/**
 * Прокрутка экрана с формой при шаге «Назад» (владелец, 2026-10-22: «возврат
 * идёт в начало экрана настроек, а не откуда ушёл»). Экраны прокручивают свою
 * область, а не окно, — браузер её положение не помнит, помним сами: по адресу
 * экрана, в sessionStorage, чтобы пережить и перезагрузку вкладки.
 */

const STORE = 'wynd:scroll';
let spots = new Map<string, number>();
try {
	spots = new Map(JSON.parse(sessionStorage.getItem(STORE) ?? '[]'));
} catch {
	/* нет хранилища — живём в памяти */
}

function save() {
	try {
		sessionStorage.setItem(STORE, JSON.stringify([...spots].slice(-50)));
	} catch {
		/* не страшно */
	}
}

export function scrollKey(url: URL): string {
	return url.pathname + url.search;
}

/** Уходим с экрана — запоминаем, где он был прокручен. */
export function rememberScroll(key: string, top: number): void {
	if (top > 0) spots.set(key, Math.round(top));
	else spots.delete(key);
	save();
}

/**
 * Вернулись на экран шагом назад: ставим прежнюю прокрутку. Содержимое часто
 * дорисовывается после открытия (данные с сервера), поэтому пробуем, пока
 * высоты не хватит, но не дольше `ms`.
 */
export function restoreScroll(el: HTMLElement, key: string, ms = 2000): () => void {
	const target = spots.get(key);
	if (!target) return () => {};
	const until = performance.now() + ms;
	let frame = 0;
	const step = () => {
		el.scrollTop = target;
		if (Math.abs(el.scrollTop - target) <= 1 || performance.now() > until) return;
		frame = requestAnimationFrame(step);
	};
	step();
	return () => cancelAnimationFrame(frame);
}

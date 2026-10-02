/**
 * Клавиатура не мигает при переходе между экранами с полем ввода.
 *
 * Полоса ввода ленты уходит на экран записи, когда текста стало много: поле
 * ленты исчезает, телефон прячет клавиатуру, а экран записи потом ставит
 * курсор в своё поле — клавиатура выезжает снова. Если фокус до исчезновения
 * старого поля перешёл в другое поле, телефон клавиатуру не прячет. Это поле —
 * невидимое, живёт в body вне страниц и переживает переход; новый экран
 * забирает фокус к себе. Не забрал за пару секунд — отпускаем.
 */
let keeper: HTMLTextAreaElement | undefined;
let release: ReturnType<typeof setTimeout> | undefined;

function keeperEl(): HTMLTextAreaElement {
	if (keeper?.isConnected) return keeper;
	const el = document.createElement('textarea');
	el.setAttribute('aria-hidden', 'true');
	el.tabIndex = -1;
	// 16px — иначе Safari приближает страницу при фокусе.
	el.style.cssText =
		'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;border:0;padding:0;font-size:16px;pointer-events:none;';
	document.body.appendChild(el);
	keeper = el;
	return el;
}

/** Перед переходом: держать открытую клавиатуру, пока новый экран не возьмёт фокус. */
export function holdKeyboard(): void {
	const active = document.activeElement;
	if (!(active instanceof HTMLTextAreaElement || active instanceof HTMLInputElement)) return;
	const el = keeperEl();
	el.focus({ preventScroll: true });
	clearTimeout(release);
	release = setTimeout(() => {
		if (document.activeElement === el) el.blur();
	}, 2500);
}

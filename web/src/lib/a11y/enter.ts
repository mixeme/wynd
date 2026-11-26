/**
 * Enter в поле запускает главное действие экрана: вход по почте, регистрацию,
 * смену адреса. После набора почты Enter — привычный способ, целиться в кнопку
 * мышью или Tab'ом не нужно (владелец, 26.11: в окне Настольного Enter не
 * отправлял код).
 *
 * `isComposing` — Enter подтверждает ввод иероглифа в IME; отправлять рано.
 * `keyCode === 229` — то же в старых WebView, где `isComposing` не выставлен.
 */
export function enterSubmit(fn: () => void) {
	return (event: KeyboardEvent) => {
		if (event.key !== 'Enter' || event.isComposing || event.keyCode === 229) return;
		event.preventDefault();
		fn();
	};
}

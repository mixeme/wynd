/**
 * Длинное нажатие и клик, который приходит следом.
 *
 * Экран «улочка» держал общий флаг `suppressClick`: таймер длинного нажатия
 * его поднимал, а каждый обработчик клика опускал — и в одном месте через
 * `queueMicrotask`, потому что порядок `pointerup` и `click` не гарантирован.
 * Это правилось в планах 31 и 41 и оставалось гонкой (GUI-6).
 *
 * Здесь вместо флага — отметка времени: клик, пришедший сразу за длинным
 * нажатием, принадлежит тому же жесту и глотается; клик позже — обычное
 * нажатие. Порядок событий больше ни на что не влияет.
 */
export const LONG_PRESS_MS = 500;

/** Клик в этом окне после длинного нажатия — хвост того же жеста. */
export const CLICK_TAIL_MS = 400;

/** Момент, когда сработало длинное нажатие; null — не срабатывало. */
export type PressMark = number | null;

export function markLongPress(now: number = Date.now()): PressMark {
	return now;
}

export function swallowsClick(mark: PressMark, now: number = Date.now()): boolean {
	if (mark === null) return false;
	return now - mark < CLICK_TAIL_MS;
}

// Toast уведомления окна по живым сигналам сервера (план 66, функция 2).
// Сервер шлёт рамку event: notify тем же правилом и из той же точки, что пуш:
// сигнал — тип, круг и имя круга, без текста записи (приватность пуша).
// Тексты — те же, что у push (web/static/push-sw.js); паритет держит тест.
// Правило «на виду — молча» применяет мост оболочки (App.Notify).

import { desktopBridge } from './bridge';

export interface NotifySignal {
	circle_id?: string;
	post_id?: string;
	type?: string;
	title?: string;
	body?: string;
}

const SIGNAL_TEXT: Record<string, string> = {
	post: 'Новая запись в круге',
	comment: 'Новый комментарий',
	reaction: 'Новая реакция',
	mention: 'Вас упомянули',
	invite: 'Вас позвали в круг',
	event: 'В круге изменения'
};

/** Короткая пачка одного круга и типа — один toast, не шторм. */
const THROTTLE_MS = 10_000;
const lastToast = new Map<string, number>();

export function notifySignalText(signal: NotifySignal): { title: string; body: string } | null {
	const type = typeof signal.type === 'string' ? signal.type : '';
	if (!type) return null;
	const title = typeof signal.title === 'string' && signal.title ? signal.title : 'Wynd';
	const body = (typeof signal.body === 'string' && signal.body) || SIGNAL_TEXT[type] || 'Новое в Wynd';
	return { title, body };
}

/**
 * Показать toast для сигнала. post и now — точки подмены в тестах.
 * В браузере (моста нет) сообщение никому не адресовано — не шлём вовсе.
 */
export function notifyFromSignal(
	signal: NotifySignal,
	post: (title: string, body: string) => void = postNotifyToShell,
	now: number = Date.now()
): void {
	if (!desktopBridge()) return;
	const text = notifySignalText(signal);
	if (!text) return;
	const key = `${signal.circle_id ?? ''}:${signal.type ?? ''}`;
	if (now - (lastToast.get(key) ?? 0) < THROTTLE_MS) return;
	lastToast.set(key, now);
	post(text.title, text.body);
}

function postNotifyToShell(title: string, body: string): void {
	window.parent?.postMessage({ type: 'wynd:notify', title, body }, window.location.origin);
}

/** Сброс дросселя — для тестов и для повторного подключения потока. */
export function resetNotifyThrottle(): void {
	lastToast.clear();
}

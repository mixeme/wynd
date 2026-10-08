import { NEWS } from './notes';

/**
 * Какую запись «Что нового» человек уже открывал или закрыл. Хранится на
 * устройстве: это отметка о баннере, а не настройка учётки — приложение
 * обновляется на каждом устройстве само по себе.
 */
// Ключ 0.26.0 (`wynd:news-seen`) брошен: та версия ставила отметку сама,
// когда список кругов пришёл пустым — в том числе из-за сбоя сети.
const KEY = 'wynd:news-read';

export function latestNewsVersion(): string {
	return NEWS[0]?.version ?? '';
}

export function hasUnseenNews(): boolean {
	const latest = latestNewsVersion();
	if (!latest) return false;
	try {
		return localStorage.getItem(KEY) !== latest;
	} catch {
		// Хранилища нет — баннер пришлось бы показывать вечно.
		return false;
	}
}

export function markNewsSeen(): void {
	try {
		localStorage.setItem(KEY, latestNewsVersion());
	} catch {
		/* не страшно */
	}
}

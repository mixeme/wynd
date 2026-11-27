import { searchLocation, type SearchChipState } from './search';
import type { CircleSearchHit } from './types';

/**
 * Память экрана поиска (C160): возврат с записи рисует прежние строки в первом
 * кадре, сеть уточняет следом. Экран поиска при возврате монтируется заново и
 * терял строки — на кадр появлялись «Ищем…» и пустое место, а поле снова
 * получало фокус (запись владельца 27.11 «И снова экран мигает»).
 *
 * Ключ — круг и адрес поиска: тот же запрос и фильтры в другом круге или в
 * общем поиске — другой экран. Держим несколько последних экранов: «Назад»
 * может вести не только на последний. Вытесняется давно не открытый.
 */
export type SearchScreenMemory = {
	key: string;
	hits: CircleSearchHit[];
	authors: string[];
	/** Адреса превью по id записи: кадр возврата не ждёт повторной выдачи. */
	thumbs: Record<string, string>;
	loading: boolean;
};

const LIMIT = 6;
const screens = new Map<string, SearchScreenMemory>();

function splitOrigin(origin: string): string {
	return origin.replace(/\/+$/, '');
}

/** Ключ экрана: круг, его сервер и канонический адрес поиска. */
export function searchScreenKey(
	origin: string,
	circleId: string,
	pathname: string,
	state: SearchChipState
): string {
	return `${splitOrigin(origin)}\u0000${circleId}\u0000${searchLocation(pathname, state)}`;
}

/** Экран показал строки — запоминаем, что именно было на экране. */
export function rememberSearchScreen(key: string, screen: Omit<SearchScreenMemory, 'key'>): void {
	// Перезапись двигает экран в конец: вытесняется давно не открытый.
	screens.delete(key);
	screens.set(key, { key, ...screen });
	while (screens.size > LIMIT) {
		const oldest = screens.keys().next().value;
		if (oldest === undefined) break;
		screens.delete(oldest);
	}
}

/** Прежние строки этого экрана, если их ещё помним. */
export function recallSearchScreen(key: string): SearchScreenMemory | undefined {
	return screens.get(key);
}

/** Выход из входа: чужие названия и отрывки не должны остаться в памяти. */
export function forgetSearchScreens(): void {
	screens.clear();
}

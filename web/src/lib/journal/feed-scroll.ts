// Место в ленте круга, пока открыта запись или альбом.
//
// Экран круга при каждом переходе строится заново ({#key} в layout круга),
// поэтому «Назад» с записи возвращал ленту в начало. Запоминаем не пиксели,
// а запись у верхнего края и её сдвиг: пока открыта запись, сверху могли
// прийти новые — по пикселям лента встала бы не туда.
//
// Держим в памяти вкладки: после перезапуска приложения лента и так
// открывается с начала.

export interface FeedSpot {
	postId: string;
	offset: number;
}

const spots = new Map<string, FeedSpot>();

export function feedSpotKey(origin: string, circleId: string): string {
	return `${origin}\n${circleId}`;
}

// Карточки опубликованных записей — прямые дети ленты, в порядке postIds.
// Записи в очереди (.q) стоят выше и в счёт не входят.
function postCards(feed: HTMLElement): HTMLElement[] {
	return Array.from(feed.querySelectorAll<HTMLElement>(':scope > .post:not(.q)'));
}

/** Первая запись, которая видна у верхнего края ленты. */
export function readFeedSpot(feed: HTMLElement, postIds: string[]): FeedSpot | undefined {
	if (feed.scrollTop <= 0) return undefined;
	const top = feed.getBoundingClientRect().top;
	const cards = postCards(feed);
	for (let i = 0; i < cards.length && i < postIds.length; i++) {
		const rect = cards[i].getBoundingClientRect();
		if (rect.bottom > top) return { postId: postIds[i], offset: rect.top - top };
	}
	return undefined;
}

export function saveFeedSpot(key: string, feed: HTMLElement | undefined, postIds: string[]) {
	const spot = feed ? readFeedSpot(feed, postIds) : undefined;
	if (spot) spots.set(key, spot);
	else spots.delete(key);
}

export function takeFeedSpot(key: string): FeedSpot | undefined {
	const spot = spots.get(key);
	spots.delete(key);
	return spot;
}

/** Ставит запись на прежнее место. Записи уже нет — лента остаётся как есть. */
export function applyFeedSpot(feed: HTMLElement, postIds: string[], spot: FeedSpot): boolean {
	const index = postIds.indexOf(spot.postId);
	if (index < 0) return false;
	const card = postCards(feed)[index];
	if (!card) return false;
	const delta = card.getBoundingClientRect().top - feed.getBoundingClientRect().top;
	feed.scrollTop += delta - spot.offset;
	return true;
}

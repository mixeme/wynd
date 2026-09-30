import { describe, expect, it } from 'vitest';
import { applyFeedSpot, feedSpotKey, readFeedSpot, saveFeedSpot, takeFeedSpot } from './feed-scroll';

// Лента с карточками высотой 300, верх ленты на 100 от окна. jsdom не
// считает раскладку — положение карточек выводим из scrollTop.
function fakeFeed(cards: number, queued = 0) {
	const feed = document.createElement('div');
	const top = 100;
	const all: HTMLElement[] = [];
	for (let i = 0; i < queued + cards; i++) {
		const el = document.createElement('div');
		el.className = i < queued ? 'post q' : 'post';
		feed.appendChild(el);
		all.push(el);
	}
	feed.getBoundingClientRect = () => ({ top }) as DOMRect;
	all.forEach((el, i) => {
		el.getBoundingClientRect = () => {
			const y = top + i * 300 - feed.scrollTop;
			return { top: y, bottom: y + 300 } as DOMRect;
		};
	});
	return feed;
}

describe('место в ленте', () => {
	it('запоминает запись у верхнего края и сдвиг', () => {
		const feed = fakeFeed(5);
		feed.scrollTop = 650;
		expect(readFeedSpot(feed, ['a', 'b', 'c', 'd', 'e'])).toEqual({ postId: 'c', offset: -50 });
	});

	it('в начале ленты помнить нечего', () => {
		const feed = fakeFeed(3);
		feed.scrollTop = 0;
		expect(readFeedSpot(feed, ['a', 'b', 'c'])).toBeUndefined();
	});

	it('сверху пришла новая запись — лента встаёт на ту же, а не на те же пиксели', () => {
		const feed = fakeFeed(5);
		feed.scrollTop = 0;
		expect(applyFeedSpot(feed, ['new', 'a', 'b', 'c', 'd'], { postId: 'b', offset: -50 })).toBe(true);
		expect(feed.scrollTop).toBe(650);
	});

	it('записи в очереди выше ленты в счёт не идут', () => {
		const feed = fakeFeed(4, 1);
		feed.scrollTop = 0;
		applyFeedSpot(feed, ['a', 'b', 'c', 'd'], { postId: 'b', offset: 0 });
		expect(feed.scrollTop).toBe(600);
	});

	it('запись удалили — место не трогаем', () => {
		const feed = fakeFeed(3);
		feed.scrollTop = 120;
		expect(applyFeedSpot(feed, ['a', 'c'], { postId: 'b', offset: 0 })).toBe(false);
		expect(feed.scrollTop).toBe(120);
	});

	it('место отдаётся один раз', () => {
		const feed = fakeFeed(3);
		feed.scrollTop = 350;
		const key = feedSpotKey('', 'circle');
		saveFeedSpot(key, feed, ['a', 'b', 'c']);
		expect(takeFeedSpot(key)).toEqual({ postId: 'b', offset: -50 });
		expect(takeFeedSpot(key)).toBeUndefined();
	});
});

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { notifyFromSignal, notifySignalText, resetNotifyThrottle } from './notify';
import { handleSyncFrame } from '../sync/sync';
import pushSw from '../../../static/push-sw.js?raw';

const withBridge = (present: boolean) => {
	const bag = window as unknown as { __wyndDesktop?: unknown };
	if (present) {
		bag.__wyndDesktop = { origin: 'https://home.example.org', apiBase: 'http://127.0.0.1:41234' };
	} else {
		delete bag.__wyndDesktop;
	}
};

beforeEach(() => {
	resetNotifyThrottle();
});

describe('notifySignalText', () => {
	it('берёт имя круга и свой текст по типу', () => {
		expect(notifySignalText({ type: 'post', title: 'Семья' })).toEqual({
			title: 'Семья',
			body: 'Новая запись в круге'
		});
	});

	it('сигнал без типа не превращается в toast', () => {
		expect(notifySignalText({ circle_id: 'c1' })).toBeNull();
	});

	it('неизвестный тип получает нейтральный текст', () => {
		expect(notifySignalText({ type: 'day_cover', title: 'Дача' })).toEqual({
			title: 'Дача',
			body: 'Новое в Wynd'
		});
	});
});

describe('notifyFromSignal', () => {
	it('в окне шлёт рамку оболочке', () => {
		withBridge(true);
		const post = vi.fn();
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post);
		expect(post).toHaveBeenCalledWith('Семья', 'Новая запись в круге');
	});

	it('в браузере не шлёт ничего', () => {
		withBridge(false);
		const post = vi.fn();
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post);
		expect(post).not.toHaveBeenCalled();
	});

	it('короткая пачка одного круга и типа даёт один toast', () => {
		withBridge(true);
		const post = vi.fn();
		const t0 = 1_000_000;
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post, t0);
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post, t0 + 3000);
		notifyFromSignal({ type: 'post', circle_id: 'c2', title: 'Дача' }, post, t0 + 4000);
		expect(post).toHaveBeenCalledTimes(2);
	});

	it('после тишины тот же круг уведомляется снова', () => {
		withBridge(true);
		const post = vi.fn();
		const t0 = 1_000_000;
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post, t0);
		notifyFromSignal({ type: 'post', circle_id: 'c1', title: 'Семья' }, post, t0 + 10_001);
		expect(post).toHaveBeenCalledTimes(2);
	});
});

// Паритет текстов с пушем: слова в toast окна и в push-уведомлении телефона
// совпадают, расхождение ловит тест, а не владелец (прецедент C88).
describe('паритет с push-sw.js', () => {
	it('тексты типов совпадают со службой push', () => {
		const block = pushSw.match(/const WYND_SIGNAL_TEXT = \{[\s\S]*?\};/);
		expect(block).toBeTruthy();
		const entries = [...block![0].matchAll(/(\w+):\s*'([^']+)'/g)].map(([, k, v]) => [k, v]);
		expect(entries.length).toBeGreaterThan(0);
		for (const [type, text] of entries) {
			expect(notifySignalText({ type })?.body).toBe(text);
		}
	});
});

// Рамка event: notify потока sync превращается в сообщение оболочке
// (wynd:notify); оболочка решает, показывать ли toast (окно скрыто).
describe('рамка notify потока sync', () => {
	it('в окне доходит до оболочки', async () => {
		withBridge(true);
		const send = vi.spyOn(window, 'postMessage').mockImplementation(() => {});
		try {
			await handleSyncFrame('https://wynd.example', '{"circle_id":"c1","type":"post","title":"Семья","count":1}');
			expect(send).toHaveBeenCalledWith(
				{ type: 'wynd:notify', title: 'Семья', body: 'Новая запись в круге' },
				window.location.origin
			);
		} finally {
			send.mockRestore();
		}
	});

	it('sync-событие уведомлением не считается', async () => {
		withBridge(true);
		const send = vi.spyOn(window, 'postMessage').mockImplementation(() => {});
		try {
			await handleSyncFrame('https://wynd.example', '{"seq":12,"id":"e1","circle_id":"c1","type":"post_created"}');
			expect(send).not.toHaveBeenCalledWith(expect.objectContaining({ type: 'wynd:notify' }), expect.anything());
		} finally {
			send.mockRestore();
		}
	});
});

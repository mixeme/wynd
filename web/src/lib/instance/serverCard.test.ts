import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadServerCard, registrationCaption } from './serverCard';
import { fetchInstance } from '$lib/auth/auth';
import { getSnapshot, putSnapshot } from '$lib/idb/db';

vi.mock('$lib/auth/auth', () => ({ fetchInstance: vi.fn() }));
vi.mock('$lib/idb/db', () => ({
	getSnapshot: vi.fn(),
	putSnapshot: vi.fn(),
	snapshotKey: (origin: string, kind: string, id: string) => `${origin}:${kind}:${id}`
}));
const known = {
	name: 'Дом Ани',
	registration_mode: 'invite' as const,
	allow_name_erasure: true,
	version: '0.43.8',
	loopback: false,
	bootstrapped: true
};
beforeEach(() => {
	vi.resetAllMocks();
	vi.mocked(putSnapshot).mockResolvedValue();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('server card information', () => {
	it('различает закрытую регистрацию и приглашения', () => {
		expect((['open', 'invite', 'closed'] as const).map(registrationCaption)).toEqual([
			'Открыт для новых',
			'Только по приглашению',
			'Регистрация закрыта'
		]);
	});
	// Сбой связи не превращает подтверждённую возможность в false.
	it('сохраняет известное имя и GDPR при недоступном сервере', async () => {
		vi.mocked(getSnapshot).mockResolvedValue({ data: known, fetched_at: 1 });
		vi.mocked(fetchInstance).mockRejectedValue(new Error('offline'));
		expect(await loadServerCard('https://one.example')).toEqual({
			info: known,
			connection: 'unavailable'
		});
	});
	// Сохранённые сведения другого origin не заменяют неизвестность.
	it('не переносит свойства между серверами', async () => {
		vi.mocked(getSnapshot).mockImplementation(async (key) =>
			key.startsWith('https://one.example:') ? { data: known, fetched_at: 1 } : undefined
		);
		vi.mocked(fetchInstance).mockRejectedValue(new Error('offline'));
		expect((await loadServerCard('https://two.example')).info).toBeUndefined();
	});
	it('свежий ответ заменяет прежнюю возможность, а ошибка кэша не скрывает связь', async () => {
		vi.mocked(getSnapshot).mockResolvedValue({ data: known, fetched_at: 1 });
		const fresh = { ...known, name: 'Новое имя', allow_name_erasure: undefined };
		vi.mocked(fetchInstance).mockResolvedValue(fresh);
		vi.mocked(putSnapshot).mockRejectedValue(new Error('no storage'));
		expect(await loadServerCard('')).toEqual({ info: fresh });
	});

	// C123, шаг 5: сервер старой версии без CORS слышен, но входу с чужого
	// адреса мешает его версия — это «устарел», а не «не отвечает».
	it('старый сервер без CORS распознаётся как устаревший', async () => {
		vi.mocked(fetchInstance).mockRejectedValue(new TypeError('Failed to fetch'));
		const probe = vi.fn().mockResolvedValue(new Response(null, { status: 200 }));
		vi.stubGlobal('fetch', probe);
		expect(await loadServerCard('https://old.example')).toEqual({ connection: 'outdated' });
		expect(probe).toHaveBeenCalledOnce();
		expect(probe.mock.calls[0][0]).toBe('https://old.example/api/v1/instance');
	});

	it('мёртвый сервер остаётся «не отвечает»', async () => {
		vi.mocked(fetchInstance).mockRejectedValue(new TypeError('Failed to fetch'));
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network down')));
		expect(await loadServerCard('https://down.example')).toEqual({ connection: 'unavailable' });
	});

	it('HTTP-ошибка живого сервера не считается устареванием', async () => {
		vi.mocked(fetchInstance).mockRejectedValue(new Error('500'));
		const probe = vi.fn();
		vi.stubGlobal('fetch', probe);
		expect(await loadServerCard('https://err.example')).toEqual({ connection: 'unavailable' });
		expect(probe).not.toHaveBeenCalled();
	});
});

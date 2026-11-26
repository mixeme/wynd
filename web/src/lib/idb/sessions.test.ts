import 'fake-indexeddb/auto';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	closeDb,
	deleteSession,
	getAdminSession,
	getSession,
	listSessions,
	migrateSessionsToSystemStorage,
	putSession
} from './db';

// Роутинг сессий (план 66, функция 3): в окне (apiBase) — учётные записи
// Windows через /__secrets локального порта, в браузере — IndexedDB.
// Откат в IndexedDB при недоступности хранилища окна.

const session = {
	origin: 'https://home.example.org',
	name: 'Аня',
	email: 'a@x.ru',
	token: 'tok',
	account_id: 'acc1'
};

function bagResponse(bag: unknown): Response {
	return new Response(JSON.stringify(bag), { status: 200 });
}

beforeEach(async () => {
	await closeDb();
	delete (window as unknown as { __wyndDesktop?: unknown }).__wyndDesktop;
	delete (window as unknown as { __wyndSecrets?: unknown }).__wyndSecrets;
	vi.restoreAllMocks();
	const db = await (await import('./db')).getDb();
	await db.clear('sessions');
	await db.delete('admin_session', 'admin');
});

afterEach(() => {
	vi.restoreAllMocks();
});

// Общее хранилище входов оболочки Android (C127): мешок по абсолютному адресу,
// как /__secrets окна. Клиент текущего сервера видит свой вход как пустой origin.
function androidStore(initial: Array<Record<string, unknown>> = []) {
	const bag = new Map<string, Record<string, unknown>>(
		initial.map((item) => [String(item.origin), item])
	);
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(raw: string) {
			const request = JSON.parse(raw);
			let result: unknown = {};
			if (request.op === 'list') result = { sessions: [...bag.values()] };
			else if (request.op === 'put') bag.set(request.session.origin, request.session);
			else if (request.op === 'delete') bag.delete(request.origin);
			setTimeout(() => target.onmessage?.({ data: JSON.stringify({ id: request.id, result }) }), 0);
		}
	};
	(window as unknown as { __wyndSecrets?: unknown }).__wyndSecrets = target;
	return bag;
}

describe('приложение Android (общий файл оболочки)', () => {
	it('хранит входы по абсолютному адресу, текущий сервер клиент видит пустым', async () => {
		const bag = androidStore();
		await putSession({ ...session, origin: '' });
		await putSession(session);
		expect(bag.has(window.location.origin)).toBe(true);
		expect(bag.has(session.origin)).toBe(true);
		const sessions = await listSessions();
		expect(sessions).toHaveLength(2);
		expect(sessions.find((item) => item.origin === '')?.origin).toBe('');
		expect((await getSession(''))?.token).toBe('tok');
		expect((await getSession(session.origin))?.origin).toBe(session.origin);
		await deleteSession('');
		expect(bag.has(window.location.origin)).toBe(false);
		expect(await getSession('')).toBeUndefined();
	});

	it('миграция выгружает прежние входы, не затирая более свежие', async () => {
		const bag = androidStore([
			{ ...session, origin: 'https://fresh.example.org', signed_in_at: '2026-11-25T12:00:00.000Z' }
		]);
		const db = await (await import('./db')).getDb();
		await db.put('sessions', { ...session, origin: '', signed_in_at: '2026-11-25T09:00:00.000Z' });
		await db.put('sessions', {
			...session,
			origin: 'https://old.example.org',
			signed_in_at: '2026-11-24T09:00:00.000Z'
		});
		await db.put('sessions', {
			...session,
			origin: 'https://fresh.example.org',
			token: 'stale',
			signed_in_at: '2026-11-25T08:00:00.000Z'
		});
		await migrateSessionsToSystemStorage();
		expect(bag.has(window.location.origin)).toBe(true);
		expect(bag.has('https://old.example.org')).toBe(true);
		expect(bag.get('https://fresh.example.org')?.token).toBe('tok');
		expect(await db.getAll('sessions')).toHaveLength(0);
	});

	it('сбой моста откатывает чтение и запись в IndexedDB', async () => {
		const failing = {
			onmessage: null as ((event: { data: string }) => void) | null,
			postMessage(raw: string) {
				const request = JSON.parse(raw);
				setTimeout(
					() =>
						failing.onmessage?.({
							data: JSON.stringify({ id: request.id, error: 'secrets_unavailable' })
						}),
					0
				);
			}
		};
		(window as unknown as { __wyndSecrets?: unknown }).__wyndSecrets = failing;
		await putSession(session);
		expect((await getSession(session.origin))?.token).toBe(session.token);
	});
});

describe('браузер (моста нет)', () => {
	it('сессии живут в IndexedDB, как раньше', async () => {
		await putSession(session);
		expect(await listSessions()).toHaveLength(1);
		expect((await getSession(session.origin))?.token).toBe('tok');
		await deleteSession(session.origin);
		expect(await listSessions()).toHaveLength(0);
	});
});

describe('окно (apiBase есть)', () => {
	beforeEach(() => {
		(window as unknown as { __wyndDesktop?: unknown }).__wyndDesktop = {
			origin: 'https://home.example.org',
			apiBase: 'http://127.0.0.1:41234'
		};
	});

	it('запись уходит в хранилище окна, профиль очищается от токена', async () => {
		const put = vi.spyOn(window, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
		await putSession(session);
		expect(put).toHaveBeenCalledWith(
			'http://127.0.0.1:41234/__secrets/session',
			expect.objectContaining({ method: 'PUT' })
		);
		const db = await (await import('./db')).getDb();
		expect(await db.getAll('sessions')).toHaveLength(0);
	});

	it('чтение берёт мешок хранилища окна', async () => {
		vi.spyOn(window, 'fetch').mockImplementation(async () =>
			bagResponse({ sessions: [session], admin: { origin: '', token: 'adm' } })
		);
		expect(await listSessions()).toHaveLength(1);
		expect((await getSession(session.origin))?.token).toBe('tok');
		expect((await getAdminSession())?.token).toBe('adm');
	});

	it('сбой хранилища окна откатывает чтение и запись в IndexedDB', async () => {
		vi.spyOn(window, 'fetch').mockRejectedValue(new Error('store down'));
		await putSession(session);
		expect((await getSession(session.origin))?.token).toBe('tok');
	});

	it('отказ ручки (старая оболочка) тоже откатывает в IndexedDB', async () => {
		vi.spyOn(window, 'fetch').mockResolvedValue(new Response('not found', { status: 404 }));
		await putSession(session);
		expect((await getSession(session.origin))?.token).toBe('tok');
	});

	it('вход текущего сервера хранится абсолютным, клиент видит пустой origin', async () => {
		const put = vi.spyOn(window, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
		await putSession({ ...session, origin: '' });
		const body = JSON.parse(String(put.mock.calls[0][1]?.body)) as { origin: string };
		expect(body.origin).toBe('https://home.example.org');

		put.mockImplementation(async () =>
			bagResponse({ sessions: [{ ...session, origin: 'https://home.example.org' }] })
		);
		expect((await getSession(''))?.token).toBe('tok');
		expect((await getSession(session.origin))?.origin).toBe('');
	});

	it('вход из IndexedDB виден, когда мешок окна пуст', async () => {
		const db = await (await import('./db')).getDb();
		await db.put('sessions', { ...session, origin: '' });
		vi.spyOn(window, 'fetch').mockResolvedValue(bagResponse({ sessions: [] }));
		expect(await listSessions()).toHaveLength(1);
		expect((await getSession(''))?.token).toBe('tok');
	});
});

describe('миграция в системное хранилище', () => {
	it('выгружает записи и очищает профиль; отказ оставляет IndexedDB целой', async () => {
		(window as unknown as { __wyndDesktop?: unknown }).__wyndDesktop = {
			origin: 'https://home.example.org',
			apiBase: 'http://127.0.0.1:41234'
		};
		// Засев напрямую в IndexedDB: putSession при мосте ушёл бы в окно.
		const db = await (await import('./db')).getDb();
		await db.put('sessions', session);
		await db.put('admin_session', { origin: '', token: 'adm' }, 'admin');

		const calls: string[] = [];
		vi.spyOn(window, 'fetch').mockImplementation(async (input, init) => {
			const url = String(input);
			calls.push(`${init?.method ?? 'GET'} ${url}`);
			if (url.endsWith('/__secrets/session')) return new Response(null, { status: 500 });
			if (url.endsWith('/__secrets/admin')) return new Response(null, { status: 204 });
			return new Response(null, { status: 404 });
		});

		// Выгрузка сессии падает — миграция останавливается: admin не тронут,
		// IndexedDB цела, повтор возможен при следующем запуске.
		await migrateSessionsToSystemStorage();
		expect(calls).toEqual(['PUT http://127.0.0.1:41234/__secrets/session']);
		expect(await db.getAll('sessions')).toHaveLength(1);
		expect(await db.get('admin_session', 'admin')).toBeDefined();

		// Повтор при здоровом хранилище выгружает всё и чистит.
		vi.spyOn(window, 'fetch').mockImplementation(async () => new Response(null, { status: 204 }));
		await migrateSessionsToSystemStorage();
		expect(await db.getAll('sessions')).toHaveLength(0);
		expect(await db.get('admin_session', 'admin')).toBeUndefined();
	});

	it('вход текущего сервера выгружается под абсолютным адресом', async () => {
		(window as unknown as { __wyndDesktop?: unknown }).__wyndDesktop = {
			origin: 'https://home.example.org',
			apiBase: 'http://127.0.0.1:41234'
		};
		const db = await (await import('./db')).getDb();
		await db.put('sessions', { ...session, origin: '' });
		const bodies: Array<{ origin: string }> = [];
		vi.spyOn(window, 'fetch').mockImplementation(async (_input, init) => {
			if (init?.body) bodies.push(JSON.parse(String(init.body)) as { origin: string });
			return new Response(null, { status: 204 });
		});
		await migrateSessionsToSystemStorage();
		expect(bodies).toEqual([expect.objectContaining({ origin: 'https://home.example.org' })]);
		expect(await db.getAll('sessions')).toHaveLength(0);
	});

	it('в браузере миграция не начинается', async () => {
		const fetchSpy = vi.spyOn(window, 'fetch');
		await migrateSessionsToSystemStorage();
		expect(fetchSpy).not.toHaveBeenCalled();
	});
});

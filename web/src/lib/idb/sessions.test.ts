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
	vi.restoreAllMocks();
});

afterEach(() => {
	vi.restoreAllMocks();
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

	it('в браузере миграция не начинается', async () => {
		const fetchSpy = vi.spyOn(window, 'fetch');
		await migrateSessionsToSystemStorage();
		expect(fetchSpy).not.toHaveBeenCalled();
	});
});

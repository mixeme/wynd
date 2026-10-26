import { afterEach, describe, expect, it, vi } from 'vitest';

const apiJson = vi.fn();
const apiFetch = vi.fn();

vi.mock('$lib/api/client', () => ({
	apiFetch: (...args: unknown[]) => apiFetch(...args),
	apiJson: (...args: unknown[]) => apiJson(...args)
}));

vi.mock('$lib/idb/db', () => ({
	invalidateSnapshots: vi.fn()
}));

import {
	archiveOwner,
	archivePeople,
	reviverIsStranger,
	uploadArchiveFiles,
	type LoadedArchive
} from './revive';

const MB = 1024 * 1024;

function archiveWith(size: number): LoadedArchive {
	const data = new Uint8Array(size);
	return {
		zip: {
			async *chunks(_entry: string, chunk: number) {
				for (let i = 0; i < size; i += chunk) yield data.subarray(i, Math.min(size, i + chunk));
			}
		},
		blobs: [
			{ id: 'b1', sha256: 'aa', size, mime: 'image/jpeg', filename: 'a.jpg', entry: 'media/b1.jpg' }
		],
		bytes: size
	} as unknown as LoadedArchive;
}

afterEach(() => {
	vi.useRealTimers();
	apiJson.mockReset();
	apiFetch.mockReset();
});

describe('uploadArchiveFiles', () => {
	it('переживает обрыв, даже когда первый вопрос «сколько принято» не прошёл', async () => {
		vi.useFakeTimers();
		apiJson.mockImplementation(async (_origin: string, path: string) => {
			if (path === '/revive/blobs') return { blobs: {} };
			if (path === '/uploads') return { id: 'u1' };
			return { id: 'blob-on-server' };
		});
		let received = 0;
		let putCalls = 0;
		let headCalls = 0;
		apiFetch.mockImplementation(async (_origin: string, _path: string, init: RequestInit) => {
			if (init.method === 'HEAD') {
				headCalls++;
				// Первый раз связи ещё нет.
				if (headCalls === 1) throw new TypeError('Failed to fetch');
				return new Response(null, { headers: { 'Upload-Offset': String(received) } });
			}
			putCalls++;
			// Второй кусок не ушёл: связь пропала.
			if (putCalls === 2) throw new TypeError('Failed to fetch');
			received += (init.body as Uint8Array).length;
			return new Response(JSON.stringify({ received_bytes: received }));
		});

		const done = uploadArchiveFiles('', archiveWith(3 * MB), () => {});
		await vi.runAllTimersAsync();
		await expect(done).resolves.toEqual({ b1: 'blob-on-server' });
		expect(received).toBe(3 * MB);
		expect(headCalls).toBe(1);
	});
});

// «Аня — это вы?» (A12, 7.29а–в): кто владелец по архиву, кто в круге и есть
// ли почта того, кто оживляет, среди участников.
describe('владелец и люди архива', () => {
	const t = (columns: string[], rows: unknown[][]) => ({ columns, rows });
	const tables = {
		circles: t(['id', 'owner_account_id'], [['c1', 'a1']]),
		accounts: t(
			['id', 'email'],
			[
				['a1', 'anya@example.org'],
				['a2', 'bob@example.org']
			]
		),
		memberships: t(
			['account_id', 'identity_id', 'status'],
			[
				['a1', 'i1', 'active'],
				['a2', 'i2', 'active'],
				['a3', 'i3', 'gone']
			]
		),
		identity_names: t(
			['identity_id', 'name', 'effective_at'],
			[
				['i1', 'Анечка', '2026-01-01T00:00:00Z'],
				['i1', 'Аня', '2026-05-01T00:00:00Z'],
				['i2', 'Боб', '2026-01-01T00:00:00Z'],
				['i3', 'Ушёл', '2026-01-01T00:00:00Z']
			]
		)
	};

	it('владелец — с почтой и последним именем', () => {
		expect(archiveOwner(tables)).toEqual({ email: 'anya@example.org', name: 'Аня' });
	});

	it('без почты владельца спросить не о ком', () => {
		expect(
			archiveOwner({ ...tables, accounts: t(['id', 'email'], [['a2', 'bob@example.org']]) })
		).toBeUndefined();
	});

	it('«Кто уже здесь» — без вышедших', () => {
		expect(archivePeople(tables)).toEqual(['Аня', 'Боб']);
	});

	it('почта в архиве — без оглядки на регистр', () => {
		const archive = { emails: ['anya@example.org', 'bob@example.org'] } as LoadedArchive;
		expect(reviverIsStranger(archive, 'Anya@Example.org')).toBe(false);
		expect(reviverIsStranger(archive, 'slava@example.org')).toBe(true);
	});
});

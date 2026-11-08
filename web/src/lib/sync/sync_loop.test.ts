import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Поток гоняется вечно; тестам хватает заглушек хранилища и сессии.
vi.mock('$lib/idb/db', () => ({
	getCursor: vi.fn(async () => ({ seq: 0 })),
	getSession: vi.fn(async () => ({ token: 't' })),
	listSessions: vi.fn(async () => []),
	putCursor: vi.fn(async () => {}),
	invalidateSnapshots: vi.fn(async () => {}),
	invalidateCircleSnapshots: vi.fn(async () => {})
}));
vi.mock('$lib/session/session.svelte', () => ({
	dropParticipantSession: vi.fn(async () => {}),
	isSessionRejected: () => false
}));
vi.mock('$lib/queue/queue', () => ({ drainQueueFor: vi.fn(async () => {}) }));
vi.mock('$lib/session/connection.svelte', () => ({
	forgetServer: vi.fn(),
	reportServerSilent: vi.fn(),
	reportServerUp: vi.fn()
}));

import { apiFetch } from '$lib/api/client';
import { reportServerSilent } from '$lib/session/connection.svelte';
import { reconnectSync, startSync, stopSync } from './sync';

// ApiError и normalizeOrigin остаются настоящими: их читают isTransportError и сам поток.
vi.mock('$lib/api/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/client')>();
	return { ...actual, apiFetch: vi.fn() };
});

const origin = 'https://wynd.example';

afterEach(() => {
	stopSync(origin);
	vi.clearAllMocks();
});

// Таймаут соединения и возврат сети (план 46, C77): повисший запрос не стоит
// дольше 30 с, а пауза после возврата сети начинается с 5 с, не с потолка.
describe('sync connect timeout and online reset', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('cancels a hung request after 30 s and counts it as a network failure', async () => {
		vi.mocked(apiFetch).mockImplementation(
			() => new Promise<Response>(() => {}) as Promise<Response>
		);
		startSync(origin);
		// Макротаск, чтобы цикл дошёл до запроса через свои await.
		await vi.advanceTimersByTimeAsync(1);
		expect(apiFetch).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(30_000);
		// Запрос отменён, а не брошен жить: свободных соединений у браузера шесть.
		const init = vi.mocked(apiFetch).mock.calls[0][2] as { signal: AbortSignal };
		expect(init.signal.aborted).toBe(true);
		// TimeoutError — сбой сети: сервер молчит, а не «кто-то отменил».
		expect(reportServerSilent).toHaveBeenCalledTimes(1);

		// После первой неудачи пауза 4–6 с — вторая попытка в пределах 7 с.
		await vi.advanceTimersByTimeAsync(7_000);
		expect(apiFetch).toHaveBeenCalledTimes(2);
	});

	it('starts the backoff over when the network returns', async () => {
		vi.mocked(apiFetch).mockRejectedValue(new TypeError('NetworkError'));
		startSync(origin);
		await vi.advanceTimersByTimeAsync(1);
		// Неудачи копятся: паузы дорастают до потолка 60 с.
		for (let i = 0; i < 5; i++) await vi.advanceTimersByTimeAsync(65_000);

		// Возврат сети: кик прерывает длинную паузу, попытка уходит сразу.
		reconnectSync();
		await vi.advanceTimersByTimeAsync(1);
		const rightAfterKick = vi.mocked(apiFetch).mock.calls.length;
		expect(rightAfterKick).toBeGreaterThan(5);

		// Эта попытка не удалась — но счётчик сброшен: следующая пауза 4–6 с,
		// не с потолка. До исправления здесь стояла бы минута.
		await vi.advanceTimersByTimeAsync(7_000);
		expect(vi.mocked(apiFetch).mock.calls.length).toBeGreaterThan(rightAfterKick);
	});
});

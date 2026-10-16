import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

let options: {
	onRegisteredSW?: (url: string, reg: ServiceWorkerRegistration | undefined) => void;
	onRegisterError?: (e: unknown) => void;
	onNeedRefresh?: () => void;
} = {};

vi.mock('virtual:pwa-register', () => ({
	registerSW: (o: typeof options) => {
		options = o;
		return async () => {};
	}
}));

function fakeWorker(state: string) {
	const listeners: (() => void)[] = [];
	return {
		state,
		addEventListener: (_: string, fn: () => void) => listeners.push(fn),
		set(next: string) {
			this.state = next;
			listeners.forEach((fn) => fn());
		}
	};
}

function fakeRegistration(over: Partial<Record<'installing' | 'waiting', unknown>> = {}) {
	return {
		installing: null,
		waiting: null,
		update: vi.fn().mockResolvedValue(undefined),
		...over
	} as unknown as ServiceWorkerRegistration;
}

const flush = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
	vi.resetModules();
	options = {};
});

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('appUpdate', () => {
	it('строка есть, даже когда регистрация на запуске не прошла', async () => {
		const reg = fakeRegistration();
		// Сразу воркера нет — он находится после ошибки регистрации.
		const getRegistration = vi.fn().mockResolvedValueOnce(undefined).mockResolvedValue(reg);
		vi.stubGlobal('navigator', { ...navigator, serviceWorker: { getRegistration } });
		const { appUpdate, initAppUpdate } = await import('./appUpdate.svelte');
		initAppUpdate();
		await flush();
		expect(appUpdate.supported).toBe(false);
		options.onRegisterError?.(new TypeError('Failed to fetch'));
		await flush();
		expect(appUpdate.supported).toBe(true);
	});

	it('пока новая версия скачивается — «загружаем», а не «последняя версия»', async () => {
		vi.useFakeTimers();
		const worker = fakeWorker('installing');
		const reg = fakeRegistration();
		(reg as unknown as { update: () => Promise<void> }).update = vi.fn(async () => {
			(reg as { installing: unknown }).installing = worker;
		});
		vi.stubGlobal('navigator', {
			...navigator,
			serviceWorker: { getRegistration: vi.fn().mockResolvedValue(reg) }
		});
		const { checkAppUpdate, initAppUpdate } = await import('./appUpdate.svelte');
		initAppUpdate();
		await vi.advanceTimersByTimeAsync(0);
		const onDownloading = vi.fn();
		const result = checkAppUpdate(onDownloading);
		await vi.advanceTimersByTimeAsync(60_000);
		expect(onDownloading).toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(60_000);
		await expect(result).resolves.toBe('downloading');
	});

	it('скачалась — «перейти на новую версию»', async () => {
		const worker = fakeWorker('installing');
		const reg = fakeRegistration();
		(reg as unknown as { update: () => Promise<void> }).update = vi.fn(async () => {
			(reg as { installing: unknown }).installing = worker;
		});
		vi.stubGlobal('navigator', {
			...navigator,
			serviceWorker: { getRegistration: vi.fn().mockResolvedValue(reg) }
		});
		const { appUpdate, checkAppUpdate, initAppUpdate } = await import('./appUpdate.svelte');
		initAppUpdate();
		await flush();
		const result = checkAppUpdate();
		await flush();
		(reg as { waiting: unknown }).waiting = worker;
		worker.set('installed');
		await expect(result).resolves.toBe('available');
		expect(appUpdate.pending).toBe(true);
	});
});

import { afterEach, describe, expect, it, vi } from 'vitest';

type FakeSentinel = { released: boolean; release: () => Promise<void> };

function installWakeLock() {
	const sentinels: FakeSentinel[] = [];
	const request = vi.fn(async () => {
		const s: FakeSentinel = {
			released: false,
			release: async () => {
				s.released = true;
			}
		};
		sentinels.push(s);
		return s;
	});
	Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true });
	return { request, sentinels };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('holdWakeLock', () => {
	afterEach(() => {
		// @ts-expect-error — убираем подмену между тестами
		delete navigator.wakeLock;
		vi.resetModules();
	});

	it('одна блокировка на нескольких держателей, снимается с последним', async () => {
		const { request, sentinels } = installWakeLock();
		const { holdWakeLock } = await import('./wake-lock');
		const a = holdWakeLock();
		const b = holdWakeLock();
		await flush();
		expect(request).toHaveBeenCalledTimes(1);
		a();
		a();
		await flush();
		expect(sentinels[0].released).toBe(false);
		b();
		await flush();
		expect(sentinels[0].released).toBe(true);
	});

	it('после возврата на вкладку берёт блокировку заново', async () => {
		const { request, sentinels } = installWakeLock();
		const { holdWakeLock } = await import('./wake-lock');
		const release = holdWakeLock();
		await flush();
		sentinels[0].released = true; // браузер снял, когда вкладку скрыли
		document.dispatchEvent(new Event('visibilitychange'));
		await flush();
		expect(request).toHaveBeenCalledTimes(2);
		release();
	});

	it('без Wake Lock API — ничего не делает', async () => {
		const { holdWakeLock } = await import('./wake-lock');
		expect(() => holdWakeLock()()).not.toThrow();
	});
});

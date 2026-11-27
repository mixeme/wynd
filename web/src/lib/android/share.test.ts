import { afterEach, describe, expect, it, vi } from 'vitest';
import { androidShare, androidShareAvailable } from './share';

const WINDOW = window as unknown as { __wyndShare?: unknown };

function bridge() {
	const value = {
		postMessage: vi.fn(),
		onmessage: null as ((event: { data: string }) => void) | null
	};
	WINDOW.__wyndShare = value;
	return value;
}

afterEach(() => {
	delete WINDOW.__wyndShare;
	vi.useRealTimers();
});

describe('androidShare — системный лист в оболочке', () => {
	it('без моста лист не открывается', async () => {
		expect(androidShareAvailable()).toBe(false);
		await expect(androidShare('https://w.example/invite/x', 'Приглашение')).resolves.toBe(false);
	});

	it('передаёт ссылку с заголовком и ждёт ответ оболочки', async () => {
		const port = bridge();
		expect(androidShareAvailable()).toBe(true);
		const shared = androidShare('https://w.example/invite/x', 'Приглашение в «Круг»');
		const request = JSON.parse(port.postMessage.mock.calls[0][0]);
		expect(request).toEqual({
			id: expect.any(Number),
			url: 'https://w.example/invite/x',
			title: 'Приглашение в «Круг»'
		});
		port.onmessage?.({ data: JSON.stringify({ id: request.id, shared: true }) });
		await expect(shared).resolves.toBe(true);
	});

	it('отказ листа не выдаётся за успех; молчание моста — отказ по времени', async () => {
		const port = bridge();
		const refused = androidShare('https://w.example/invite/x');
		const refusedId = JSON.parse(port.postMessage.mock.calls[0][0]).id;
		port.onmessage?.({ data: JSON.stringify({ id: refusedId, shared: false }) });
		await expect(refused).resolves.toBe(false);

		vi.useFakeTimers();
		const waiting = androidShare('https://w.example/invite/x');
		await vi.advanceTimersByTimeAsync(5000);
		await expect(waiting).resolves.toBe(false);
	});
});

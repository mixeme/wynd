import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { apiJson } from '$lib/api/client';
import { downloadFullArchive, streamFullArchive } from './fullarchive';

vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));

beforeEach(() => vi.mocked(apiJson).mockReset());
afterEach(() => {
	vi.restoreAllMocks();
	delete (navigator as Navigator & { standalone?: boolean }).standalone;
});

function iosWindow() {
	Object.defineProperty(navigator, 'standalone', { value: true, configurable: true });
	const child = {
		closed: false,
		opener: window as Window | null,
		location: { replace: vi.fn() },
		close: vi.fn()
	};
	const open = vi.spyOn(window, 'open').mockReturnValue(child as unknown as Window);
	return { child, open };
}

it('reserves the ready full archive download before issuing its link', async () => {
	const { child, open } = iosWindow();
	vi.mocked(apiJson).mockImplementationOnce(async () => {
		expect(open).toHaveBeenCalledOnce();
		return { url: '/api/v1/full-archive/token' };
	});
	await downloadFullArchive('https://other.example', 'c1');
	expect(child.location.replace).toHaveBeenCalledWith(
		'https://other.example/api/v1/full-archive/token'
	);
});

it('reserves the streaming window before asynchronous key preparation', async () => {
	const { child, open } = iosWindow();
	const key = vi.fn(async () => {
		expect(open).toHaveBeenCalledOnce();
		expect(apiJson).not.toHaveBeenCalled();
		return undefined;
	});
	vi.mocked(apiJson).mockResolvedValue({ url: '/api/v1/full-archive/stream-token' });
	await streamFullArchive('', 'c1', key, true);
	expect(key).toHaveBeenCalledOnce();
	expect(JSON.parse(vi.mocked(apiJson).mock.calls[0][2]!.body as string)).toEqual({
		delete_circle: true
	});
	expect(child.location.replace).toHaveBeenCalledWith('/api/v1/full-archive/stream-token');
});

it('closes the streaming window when key preparation fails without requesting a link', async () => {
	const { child } = iosWindow();
	const error = new Error('key failed');
	await expect(
		streamFullArchive('', 'c1', async () => {
			throw error;
		})
	).rejects.toBe(error);
	expect(child.close).toHaveBeenCalledOnce();
	expect(apiJson).not.toHaveBeenCalled();
});

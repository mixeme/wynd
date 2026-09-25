import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiJson } from '$lib/api/client';

vi.mock('$lib/api/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/client')>();
	return {
		...actual,
		apiJson: vi.fn()
	};
});

const FALLBACK = 'https://github.com/mixeme/wynd';

describe('source url', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.clearAllMocks();
	});

	it('falls back to the built-in address until the server answers', async () => {
		const source = await import('./source.svelte');
		expect(source.sourceUrl()).toBe(FALLBACK);
		expect(source.sourceUrl('https://other.example')).toBe(FALLBACK);
	});

	it('keeps an address per server, same origin under the empty key', async () => {
		const source = await import('./source.svelte');
		source.rememberSourceUrl('', 'https://git.example/mine');
		source.rememberSourceUrl('https://other.example', 'https://forge.example/fork');
		expect(source.sourceUrl()).toBe('https://git.example/mine');
		expect(source.sourceUrl('https://other.example')).toBe('https://forge.example/fork');
		// Чужой адрес не подменяет свой и наоборот.
		expect(source.sourceUrl('https://third.example')).toBe(FALLBACK);
	});

	it('ignores a server that does not send the field', async () => {
		const source = await import('./source.svelte');
		source.rememberSourceUrl('', undefined);
		source.rememberSourceUrl('', '');
		expect(source.sourceUrl()).toBe(FALLBACK);
	});

	it('loads the address once per origin', async () => {
		vi.mocked(apiJson).mockResolvedValue({ source_url: 'https://git.example/mine' });
		const source = await import('./source.svelte');
		source.loadSourceUrl();
		source.loadSourceUrl();
		await vi.waitFor(() => {
			expect(source.sourceUrl()).toBe('https://git.example/mine');
		});
		source.loadSourceUrl();
		expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(1);
	});

	it('survives a server that does not answer', async () => {
		vi.mocked(apiJson).mockRejectedValue(new Error('offline'));
		const source = await import('./source.svelte');
		source.loadSourceUrl('https://down.example');
		await vi.waitFor(() => {
			expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(1);
		});
		expect(source.sourceUrl('https://down.example')).toBe(FALLBACK);
	});
});

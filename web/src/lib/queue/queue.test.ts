import { beforeEach, describe, expect, it, vi } from 'vitest';

describe('queue', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.stubGlobal('navigator', { ...navigator, onLine: false });
	});

	it('enqueues and lists draft posts', async () => {
		const { enqueuePost, listQueuedPosts, CHUNK_SIZE } = await import('./queue');
		expect(CHUNK_SIZE).toBe(1024 * 1024);

		const id = await enqueuePost('https://srv.test', 'circle1', {
			body: 'hello',
			entry_date: '2026-09-02'
		});
		const posts = await listQueuedPosts('https://srv.test', 'circle1');
		expect(posts).toEqual([
			{
				id,
				body: 'hello',
				entry_date: '2026-09-02',
				file_count: 0,
				state: 'pending'
			}
		]);
	});

	it('updates and removes queued posts', async () => {
		const { enqueuePost, updateQueuedPost, removeQueueItem, listQueuedPosts } =
			await import('./queue');
		const id = await enqueuePost('https://srv.test', 'c1', {
			body: 'draft',
			entry_date: '2026-09-01'
		});
		await updateQueuedPost(id, { body: 'edited', entry_date: '2026-09-02' });
		expect((await listQueuedPosts('https://srv.test', 'c1'))[0].body).toBe('edited');
		await removeQueueItem(id);
		expect(await listQueuedPosts('https://srv.test', 'c1')).toHaveLength(0);
	});

	it('collects failed items with errors', async () => {
		const { getFailedItems } = await import('./queue');
		const failed = getFailedItems([
			{
				id: 1,
				type: 'post',
				origin: '',
				circle_id: 'c',
				payload: { body: '', entry_date: '2026-01-01' },
				files: [],
				state: 'failed',
				error: 'edit_window_closed',
				created_at: 0
			},
			{
				id: 2,
				type: 'comment',
				origin: '',
				circle_id: 'c',
				payload: { post_id: 'p', body: 'x' },
				files: [],
				state: 'pending',
				created_at: 0
			}
		]);
		expect(failed).toHaveLength(1);
		expect(failed[0].error).toBe('edit_window_closed');
	});
});

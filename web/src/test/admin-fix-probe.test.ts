import { flushSync, mount, unmount } from 'svelte';
import { readable } from 'svelte/store';
import { describe, expect, it, vi } from 'vitest';
import { putAdminSession } from '$lib/idb/db';

vi.mock('$app/stores', () => ({
	page: readable({ url: new URL('http://localhost/admin/fix') })
}));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$lib/admin/admin', () => ({
	fetchProxySnippet: vi.fn(async () => ({ snippet: '' })),
	serverCaption: vi.fn(async () => 'localhost')
}));
const probe = vi.fn(async (_origin: string) => ({
	x_forwarded_for: '203.0.113.7',
	x_real_ip: '203.0.113.7',
	client_ip: '203.0.113.7'
}));
vi.mock('$lib/admin/external-probe', () => ({ fetchProbeDiagnostics: probe }));

const { default: FixPage } = await import('../routes/admin/fix/+page.svelte');

// Инвариант: админ входит на своём сервере, origin его сессии — пустая
// строка. Экран «Исправить» всё равно спрашивает /probe и показывает
// заголовки прокси; раньше проверка origin на истинность пропускала пробу,
// и вместо значений всегда стояли прочерки.
describe('admin fix screen', () => {
	it('probes proxy headers for a same-origin admin session', async () => {
		await putAdminSession({ origin: '', token: 'admin-token' });
		const target = document.createElement('div');
		document.body.append(target);
		const instance = mount(FixPage, { target });
		await vi.waitFor(() => {
			flushSync();
			expect(target.textContent).toContain('X-Forwarded-For: 203.0.113.7');
		});
		expect(probe).toHaveBeenCalledWith('');
		unmount(instance);
		target.remove();
	});
});

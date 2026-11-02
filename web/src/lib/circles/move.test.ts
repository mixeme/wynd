import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import MovePage from '../../routes/circles/[id]/move/+page.svelte';
import { apiJson } from '$lib/api/client';
import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
import type { CircleMove, CircleMoveState } from './move';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));
vi.mock('$lib/navigation/up', () => ({ goUp: vi.fn(), setBackHandler: () => () => {} }));
vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));

const oldMove: CircleMove = {
	id: 'old',
	status: 'waiting',
	started_at: '2026-10-30T14:02:00Z',
	expires_at: '2026-11-02T14:02:00Z',
	link_alive: false,
	archive_status: 'none',
	archive_bytes: 0,
	pulled_bytes: 0
};
const state: CircleMoveState = {
	estimate: { posts: 3, members: 2, bytes: 100 },
	frozen_at: oldMove.started_at,
	move: oldMove
};
let dispose: (() => Promise<void>) | undefined;
beforeEach(() => vi.mocked(apiJson).mockReset());
afterEach(async () => {
	await dispose?.();
	dispose = undefined;
});

async function openPage() {
	const target = document.createElement('div');
	document.body.append(target);
	const circle = {
		origin: '',
		circleId: 'c1',
		name: 'Семья',
		color: 'terracotta',
		frozenAt: state.frozen_at,
		canWrite: false
	} as CircleContext;
	const instance = mount(MovePage, { target, context: new Map([[CIRCLE_CTX, circle]]) });
	dispose = async () => {
		await unmount(instance);
		target.remove();
	};
	await vi.waitFor(() => expect(target.textContent).toContain('Получить новую ссылку'));
	const button = [...target.querySelectorAll('button')].find((el) =>
		el.textContent?.includes('Получить новую ссылку')
	)!;
	return { target, button, circle };
}

describe('expired move link', () => {
	// Перевыпуск — одна операция; экран возвращается к обычной ссылке без отмены переноса.
	it('показывает новую ссылку и блокирует повторное нажатие во время запроса', async () => {
		let finish!: (move: CircleMove) => void;
		const pending = new Promise<CircleMove>((resolve) => {
			finish = resolve;
		});
		vi.mocked(apiJson).mockResolvedValueOnce(state).mockReturnValueOnce(pending);
		const { target, button, circle } = await openPage();
		button.click();
		flushSync();
		expect(button.disabled).toBe(true);
		button.click();
		finish({
			...oldMove,
			id: 'new',
			link_alive: true,
			link: 'https://old.example/move/new-token',
			expires_at: '2026-11-05T14:02:00Z'
		});
		await vi.waitFor(() =>
			expect(target.textContent).toContain('https://old.example/move/new-token')
		);
		expect(target.textContent).not.toContain('Получить новую ссылку');
		expect(circle.frozenAt).toBe(state.frozen_at);
		expect(circle.canWrite).toBe(false);
		const writes = vi.mocked(apiJson).mock.calls.filter((call) => call[2]?.method === 'POST');
		expect(writes).toEqual([['', '/circles/c1/move/reissue', { method: 'POST' }]]);
	});
	it('оставляет истёкшую ссылку и возможность повторить при ошибке', async () => {
		vi.mocked(apiJson).mockResolvedValueOnce(state).mockRejectedValueOnce(new Error('offline'));
		const { target, button } = await openPage();
		button.click();
		await vi.waitFor(() => expect(button.disabled).toBe(false));
		expect(target.textContent).toContain('Срок ссылки вышел');
		expect(target.textContent).not.toContain('начните заново');
	});
});

import { flushSync, mount, unmount } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import LoginPage from '../../routes/+page.svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));
vi.mock('$lib/navigation/up', () => ({ navigateBack: vi.fn() }));
vi.mock('$lib/session/session.svelte', async (importOriginal) => {
	const real = await importOriginal<typeof import('$lib/session/session.svelte')>();
	return {
		...real,
		initSession: vi.fn(async () => {}),
		loadSessions: vi.fn(async () => [])
	};
});
vi.mock('$lib/instance/serverCard', async (importOriginal) => {
	const real = await importOriginal<typeof import('$lib/instance/serverCard')>();
	return {
		...real,
		loadServerCard: vi.fn(async () => ({
			info: {
				name: 'Wynd',
				version: '0.54.1',
				registration_mode: 'invite',
				loopback: false,
				bootstrapped: true,
				code_delivery: 'mail'
			}
		}))
	};
});
vi.mock('$lib/auth/auth', async (importOriginal) => {
	const real = await importOriginal<typeof import('$lib/auth/auth')>();
	return { ...real, sendAuthCode: vi.fn(async () => ({ codeSentAt: Date.now() })) };
});

// Вход по почте: Enter в поле — привычное действие после набора адреса.
// Владелец 26.11: в окне Настольного Enter не отправлял код, приходилось
// целиться в кнопку. Сторож держит клавишу на экране входа.
describe('вход по Enter', () => {
	let target: HTMLElement | undefined;
	let instance: ReturnType<typeof mount> | undefined;

	afterEach(async () => {
		if (instance) await unmount(instance);
		instance = undefined;
		target?.remove();
		target = undefined;
	});

	it('Enter в поле почты отправляет код и ведёт на экран кода', async () => {
		const { goto } = await import('$app/navigation');
		const { sendAuthCode } = await import('$lib/auth/auth');
		vi.mocked(sendAuthCode).mockClear();
		vi.mocked(goto).mockClear();

		target = document.createElement('div');
		document.body.append(target);
		instance = mount(LoginPage, { target });

		const input = await vi.waitFor(() => {
			flushSync();
			const el = target?.querySelector<HTMLInputElement>('input[type="email"]');
			expect(el).toBeTruthy();
			return el as HTMLInputElement;
		});
		// Карточка сервера пришла: форма готова принять почту, кнопка активна.
		await vi.waitFor(() => {
			flushSync();
			expect(target?.textContent).toContain('Только по приглашению');
		});

		input.value = 'mix.public@ya.ru';
		input.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();
		const delivered = input.dispatchEvent(
			new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
		);
		// preventDefault — Enter обработан полем, а не потерян.
		expect(delivered).toBe(false);

		await vi.waitFor(() => {
			expect(sendAuthCode).toHaveBeenCalledOnce();
			expect(goto).toHaveBeenCalledWith('/auth/code');
		});
		expect(vi.mocked(sendAuthCode).mock.calls[0][0]).toMatchObject({
			email: 'mix.public@ya.ru',
			flow: 'login'
		});
	});
});

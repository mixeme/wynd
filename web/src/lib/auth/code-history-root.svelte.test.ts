import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import CodePage from '../../routes/auth/code/+page.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(async () => {}),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn(),
	pushState: vi.fn()
}));

vi.mock('$lib/auth/auth', async (importOriginal) => {
	const real = await importOriginal<typeof import('$lib/auth/auth')>();
	return {
		...real,
		verifyCode: vi.fn(async () => ({})),
		persistSession: vi.fn(async () => {})
	};
});

vi.mock('$lib/auth/pending', () => ({
	loadPendingAuth: vi.fn(() => ({
		origin: 'https://wynd.example',
		email: 'mix.public@ya.ru',
		flow: 'login' as const,
		codeDelivery: 'mail' as const
	})),
	clearPendingAuth: vi.fn(),
	canResendCode: vi.fn(() => false),
	resendCooldownSec: vi.fn(() => 0),
	applyRateLimitToPending: vi.fn(),
	savePendingAuth: vi.fn()
}));

vi.mock('$lib/navigation/afterLogin', () => ({ takeWantedPath: vi.fn(() => undefined) }));

// Улица — корень истории после входа (C162): экран кода заменяется улицей,
// и «Назад» с улочки в приложении закрывает его, как после запуска с
// сохранённым входом (C158). Запись кода поверх улицы оставляла за ней
// «настоящий» экран: выход не ставился, шаг уводил в код и снова на улицу.
describe('после кода улица заменяет запись кода', () => {
	let target: HTMLElement | undefined;
	let instance: ReturnType<typeof mount> | undefined;

	beforeEach(async () => {
		const { goto } = await import('$app/navigation');
		const { verifyCode, persistSession } = await import('$lib/auth/auth');
		const { takeWantedPath } = await import('$lib/navigation/afterLogin');
		vi.mocked(goto).mockClear();
		vi.mocked(verifyCode).mockClear();
		vi.mocked(persistSession).mockClear();
		vi.mocked(takeWantedPath).mockClear();
	});

	afterEach(async () => {
		if (instance) await unmount(instance);
		instance = undefined;
		target?.remove();
		target = undefined;
	});

	async function submitCode(): Promise<void> {
		target = document.createElement('div');
		document.body.append(target);
		instance = mount(CodePage, { target });

		const input = await vi.waitFor(() => {
			flushSync();
			const el = target?.querySelector<HTMLInputElement>('input.code-input');
			expect(el).toBeTruthy();
			return el as HTMLInputElement;
		});

		input.value = '123456';
		input.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();

		await vi.waitFor(async () => {
			const { verifyCode } = await import('$lib/auth/auth');
			expect(vi.mocked(verifyCode)).toHaveBeenCalledOnce();
		});
	}

	it(
		'обычный вход: улица с заменой записи, код отправлен один раз',
		{ timeout: 8000 },
		async () => {
			const { goto } = await import('$app/navigation');
			const { verifyCode } = await import('$lib/auth/auth');
			await submitCode();
			await vi.waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/circles', { replaceState: true });
			});
			// Барьер повторной отправки: без него эффект перезапускает отправку
			// после успеха (loading=false при неизменных code и pending).
			expect(verifyCode).toHaveBeenCalledOnce();
			expect(goto).toHaveBeenCalledOnce();
		}
	);

	it('путь из письма после входа — тоже с заменой записи', { timeout: 8000 }, async () => {
		const { goto } = await import('$app/navigation');
		const { takeWantedPath } = await import('$lib/navigation/afterLogin');
		vi.mocked(takeWantedPath).mockReturnValueOnce('/circles/abc/archive');
		await submitCode();
		await vi.waitFor(() => {
			expect(goto).toHaveBeenCalledWith('/circles/abc/archive', { replaceState: true });
		});
	});
});

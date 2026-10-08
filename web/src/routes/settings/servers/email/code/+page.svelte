<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { goUp } from '$lib/navigation/up';
	import Button from '$ui/forms/Button.svelte';
	import CodeBox from '$ui/forms/CodeBox.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { ApiError } from '$lib/api/client';
	import { applyRateLimitToPending, canResendCode, resendCooldownSec } from '$lib/auth/pending';
	import {
		clearPendingEmailChange,
		confirmEmailChange,
		emailChangeErrorHint,
		loadPendingEmailChange,
		requestEmailChange,
		savePendingEmailChange,
		type PendingEmailChange
	} from '$lib/auth/emailChange';

	let pending = $state<PendingEmailChange | undefined>();
	let code = $state('');
	let loading = $state(false);
	let error = $state('');
	let cooldown = $state(0);
	let codeInput: HTMLInputElement | undefined = $state();

	onMount(() => {
		pending = loadPendingEmailChange();
		if (!pending?.email) {
			goto('/settings/servers');
			return;
		}
		tickCooldown();
		const timer = setInterval(tickCooldown, 1000);
		queueMicrotask(() => codeInput?.focus());
		return () => clearInterval(timer);
	});

	function tickCooldown() {
		const p = loadPendingEmailChange();
		if (!p) return;
		cooldown = resendCooldownSec(p);
	}

	const logDelivery = $derived(pending?.codeDelivery === 'log');
	const pageTitle = $derived(logDelivery ? 'Код с сервера' : 'Код из письма');

	$effect(() => {
		if (code.length === 6 && pending && !loading) {
			void submitCode(code);
		}
	});

	async function submitCode(value: string) {
		if (!pending) return;
		loading = true;
		error = '';
		try {
			await confirmEmailChange(pending.origin, value);
			clearPendingEmailChange();
			// Форма и экран кода из истории уходят: «Назад» с «Серверов» — в настройки.
			await goto('/settings/servers', { replaceState: true });
		} catch (err) {
			error = emailChangeErrorHint(err, 'code');
			code = '';
		} finally {
			loading = false;
		}
	}

	async function resend() {
		if (!pending || !canResendCode(pending)) return;
		loading = true;
		error = '';
		try {
			await requestEmailChange(pending.origin, pending.email);
			const next = { ...pending, codeSentAt: Date.now(), retryUntil: undefined };
			savePendingEmailChange(next);
			pending = next;
			tickCooldown();
		} catch (err) {
			if (err instanceof ApiError && err.code === 'rate_limited' && err.retryAfterSec != null) {
				const next = applyRateLimitToPending(pending, err.retryAfterSec);
				savePendingEmailChange(next);
				pending = next;
				tickCooldown();
			}
			error = emailChangeErrorHint(err, 'email');
		} finally {
			loading = false;
		}
	}

	function changeEmail() {
		void goUp('/settings/servers/email');
	}
</script>

<FormLayout shell app title={pageTitle} onback={changeEmail}>
	{#if pending}
		{#if logDelivery}
			<Hint class="m-0-16">
				На этом компьютере письмо не уходит. Код напечатан в окне сервера.
			</Hint>
		{:else}
			<div class="m-0-16">
				Отправлен на <strong>{pending.email}</strong>
			</div>
		{/if}
		<div class="m-7-16-0">
			<TextButton class="link" onclick={changeEmail}>изменить адрес</TextButton>
		</div>
		<CodeBox bind:value={code} bind:el={codeInput} autofocus />
		<Hint>
			Код действует 15 минут. Три попытки. Пока код не введён, вход — по прежней почте.
		</Hint>
		<div class="rowin">
			<Button class="grow" disabled={!canResendCode(pending)} {loading} onclick={resend}>
				Отправить ещё раз
			</Button>
			{#if cooldown > 0}
				<span class="faint sz-12">через {cooldown} с</span>
			{/if}
		</div>
	{/if}
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>

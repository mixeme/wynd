<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import CodeBox from '$ui/forms/CodeBox.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import {
		authErrorHint,
		persistSession,
		sendAuthCode,
		verifyCode
	} from '$lib/auth/auth';
	import {
		canResendCode,
		clearPendingAuth,
		loadPendingAuth,
		resendCooldownSec,
		savePendingAuth,
		type PendingAuth
	} from '$lib/auth/pending';
	import { fetchCircles } from '$lib/circles/circles';
	import { getCircleIdentity, setCircleIdentity } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { saveInviteJoinToken } from '$lib/auth/invites';

	let pending = $state<PendingAuth | undefined>();
	let code = $state('');
	let loading = $state(false);
	let error = $state('');
	let cooldown = $state(0);
	let timer: ReturnType<typeof setInterval> | undefined;
	let codeInput: HTMLInputElement | undefined = $state();

	onMount(() => {
		pending = loadPendingAuth();
		if (!pending) {
			goto('/');
			return;
		}
		tickCooldown();
		timer = setInterval(tickCooldown, 1000);
		queueMicrotask(() => codeInput?.focus());
		return () => clearInterval(timer);
	});

	function tickCooldown() {
		const p = loadPendingAuth();
		if (!p) return;
		cooldown = resendCooldownSec(p);
	}

	const digits = $derived(code.replace(/\D/g, '').slice(0, 6).split(''));
	const active = $derived(Math.min(digits.length, 5));

	$effect(() => {
		const clean = code.replace(/\D/g, '').slice(0, 6);
		if (clean !== code) code = clean;
		if (clean.length === 6 && pending && !loading) {
			void submitCode(clean);
		}
	});

	async function submitCode(value: string) {
		if (!pending) return;
		loading = true;
		error = '';
		try {
			const result = await verifyCode(pending.origin, pending.email, value);
			await persistSession(pending.origin, result, pending.instanceName ?? '');
			const after = pending;
			clearPendingAuth();
			if (after.circleInvite && result.pending_circle_id && after.inviteToken) {
				saveInviteJoinToken(result.pending_circle_id, after.inviteToken);
				goto(`/circles/${result.pending_circle_id}/join`);
				return;
			}
			if (after.circleInvite && after.inviteName) {
				try {
					const circles = await fetchCircles(after.origin);
					const unset = [];
					for (const c of circles) {
						if (!(await getCircleIdentity(after.origin, c.id))) unset.push(c);
					}
					if (unset.length === 1) {
						await setCircleIdentity(after.origin, unset[0].id, after.inviteName);
						rememberCircleOrigin(unset[0].id, after.origin);
						goto(`/circles/${unset[0].id}/join`);
						return;
					}
				} catch {
					/* street is fine */
				}
			}
			goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
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
			const next = await sendAuthCode(pending);
			savePendingAuth(next);
			pending = next;
			tickCooldown();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	function changeEmail() {
		clearPendingAuth();
		goto('/');
	}
</script>

<FormLayout shell app title="Код из письма" onback={changeEmail}>
	{#if pending}
		<div style="margin:0 16px">
			Отправлен на <strong>{pending.email}</strong>
		</div>
		<div style="margin:7px 16px 0">
			<TextButton class="link" onclick={changeEmail}>изменить адрес</TextButton>
		</div>
		<div class="code-wrap">
			<input
				bind:this={codeInput}
				class="code-input"
				type="text"
				inputmode="numeric"
				autocomplete="one-time-code"
				maxlength="6"
				bind:value={code}
				aria-label="Код из шести цифр"
			/>
			<CodeBox {digits} {active} />
		</div>
		<Hint>Код действует 15 минут. Три попытки.</Hint>
		<div class="rowin">
			<Button
				style="flex:1"
				disabled={!canResendCode(pending)}
				{loading}
				onclick={resend}
			>
				Отправить ещё раз
			</Button>
			{#if cooldown > 0}
				<span style="color:var(--faint);font-size:12.5px">через {cooldown} с</span>
			{/if}
		</div>
	{/if}
	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>

<style>
	.code-wrap {
		position: relative;
	}
	.code-input {
		position: absolute;
		inset: 0;
		opacity: 0;
		width: 100%;
		height: 100%;
		font-size: 16px;
		border: none;
		background: transparent;
		caret-color: transparent;
	}
</style>

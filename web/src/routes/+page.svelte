<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Logo from '$ui/Logo.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import {
		authErrorHint,
		fetchInstance,
		sendAuthCode
	} from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { savePendingAuth } from '$lib/auth/pending';
	import { loadSessions } from '$lib/session/session.svelte';

	let email = $state('');
	let instanceName = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(async () => {
		const sessions = await loadSessions();
		if (sessions.length) {
			goto('/circles');
			return;
		}
		try {
			const info = await fetchInstance('');
			instanceName = info.name;
		} catch {
			error = 'Сервер недоступен';
		}
	});

	async function onSubmit() {
		error = '';
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		loading = true;
		try {
			const info = await fetchInstance('');
			const pending = {
				origin: '',
				email: trimmed,
				flow: 'login' as const,
				instanceName: info.name
			};
			await sendAuthCode(pending);
			savePendingAuth({ ...pending, codeSentAt: Date.now() });
			goto('/auth/code');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<PlainLayout shell app>
	<div class="logo-wrap">
		<Logo />
	</div>
	<div class="h1s ctr" style="margin-top:30px">Войти</div>
	<div class="hint ctr">
		Wynd не помнит устройств — только почту, которой вы называетесь на этом сервере.
	</div>
	<Label style="margin-top:26px">Сервер</Label>
	<FieldDisplay>
		<div style="font-weight:600">{instanceName || '…'}</div>
		<div style="font-size:12.5px;color:var(--muted)">{displayHost('')}</div>
	</FieldDisplay>
	<Label>Почта</Label>
	<Input active type="email" autocomplete="email" bind:value={email} />
	<Button {loading} onclick={onSubmit}>Получить код</Button>
	<div class="hint ctr" style="margin-top:30px">
		Впервые?
		<a class="under" href="/join">Прийти без приглашения</a><br />
		или откройте присланную ссылку.
	</div>
	{#if error}
		<Hint centered style="margin-top:12px">{error}</Hint>
	{/if}
</PlainLayout>

<style>
	.logo-wrap {
		margin: 38px 0 0;
		display: flex;
		justify-content: center;
		color: var(--ink);
	}
</style>

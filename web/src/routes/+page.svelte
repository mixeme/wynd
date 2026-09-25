<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Logo from '$ui/Logo.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import {
		authErrorHint,
		fetchInstance,
		sendAuthCode,
		type InstanceInfo
	} from '$lib/auth/auth';
	import { displayHost, resolveServerOrigin } from '$lib/auth/origin';
	import { loadPendingAuth, savePendingAuth } from '$lib/auth/pending';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import { appVersion } from '$lib/appinfo';

	let address = $state('');
	let email = $state('');
	let origin = $state('');
	let instance = $state<InstanceInfo | undefined>();
	let loading = $state(false);
	let checking = $state(false);
	let error = $state('');

	async function checkServer() {
		error = '';
		instance = undefined;
		const resolved = resolveServerOrigin(address);
		if (!resolved && !address.trim()) {
			origin = '';
			return;
		}
		checking = true;
		try {
			const info = await fetchInstance(resolved);
			origin = resolved;
			instance = info;
		} catch {
			error = 'Сервер не отвечает — проверьте адрес';
			origin = '';
		} finally {
			checking = false;
		}
	}

	onMount(async () => {
		await initSession();
		const sessions = await loadSessions();
		if (sessions.length) {
			goto('/circles');
			return;
		}
		const pending = loadPendingAuth();
		if (pending?.flow === 'login') {
			email = pending.email;
			if (pending.origin) {
				address = displayHost(pending.origin);
				void checkServer();
			}
		} else if (typeof window !== 'undefined' && window.location.hostname === '127.0.0.1') {
			address = '127.0.0.1:5173';
			void checkServer();
		}
	});

	async function onSubmit() {
		error = '';
		if (!instance) return;
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		loading = true;
		try {
			const pending = {
				origin,
				email: trimmed,
				flow: 'login' as const,
				instanceName: instance.name
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

	const serverSubtitle = $derived(
		instance ? `принимает вход · Wynd ${instance.version || appVersion}` : ''
	);
</script>

<PlainLayout shell app>
	<div class="logo-wrap">
		<Logo />
	</div>
	<div class="h1s ctr" style="margin-top:30px">Войти</div>
	<Label style="margin-top:26px">Адрес сервера</Label>
	<Input
		active
		mono
		style="font-size:12.5px"
		type="text"
		spellcheck="false"
		bind:value={address}
		onchange={checkServer}
		onblur={checkServer}
	/>
	{#if checking}
		<Hint style="margin-top:8px">Проверяем сервер…</Hint>
	{:else if instance}
		<ServerRow name={instance.name} subtitle={serverSubtitle} variant="ok" card />
	{/if}
	<Label style="margin-top:16px">Почта</Label>
	<Input active type="email" autocomplete="email" bind:value={email} />
	<Hint>Пришлём код для входа. Пароля нет.</Hint>
	<Button {loading} disabled={!instance} onclick={onSubmit}>Получить код</Button>
	<div class="hint ctr" style="margin-top:30px">
		<a class="under" href="/join">Регистрация без приглашения</a><br />
		Если прислали ссылку — откройте её.
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

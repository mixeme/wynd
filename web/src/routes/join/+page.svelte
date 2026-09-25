<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import {
		authErrorHint,
		fetchInstance,
		flowForJoin,
		sendAuthCode,
		type InstanceInfo
	} from '$lib/auth/auth';
	import { displayHost, resolveServerOrigin } from '$lib/auth/origin';
	import { savePendingAuth } from '$lib/auth/pending';
	import { appVersion } from '$lib/appinfo';

	let address = $state('');
	let email = $state('');
	let origin = $state('');
	let instance = $state<InstanceInfo | undefined>();
	let loading = $state(false);
	let checking = $state(false);
	let error = $state('');
	let blocked = $state(false);

	async function checkServer() {
		error = '';
		blocked = false;
		instance = undefined;
		const resolved = resolveServerOrigin(address);
		if (!resolved && !address.trim()) {
			return;
		}
		checking = true;
		try {
			const info = await fetchInstance(resolved);
			origin = resolved;
			instance = info;
			blocked = info.registration_mode !== 'open';
		} catch {
			error = 'Сервер не отвечает — проверьте адрес';
			origin = '';
		} finally {
			checking = false;
		}
	}

	onMount(() => {
		if (typeof window !== 'undefined' && window.location.hostname === '127.0.0.1') {
			address = '127.0.0.1:5173';
			void checkServer();
		}
	});

	async function onSubmit() {
		error = '';
		if (!instance || blocked) return;
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		const flow = flowForJoin(instance.registration_mode, false);
		if (flow === 'closed') {
			blocked = true;
			return;
		}
		loading = true;
		try {
			const pending = {
				origin,
				email: trimmed,
				flow,
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

	const subtitle = $derived(
		instance
			? `${displayHost(origin)} · ${instance.registration_mode === 'open' ? 'открыт для новых' : 'только по приглашению'} · Wynd ${instance.version || appVersion}`
			: ''
	);
</script>

<FormLayout shell app title={blocked ? 'Сервер не принимает' : 'Без приглашения'} onback={() => goto('/')}>
	{#if !blocked}
		<Hint>
			Учётка живёт на одном сервере. Общей на весь Wynd не бывает: серверы друг о друге не
			знают.
		</Hint>
		<Label style="margin-top:18px">Адрес сервера</Label>
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
			<ServerRow
				name={instance.name}
				{subtitle}
				variant={blocked ? 'warn' : 'ok'}
				card
			/>
		{/if}
		<Label style="margin-top:16px">Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>
			Пришлём код. Пароля не будет: почта понадобится, только чтобы вернуться на другом
			устройстве.
		</Hint>
		<Button
			{loading}
			disabled={!instance || blocked}
			onclick={onSubmit}
		>
			Получить код
		</Button>
		<Hint>
			Сервер хранит данные незашифрованными. Выбирайте сервер, которому доверяете, или
			<a class="link under" href="https://github.com/mixeme/wynd">поднимите свой</a>.
		</Hint>
	{:else}
		<Hint>Этот сервер не принимает новых участников без приглашения.</Hint>
		{#if instance}
			<ServerRow name={instance.name} subtitle={displayHost(origin)} variant="warn" card />
		{/if}
		<Button disabled onclick={() => {}}>Получить код</Button>
	{/if}
	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>

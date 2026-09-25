<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Button from '$ui/forms/Button.svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import {
		authErrorHint,
		completeBootstrap,
		fetchInstance
	} from '$lib/auth/auth';

	const token = $derived(page.url.searchParams.get('token') ?? '');

	let instanceName = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');
	let ready = $state(false);
	let alreadyDone = $state(false);

	onMount(async () => {
		try {
			const instance = await fetchInstance('');
			if (instance.bootstrapped) {
				alreadyDone = true;
			}
		} catch {
			error = 'Сервер не отвечает';
		} finally {
			ready = true;
		}
	});

	async function onSubmit() {
		error = '';
		if (!token) {
			error = 'Нет токена в ссылке — откройте URL из лога установки';
			return;
		}
		const name = instanceName.trim();
		if (!name) {
			error = 'Введите имя сервера';
			return;
		}
		if (!password) {
			error = 'Введите пароль администратора';
			return;
		}
		loading = true;
		try {
			await completeBootstrap({
				token,
				instance_name: name,
				password
			});
			goto('/admin/check');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<AdminWideLayout app height="auto" nav={false}>
	<AdminSection title="Сервер поднят">
		{#if !ready}
			<Hint>Проверяем сервер…</Hint>
		{:else if alreadyDone}
			<Hint>Администратор уже создан.</Hint>
			<Button onclick={() => goto('/admin')}>Открыть панель</Button>
		{:else}
			<Hint style="margin-bottom:18px">
				Осталось задать имя сервера и пароль администратора. Конфиг руками не
				редактируется — всё настраивается здесь.
			</Hint>
			<Label>Имя сервера</Label>
			<Input
				active
				type="text"
				autocomplete="organization"
				bind:value={instanceName}
				placeholder="Мой сервер"
			/>
			<Label style="margin-top:16px">Пароль администратора</Label>
			<Input active type="password" autocomplete="new-password" bind:value={password} />
			<Hint style="margin-top:8px">
				Единственный пароль на панель. Сброс — секретной ссылкой в логе.
			</Hint>
			<Button
				style="margin-top:20px"
				loading={loading}
				disabled={loading}
				onclick={loading ? () => {} : onSubmit}
			>
				Сохранить и открыть панель
			</Button>
			<Hint style="margin-top:16px">
				Страница открыта по одноразовой ссылке из лога. После сохранения сразу
				откроется проверка инстанса.
			</Hint>
		{/if}
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	</AdminSection>
</AdminWideLayout>

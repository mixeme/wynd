<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import Logo from '$ui/Logo.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Button from '$ui/forms/Button.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import { displayHost } from '$lib/auth/origin';
	import { decodeQrFromFile } from '$lib/auth/links';
	import {
		cancelInterfaceCheck,
		checkInterface,
		interfaceAvailable,
		interfaceInvite,
		selectInterface,
		takeScannedInvite,
		type CheckedInterface
	} from '$lib/android/interface';

	let address = $state('');
	let link = $state('');
	let targetPath = '/';
	let checked = $state<CheckedInterface>();
	let checking = $state(false);
	let selecting = $state(false);
	let failed = $state(false);
	let error = $state('');
	let photoPicker: FilePicker | undefined = $state();
	let timer: ReturnType<typeof setTimeout>;
	let generation = 0;
	let destroyed = false;

	function schedule(value: string, path = '/') {
		address = value;
		targetPath = path;
		const version = ++generation;
		clearTimeout(timer);
		cancelInterfaceCheck();
		checked = undefined;
		failed = false;
		error = '';
		checking = !!value.trim();
		if (!value.trim()) return;
		timer = setTimeout(async () => {
			try {
				const result = await checkInterface(value);
				if (!destroyed && version === generation) checked = result;
			} catch {
				if (!destroyed && version === generation) failed = true;
			} finally {
				if (!destroyed && version === generation) checking = false;
			}
		}, 450);
	}
	function invitation(value: string): boolean {
		link = value;
		const target = interfaceInvite(value);
		if (!target) {
			schedule('');
			if (value.trim()) error = 'Это не ссылка-приглашение Wynd';
			return false;
		}
		schedule(target.origin, target.path);
		return true;
	}
	async function photo(file: File) {
		try {
			invitation(await decodeQrFromFile(file));
		} catch {
			error = 'Не удалось прочитать код — попробуйте другое фото';
		}
	}
	async function next() {
		if (!checked || selecting) return;
		selecting = true;
		error = '';
		try {
			await selectInterface(checked.ticket, targetPath);
		} catch (reason) {
			selecting = false;
			if (reason instanceof Error && reason.message === 'busy') {
				error =
					'Сначала завершите сохранение файлов, выберите круг для полученного или отмените его.';
			} else if (reason instanceof Error && reason.message === 'storage') {
				error = 'Не удалось сохранить выбранный сервер. Попробуйте ещё раз.';
			} else schedule(address, targetPath);
		}
	}
	onMount(() => {
		if (!interfaceAvailable()) {
			void goto('/settings', { replaceState: true });
			return;
		}
		const scanned = takeScannedInvite();
		if (scanned) invitation(scanned);
	});
	onDestroy(() => {
		destroyed = true;
		generation++;
		clearTimeout(timer);
		cancelInterfaceCheck();
	});
</script>

<PlainLayout shell app class="android-interface">
	<div class="logo-wrap"><Logo /></div>
	<ScreenTitle class="mt-30" centered>Где ваш сервер?</ScreenTitle>
	<Label class="mt-18">Адрес сервера</Label>
	<Input
		active
		mono
		small
		value={address}
		disabled={selecting}
		type="text"
		spellcheck="false"
		autocomplete="url"
		autocapitalize="none"
		placeholder="home.example.org"
		oninput={(event) => {
			link = '';
			schedule(event.currentTarget.value);
		}}
	/>
	{#if checked}
		<ServerRow
			card
			name={checked.name || 'Сервер'}
			subtitle={displayHost(checked.origin)}
			gdpr={checked.allow_name_erasure}
		/>
	{:else}
		<Hint
			>{failed
				? 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.'
				: 'Адрес даёт тот, кто держит сервер. Он же стоит в начале ссылки-приглашения.'}</Hint
		>
	{/if}
	<Button disabled={!checked || checking} loading={selecting} onclick={() => void next()}
		>Дальше</Button
	>
	{#if checked}
		<Hint>Приложение будет открываться с этого сервера. Сменить его можно в настройках.</Hint>
	{:else}
		<Label class="mt-22">Есть приглашение</Label>
		<Input
			active
			mono
			small
			value={link}
			type="text"
			spellcheck="false"
			autocapitalize="none"
			placeholder="вставьте ссылку"
			oninput={(event) => invitation(event.currentTarget.value)}
		/>
		<Button variant="ghost" class="mt-14" onclick={() => goto('/settings/interface/scan')}
			>Сканировать QR-код</Button
		>
		<Button variant="ghost" onclick={() => photoPicker?.open()}>Фото с QR-кодом</Button>
	{/if}
	{#if error}<Hint>{error}</Hint>{/if}
</PlainLayout>
<FilePicker
	bind:this={photoPicker}
	accept="image/*"
	onfiles={([file]) => file && void photo(file)}
/>

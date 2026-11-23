<!-- План 50: три кадра приняты 17.11.2026, основа 232adff3 /dev/smoke/android-interface. -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { navigateBack } from '$lib/navigation/up';
	import { displayHost } from '$lib/auth/origin';
	import {
		androidInterfacesAvailable,
		loadInterfaces,
		previewInterface,
		addInterface,
		removeInterface,
		selectInterface,
		InterfaceError,
		type InterfaceCatalog,
		type InterfaceServer
	} from '$lib/android/interfaces';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Loading from '$ui/Loading.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';

	let data = $state<InterfaceCatalog | null>(null);
	let adding = $state(false);
	let address = $state('');
	let preview = $state<InterfaceServer | null>(null);
	let removeTarget = $state<InterfaceServer | null>(null);
	let checking = $state(false);
	let busy = $state(false);
	let error = $state('');
	let generation = 0;
	let stopped = false;

	onMount(() => {
		if (!androidInterfacesAvailable()) void goto('/settings/app', { replaceState: true });
		else
			void loadInterfaces()
				.then((value) => {
					if (!stopped) data = value;
				})
				.catch(() => {
					if (!stopped) error = 'Не удалось прочитать список серверов. Попробуйте ещё раз.';
				});
		const back = (event: Event) => {
			if (event.defaultPrevented) return;
			if (busy || adding) {
				event.preventDefault();
				if (!busy) closeAdd();
			}
		};
		window.addEventListener('wynd:android-back', back);
		return () => {
			stopped = true;
			generation++;
			window.removeEventListener('wynd:android-back', back);
		};
	});
	beforeNavigate((navigation) => {
		if (busy) navigation.cancel();
		else if (adding && navigation.type === 'popstate') {
			navigation.cancel();
			closeAdd();
		}
	});
	function resetPreview() {
		preview = null;
		error = '';
		checking = false;
		generation++;
	}
	function openAdd() {
		if (busy) return;
		address = '';
		resetPreview();
		adding = true;
	}
	function closeAdd() {
		if (busy) return;
		resetPreview();
		adding = false;
	}
	function back() {
		if (busy) return;
		if (adding) closeAdd();
		else void navigateBack();
	}
	async function checkAddress() {
		const value = address.trim(),
			current = ++generation;
		preview = null;
		error = '';
		if (!value) {
			checking = false;
			return;
		}
		checking = true;
		try {
			const checked = await previewInterface(value);
			if (!stopped && current === generation) preview = checked;
		} catch {
			if (!stopped && current === generation)
				error = 'Сервер Wynd по этому адресу не отвечает. Проверьте адрес и сеть.';
		} finally {
			if (!stopped && current === generation) checking = false;
		}
	}
	async function addChecked() {
		if (!preview || busy || checking) return;
		busy = true;
		error = '';
		try {
			data = await addInterface(preview.origin);
			adding = false;
			resetPreview();
		} catch {
			error = 'Не удалось добавить сервер интерфейса. Попробуйте ещё раз.';
		} finally {
			busy = false;
		}
	}
	async function choose(origin: string) {
		if (busy || origin === data?.selected) return;
		busy = true;
		error = '';
		try {
			await selectInterface(origin);
		} catch (failure) {
			error =
				failure instanceof InterfaceError && failure.code === 'busy'
					? 'Сначала завершите текущее действие или отмените его.'
					: 'Не удалось открыть сервер. Попробуйте ещё раз.';
			busy = false;
		}
	}
	async function confirmRemove() {
		if (!removeTarget || busy) return;
		busy = true;
		error = '';
		try {
			data = await removeInterface(removeTarget.origin);
		} catch {
			error = 'Не удалось убрать сервер из списка. Попробуйте ещё раз.';
		} finally {
			removeTarget = null;
			busy = false;
		}
	}
</script>

{#if adding}
	<FormLayout shell app title="Добавить сервер интерфейса" onback={back}>
		<Label>Адрес сервера</Label>
		<Input
			active
			mono
			small
			bind:value={address}
			placeholder="wynd.example.org"
			aria-label="Адрес сервера"
			type="text"
			spellcheck={false}
			autocomplete="off"
			disabled={busy}
			oninput={resetPreview}
			onblur={() => void checkAddress()}
			onkeydown={(event: KeyboardEvent) => {
				if (event.key === 'Enter') {
					if (preview) void addChecked();
					else (event.currentTarget as HTMLInputElement).blur();
				}
			}}
		/>
		{#if preview}
			<ServerRow
				name={preview.name}
				subtitle={displayHost(preview.origin)}
				detail="Сервер доступен"
				card
			/>
		{:else if checking}<Hint class="mt-8">Проверяем сервер…</Hint>{/if}
		<Hint>Сервер появится в списке источников интерфейса. Вход и учётная запись не создаются.</Hint>
		{#if error}<Hint>{error}</Hint>{/if}
		<Button
			disabled={!preview || checking}
			loading={busy}
			loadingLabel="Добавляем…"
			onclick={() => void addChecked()}>Добавить сервер интерфейса</Button
		>
	</FormLayout>
{:else}
	<FormLayout shell app title="Сервер интерфейса" onback={back}>
		{#if data}
			<Hint class="mt-16"
				>Приложение загружает интерфейс с выбранного сервера. Ваши входы и круги останутся без
				изменений.</Hint
			>
			<Label>Серверы интерфейса</Label>
			{#each data.servers as server (server.origin)}
				{#if server.origin === data.selected}
					<ServerRow
						name={server.name}
						subtitle={displayHost(server.origin)}
						variant="ok"
						onclick={() => {}}
					/>
				{:else}
					<ServerRow
						name={server.name}
						subtitle={displayHost(server.origin)}
						onclick={() => void choose(server.origin)}
						onremove={() => (removeTarget = server)}
						removeDisabled={busy}
					/>
				{/if}
			{/each}
			{#if error}<Hint class="mt-16">{error}</Hint>{/if}
			<Button disabled={busy} onclick={openAdd}>Добавить сервер интерфейса</Button>
		{:else if error}<Hint>{error}</Hint>{:else}<Loading compact />{/if}
		{#if removeTarget}
			<ConfirmDialog
				title={`Убрать сервер «${removeTarget.name}» из списка?`}
				confirmLabel="Убрать"
				cancelLabel="Оставить"
				loading={busy}
				onconfirm={() => void confirmRemove()}
				oncancel={() => {
					if (!busy) removeTarget = null;
				}}
			>
				<Hint class="mb-16"
					>Сервер исчезнет только из источников интерфейса. Вход и данные останутся. Добавить его
					снова можно по адресу.</Hint
				>
			</ConfirmDialog>
		{/if}
	</FormLayout>
{/if}

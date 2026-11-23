<!-- План 50: предварительный рендер, не реализация Android-моста.
     Основа: план 62, flow-change-standard / flow-change-confirm (55f1f9d2),
     flow-add-interface; действующая оболочка 3a4ec9b1 + 53a1e87e.
     Только демонстрационные данные в памяти; сеть и хранилище не меняются. -->
<script lang="ts">
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';

	const examples = [
		{ name: 'Дом Ани', host: 'home.example.org' },
		{ name: 'У Славы', host: 'wynd.u-slavy.ru' }
	];
	const additional = { name: 'Файлы', host: 'files.example.org' };
	let servers = $state([...examples]);
	let selected = $state(examples[0].host);
	let adding = $state(false);
	let address = $state('');
	let preview = $state(false);
	let removeTarget = $state<(typeof examples)[number] | null>(null);

	function openAdd() {
		address = '';
		preview = false;
		adding = true;
	}

	function addExample() {
		if (!preview) return;
		if (!servers.some((server) => server.host === additional.host)) {
			servers = [...servers, additional];
		}
		adding = false;
	}
</script>

<div class="preview">
	{#if adding}
		<FormLayout shell app title="Добавить сервер интерфейса" onback={() => (adding = false)}>
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
				oninput={() => (preview = false)}
				onblur={() => (preview = address.trim() === additional.host)}
			/>
			{#if preview}
				<ServerRow
					name={additional.name}
					subtitle={additional.host}
					detail="Сервер доступен"
					card
				/>
			{/if}
			<Hint
				>Сервер появится в списке источников интерфейса. Вход и учётная запись не создаются.</Hint
			>
			<Button disabled={!preview} onclick={addExample}>Добавить сервер интерфейса</Button>
		</FormLayout>
	{:else}
		<FormLayout shell app title="Сервер интерфейса" onback={() => {}}>
			<Hint class="mt-16"
				>Приложение загружает интерфейс с выбранного сервера. Ваши входы и круги останутся без
				изменений.</Hint
			>
			<Label>Серверы интерфейса</Label>
			{#each servers as server (server.host)}
				{#if server.host === selected}
					<ServerRow name={server.name} subtitle={server.host} variant="ok" onclick={() => {}} />
				{:else}
					<div class="removable-server">
						<ServerRow
							name={server.name}
							subtitle={server.host}
							style="padding-right:52px"
							onclick={() => (selected = server.host)}
						/>
						<IconButton
							name="x"
							size="sm"
							label={`Убрать сервер «${server.name}» из списка`}
							onclick={() => (removeTarget = server)}
						/>
					</div>
				{/if}
			{/each}
			<Button onclick={openAdd}>Добавить сервер интерфейса</Button>
			{#if removeTarget}
				<ConfirmDialog
					title={`Убрать сервер «${removeTarget.name}» из списка?`}
					confirmLabel="Убрать"
					cancelLabel="Оставить"
					onconfirm={() => {
						servers = servers.filter((server) => server.host !== removeTarget?.host);
						removeTarget = null;
					}}
					oncancel={() => (removeTarget = null)}
				>
					<Hint class="mb-16"
						>Сервер исчезнет только из источников интерфейса. Вход и данные останутся. Добавить его
						снова можно по адресу.</Hint
					>
				</ConfirmDialog>
			{/if}
		</FormLayout>
	{/if}
</div>

<style>
	/* Только стенд: снимает внешние поля галереи /dev, не меняя Wynd UI. */
	.preview {
		position: fixed;
		inset: 0;
	}
	/* Без изменений из cmd/wynd-desktop/frontend/src/app.css. */
	.removable-server {
		position: relative;
	}
	.removable-server :global(.ib) {
		position: absolute;
		right: 16px;
		top: 50%;
		transform: translateY(-50%);
	}
</style>

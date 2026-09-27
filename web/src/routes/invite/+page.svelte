<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { decodeQrFromFile, foreignWyndLinkOrigin, parseWyndLink } from '$lib/auth/links';

	// 2.16: куда вставить присланную ссылку. В установленном приложении ссылка
	// из мессенджера открывается в браузере, а не здесь, — человеку с пустой
	// улочкой нужно место, куда её положить.
	let link = $state('');
	let error = $state('');
	let photoInput: HTMLInputElement | undefined = $state();

	const foreignLinkError = (origin: string) =>
		`Ссылка ведёт на другой сервер (${origin}) — откройте её там`;

	function open(text: string): boolean {
		error = '';
		const foreign = foreignWyndLinkOrigin(text);
		if (foreign) {
			error = foreignLinkError(foreign);
			return false;
		}
		const path = parseWyndLink(text);
		if (!path) {
			error = 'Это не ссылка-приглашение Wynd';
			return false;
		}
		goto(path);
		return true;
	}

	function onPaste(event: ClipboardEvent) {
		const text = event.clipboardData?.getData('text') ?? '';
		if (parseWyndLink(text) || foreignWyndLinkOrigin(text)) {
			event.preventDefault();
			link = text.trim();
			open(text);
		}
	}

	async function onPhotoSelected() {
		const file = photoInput?.files?.[0];
		if (!file) return;
		error = '';
		try {
			open(await decodeQrFromFile(file));
		} catch (err) {
			error =
				err instanceof Error && err.message === 'no_detector'
					? 'Сканер QR недоступен в этом браузере — вставьте ссылку в поле'
					: 'Не удалось прочитать код — попробуйте другое фото';
		} finally {
			if (photoInput) photoInput.value = '';
		}
	}
</script>

<FormLayout shell app title="Приглашение" onback={() => goto('/circles')}>
	<Hint>Вставьте ссылку, которую вам прислали, или выберите фото с QR-кодом.</Hint>
	<Label>Ссылка</Label>
	<Input
		active
		mono
		type="text"
		spellcheck="false"
		bind:value={link}
		onpaste={onPaste}
		placeholder="https://…/invite/…"
	/>
	<Button disabled={!link.trim()} onclick={() => open(link)}>Открыть</Button>
	<Button variant="ghost" onclick={() => photoInput?.click()}>Фото с QR-кодом</Button>
	{#if error}
		<Hint>{error}</Hint>
	{/if}
</FormLayout>

<input
	bind:this={photoInput}
	type="file"
	accept="image/*"
	hidden
	onchange={() => void onPhotoSelected()}
/>

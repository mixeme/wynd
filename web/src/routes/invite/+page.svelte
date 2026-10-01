<script lang="ts">
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import {
		decodeQrFromFile,
		foreignWyndLinkOrigin,
		inviteTarget,
		parseWyndLink
	} from '$lib/auth/links';

	// 2.16: куда вставить присланную ссылку. В установленном приложении ссылка
	// из мессенджера открывается в браузере, а не здесь, — человеку с пустой
	// улочкой нужно место, куда её положить.
	let link = $state('');
	let error = $state('');
	let photoPicker: FilePicker | undefined = $state();

	function open(text: string): boolean {
		const target = inviteTarget(text);
		if ('error' in target) {
			error = target.error;
			return false;
		}
		error = '';
		goto(target.path);
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

	async function onPhotoSelected(file: File) {
		error = '';
		try {
			open(await decodeQrFromFile(file));
		} catch {
			error = 'Не удалось прочитать код — попробуйте другое фото';
		}
	}
</script>

<FormLayout shell app title="Приглашение" onback={() => goto('/circles')}>
	<Hint>Вставьте ссылку, которую вам прислали, или отсканируйте QR-код.</Hint>
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
	<Button variant="ghost" onclick={() => goto('/invite/scan')}>Сканировать QR-код</Button>
	<Button variant="ghost" onclick={() => photoPicker?.open()}>Фото с QR-кодом</Button>
	{#if error}
		<Hint>{error}</Hint>
	{/if}
</FormLayout>

<FilePicker bind:this={photoPicker} accept="image/*" onfiles={([file]) => void onPhotoSelected(file)} />

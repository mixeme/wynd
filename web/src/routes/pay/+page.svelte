<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchCompression, uploadBlob } from '$lib/journal/posts';
	import { compressImage, fileToQueueBuffer, isImageFile } from '$lib/media/compress';
	import { fetchPayStatus, submitPayRequest } from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import type { QueueFile } from '$lib/idb/db';

	let origin = $state('');
	let comment = $state('');
	let file = $state<QueueFile | undefined>();
	let fileLabel = $state('');
	let loading = $state(false);
	let pageLoading = $state(true);
	let redirecting = $state(false);
	let error = $state('');
	let maxBytes = $state(0);
	let photoInput: HTMLInputElement | undefined = $state();

	const canSubmit = $derived(Boolean(file));

	async function onFilesSelected(list: FileList | null) {
		if (!list?.length || !origin) return;
		const picked = list[0];
		if (!isImageFile(picked)) {
			error = 'Нужно изображение';
			return;
		}
		const compression = await fetchCompression(origin);
		let data: QueueFile = isImageFile(picked)
			? await compressImage(picked, compression)
			: await fileToQueueBuffer(picked);
		if (data.size > maxBytes) {
			error = 'Файл слишком большой';
			return;
		}
		file = data;
		fileLabel = `${picked.name} · ${Math.round(data.size / 1024)} КБ`;
		error = '';
		if (photoInput) photoInput.value = '';
	}

	async function submit() {
		if (!file || !origin) return;
		loading = true;
		error = '';
		try {
			const blobId = await uploadBlob(origin, file);
			await submitPayRequest(origin, { blob_id: blobId, comment: comment.trim() });
			goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
			loading = false;
		}
	}

	onMount(async () => {
		try {
			await initSession();
			const sessions = await loadSessions();
			if (!sessions.length) {
				goto('/');
				return;
			}
			origin = sessions[0].origin;
			const status = await fetchPayStatus(origin);
			if (!status.has_requisites || status.pending) {
				redirecting = true;
				goto('/circles');
				return;
			}
			const compression = await fetchCompression(origin);
			maxBytes = compression?.attachment_max_bytes ?? 104_857_600;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			if (!redirecting) pageLoading = false;
		}
	});
</script>

<FormLayout app title="Я оплатил" onback={() => goto('/circles')}>
	{#if pageLoading}
		<Loading />
	{:else}
		<Hint>
			Администратор увидит скриншот и решит, на сколько продлить доступ. Комментарий не обязателен.
		</Hint>
		<Label style="margin-top:18px">Скриншот</Label>
		{#if file}
			<div class="att" style="margin:0 16px">
				<div class="g">
					<div style="font-weight:600">{fileLabel.split(' · ')[0]}</div>
					<div class="sz">{fileLabel.split(' · ')[1] || ''} · как вложение записи</div>
				</div>
			</div>
		{/if}
		<AddPhotoButton onclick={() => photoInput?.click()} />
		<input
			bind:this={photoInput}
			type="file"
			accept="image/*"
			capture="environment"
			hidden
			onchange={(e) => void onFilesSelected(e.currentTarget.files)}
		/>
		<Hint>Фото из галереи или с камеры.</Hint>
		<Label style="margin-top:18px">Комментарий</Label>
		<TextArea variant="field" active bind:value={comment} />
		<Button disabled={!canSubmit || loading} onclick={() => void submit()}>Отправить заявку</Button>
		{#if error}
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}
	{/if}
</FormLayout>

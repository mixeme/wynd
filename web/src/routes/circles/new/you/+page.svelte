<script lang="ts">
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onDestroy, onMount } from 'svelte';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { createCircle } from '$lib/circles/circles';
	import { setCircleColor, setCircleIdentity } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { updateIdentity } from '$lib/circles/settings';
	import {
		NEW_CIRCLE_CTX,
		newCircleEditWindowSec,
		type NewCircleContext
	} from '$lib/circles/new-circle';
	import { createPost, uploadBlob } from '$lib/journal/posts';
	import type { CroppedImage } from '$lib/media/crop';

	// Создатель круга выбирает имя и фото тем же экраном, что и вступающий
	// (1.3): у него в круге такая же идентичность, и спрашивать её надо так же.
	// «Кто уже здесь» нет — в новом круге никого. Круг заводится здесь, уже с
	// выбранным именем: иначе первой строкой журнала было бы переименование.

	const form = getContext<NewCircleContext>(NEW_CIRCLE_CTX);

	let name = $state('');
	let firstPost = $state('');
	let loading = $state(false);
	let error = $state('');
	let cropFile = $state<File | undefined>();
	let pendingAvatar = $state<CroppedImage | undefined>();
	let avatarPreview = $state('');
	let fileInput: HTMLInputElement | undefined = $state();

	const session = $derived(form.sessions.find((s) => s.origin === form.selectedOrigin));

	onMount(() => {
		// Прямой заход или перезагрузка: формы нет — назад к её началу.
		if (!form.name.trim()) goto('/circles/new', { replaceState: true });
	});

	function openPhotoPicker() {
		fileInput?.click();
	}

	function onPhotoSelected(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		if (file.type && !file.type.startsWith('image/')) {
			error = 'Нужно фото';
			return;
		}
		cropFile = file;
	}

	function onCropDone(crop: CroppedImage) {
		cropFile = undefined;
		if (avatarPreview) URL.revokeObjectURL(avatarPreview);
		pendingAvatar = crop;
		avatarPreview = URL.createObjectURL(new Blob([crop.data], { type: crop.type }));
	}

	function today(): string {
		const d = new Date();
		const mm = String(d.getMonth() + 1).padStart(2, '0');
		const dd = String(d.getDate()).padStart(2, '0');
		return `${d.getFullYear()}-${mm}-${dd}`;
	}

	async function create() {
		error = '';
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Введите имя';
			return;
		}
		if (!session) {
			goto('/circles/new');
			return;
		}
		const origin = session.origin;
		loading = true;
		try {
			const created = await createCircle(origin, {
				name: form.name.trim(),
				owner_name: trimmed,
				edit_window_sec: newCircleEditWindowSec(form),
				color: form.color
			});
			await setCircleColor(origin, created.id, form.color);
			await setCircleIdentity(origin, created.id, trimmed);
			rememberCircleOrigin(created.id, origin);
			// Фото и первая запись — не повод терять уже созданный круг: не
			// вышло — круг открывается всё равно, фото ставится в профиле.
			let avatarFailed = false;
			if (pendingAvatar) {
				try {
					const blobId = await uploadBlob(origin, pendingAvatar);
					await updateIdentity(origin, created.id, { avatar_blob_id: blobId });
				} catch {
					avatarFailed = true;
				}
			}
			const body = firstPost.trim();
			if (body) {
				try {
					await createPost(origin, created.id, { body, entry_date: today(), media: [] });
				} catch {
					/* запись можно написать в ленте */
				}
			}
			form.name = '';
			const avatarQuery = avatarFailed ? '?joinAvatar=fail' : '';
			if (form.diaryMode) goto(`/circles/${created.id}${avatarQuery}`, { replaceState: true });
			else goto(`/circles/${created.id}/settings/invite?from=create`, { replaceState: true });
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	onDestroy(() => {
		if (avatarPreview) URL.revokeObjectURL(avatarPreview);
	});
</script>

<FormLayout app color={form.color} circleTitle={form.name} onback={() => goto('/circles/new')}>
	<ScreenTitle class="mt-22 lh-125">
		Как вас зовут<br />в этом круге?
	</ScreenTitle>
	<AddPhotoButton previewUrl={avatarPreview || undefined} onclick={openPhotoPicker} />
	<Hint centered class="mt-8">
		<TextButton onclick={openPhotoPicker}>добавить фото</TextButton>
	</Hint>
	<input bind:this={fileInput} type="file" accept="image/*" hidden onchange={onPhotoSelected} />
	<Label class="mt-18">Имя</Label>
	<Input active type="text" autocomplete="name" bind:value={name} />
	<Label class="label-row">
		<span>{form.diaryMode ? 'Первая запись' : 'Скажи что-нибудь кругу'}</span>
		<span class="label-aside">необязательно</span>
	</Label>
	<TextArea
		variant="area"
		active
		rows={3}
		bind:value={firstPost}
		placeholder={form.diaryMode ? 'С чего начнётся дневник' : 'Первая запись в журнале круга'}
	/>
	<Button variant="colored" {loading} onclick={() => void create()}>
		{form.diaryMode ? 'Завести дневник' : 'Создать и позвать'}
	</Button>
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>

{#if cropFile}
	<AvatarCrop
		file={cropFile}
		color={form.color}
		ondone={onCropDone}
		oncancel={() => (cropFile = undefined)}
	/>
{/if}

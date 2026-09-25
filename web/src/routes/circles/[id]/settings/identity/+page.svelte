<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		fetchCircleSettings,
		fetchIdentityHistory,
		updateIdentity
	} from '$lib/circles/settings';
	import { circleInitial } from '$lib/circles/meta';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { getMediaUrl, revokeMediaUrl, seedMediaUrl } from '$lib/media/objectUrl';
	import { uploadBlob } from '$lib/journal/posts';
	import type { CroppedImage } from '$lib/media/crop';
	import { formatEntryDate } from '$lib/format/time';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let name = $state('');
	let history = $state<Array<{ name: string; effective_at: string }>>([]);
	let avatarUrl = $state('');
	let error = $state('');
	let saving = $state(false);
	let cropFile = $state<File | undefined>();
	let fileInput: HTMLInputElement | undefined = $state();

	async function loadAvatar(blobId?: string) {
		if (blobId) {
			if (blobId !== circle.avatarBlobId) revokeMediaUrl(circle.origin, blobId);
			avatarUrl = await getMediaUrl(circle.origin, blobId);
			circle.avatarBlobId = blobId;
			circle.avatarUrl = avatarUrl;
		} else {
			avatarUrl = '';
			circle.avatarBlobId = '';
			circle.avatarUrl = '';
		}
	}

	async function load() {
		try {
			const [settings, names] = await Promise.all([
				fetchCircleSettings(circle.origin, circle.circleId),
				fetchIdentityHistory(circle.origin, circle.circleId)
			]);
			history = names;
			name = names[0]?.name ?? settings.identity_name ?? circle.identityName;
			await loadAvatar(settings.avatar_blob_id);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function save() {
		const trimmed = name.trim();
		if (!trimmed) return;
		saving = true;
		error = '';
		try {
			await updateIdentity(circle.origin, circle.circleId, { name: trimmed });
			circle.identityName = trimmed;
			circle.identityInitial = circleInitial(trimmed);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			saving = false;
		}
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
		error = '';
		cropFile = file;
	}

	async function onCropDone(crop: CroppedImage) {
		error = '';
		try {
			const blobId = await uploadBlob(circle.origin, crop);
			await updateIdentity(circle.origin, circle.circleId, { avatar_blob_id: blobId });
			avatarUrl = seedMediaUrl(circle.origin, blobId, crop.data, crop.type);
			circle.avatarBlobId = blobId;
			circle.avatarUrl = avatarUrl;
			cropFile = undefined;
		} catch (err) {
			throw new Error(authErrorHint(err));
		}
	}

	function onCropCancel() {
		cropFile = undefined;
	}

	async function clearPhoto() {
		if (!confirm('Убрать фото?')) return;
		try {
			if (circle.avatarBlobId) revokeMediaUrl(circle.origin, circle.avatarBlobId);
			await updateIdentity(circle.origin, circle.circleId, { avatar_blob_id: '' });
			avatarUrl = '';
			circle.avatarBlobId = '';
			circle.avatarUrl = '';
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(() => {
		void load();
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Кто ты в этом круге"
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	<div style="width:96px;height:96px;margin:22px auto 0;display:grid;place-items:center">
		<Avatar
			initial={circle.identityInitial}
			color={circle.colorHex}
			src={avatarUrl}
			style="width:96px;height:96px;font-size:38px"
		/>
	</div>
	<div class="hint ctr" style="margin-top:10px">
		<TextButton onclick={() => fileInput?.click()}>сменить фото</TextButton>
	</div>
	{#if avatarUrl}
		<div class="hint ctr" style="margin-top:6px">
			<TextButton onclick={() => void clearPhoto()}>убрать фото</TextButton>
		</div>
	{/if}
	<input
		bind:this={fileInput}
		type="file"
		accept="image/*"
		hidden
		onchange={(e) => void onPhotoSelected(e)}
	/>
	<Label style="margin-top:20px">Имя</Label>
	<Input active bind:value={name} />
	<Hint
		>Это имя видно только в «{circle.name}». В других кругах вас зовут иначе, и связать одно с
		другим нельзя — даже администратору сервера.</Hint
	>
	{#if history.length > 1}
		<Label style="margin-top:22px">Прежние имена</Label>
		{#each history.slice(1) as row (row.effective_at)}
			<div class="row2" style="padding-top:2px">
				<div class="g">
					<div style="font-weight:600">{row.name}</div>
					<div class="sub">до {formatEntryDate(row.effective_at.slice(0, 10))}</div>
				</div>
			</div>
		{/each}
	{/if}
	<Hint
		>Старые записи остаются подписаны прежним именем — так, как их читали тогда. Смена имени —
		событие хроники.</Hint
	>
	<Button variant="colored" style="margin-top:16px" loading={saving} onclick={() => void save()}>
		Сохранить
	</Button>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>

{#if cropFile}
	<AvatarCrop
		file={cropFile}
		color={circle.color}
		ondone={onCropDone}
		oncancel={onCropCancel}
	/>
{/if}

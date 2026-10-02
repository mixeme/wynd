<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import GenderPicker from '$ui/forms/GenderPicker.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		fetchCircleSettings,
		fetchIdentity,
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
	let photoPicker: FilePicker | undefined = $state();
	let nameHint = $state('');
	let gender = $state<'' | 'm' | 'f'>('');
	let savedGender: '' | 'm' | 'f' = '';

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
			const [settings, identity] = await Promise.all([
				fetchCircleSettings(circle.origin, circle.circleId),
				fetchIdentity(circle.origin, circle.circleId)
			]);
			const names = identity.names;
			history = names;
			gender = savedGender = identity.gender;
			name = names[0]?.name ?? settings.identity_name ?? circle.identityName;
			await loadAvatar(settings.avatar_blob_id);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function save() {
		const trimmed = name.trim();
		if (!trimmed) {
			nameHint = 'Укажите имя';
			return;
		}
		nameHint = '';
		saving = true;
		error = '';
		try {
			await updateIdentity(circle.origin, circle.circleId, {
				name: trimmed,
				...(gender !== savedGender ? { gender } : {})
			});
			circle.identityName = trimmed;
			circle.identityInitial = circleInitial(trimmed);
			goto(`/circles/${circle.circleId}/settings`);
		} catch (err) {
			error = authErrorHint(err);
			saving = false;
		}
	}

	function onPhotoSelected(file: File) {
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
		// Вышедший с доступом имя в круге не меняет: ссылка на экран у него
		// скрыта, а по прямому адресу сохранение ответило бы forbidden (SCR-2).
		if (!circle.canWrite) {
			goto(`/circles/${circle.circleId}`, { replaceState: true });
			return;
		}
		void load();
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Кто вы в этом круге"
	onback={() => goUp(`/circles/${circle.circleId}/settings`)}
>
	<Avatar initial={circle.identityInitial} color={circle.colorHex} src={avatarUrl} size="lg" />
	<Hint centered class="mt-10">
		<TextButton onclick={() => photoPicker?.open()}>сменить фото</TextButton>
	</Hint>
	{#if avatarUrl}
		<Hint class="mt-6" centered>
			<TextButton onclick={() => void clearPhoto()}>убрать фото</TextButton>
		</Hint>
	{/if}
	<FilePicker bind:this={photoPicker} accept="image/*" onfiles={([file]) => onPhotoSelected(file)} />
	<Label class="mt-20">Имя</Label>
	<Input active bind:value={name} />
	{#if nameHint}
		<Hint class="mt-8">{nameHint}</Hint>
	{/if}
	<GenderPicker bind:value={gender} />
	<Hint
		>Это имя видно только в «{circle.name}». В других кругах вас зовут иначе, и связать одно с
		другим нельзя — даже администратору сервера.</Hint
	>
	{#if history.length > 1}
		<Label class="mt-22">Прежние имена</Label>
		{#each history.slice(1) as row (row.effective_at)}
			<SettingsRow class="pt-2"
				title={row.name}
				subtitle="до {formatEntryDate(row.effective_at.slice(0, 10))}"
				chevron={false}
			/>
		{/each}
	{/if}
	<Hint
		>Старые записи остаются подписаны прежним именем — так, как их читали тогда. Смена имени —
		событие хроники.</Hint
	>
	<Button class="mt-16" variant="colored" loading={saving} onclick={() => void save()}>
		Сохранить
	</Button>
	{#if error}
		<Hint class="gutter">{error}</Hint>
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

<script lang="ts">
	import { onDestroy } from 'svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import type { CroppedImage } from '$lib/media/crop';
	import type { CircleColor } from '$lib/theme/colors';

	// «Как вас зовут в этом круге?» (1.3; план 47, 5.8): имя, фото с
	// кадрированием и первая запись. Одинакова у вступающего и у создателя
	// круга — была скопирована. Кнопку и ошибку экран ставит сам: у них
	// разные действия.
	let {
		color,
		name = $bindable(''),
		firstPost = $bindable(''),
		avatar = $bindable(),
		postLabel = 'Скажи что-нибудь кругу',
		postPlaceholder = 'Первая запись в журнале круга',
		onerror
	}: {
		/** Цвет круга: рамка кадрирования. */
		color: CircleColor;
		name?: string;
		firstPost?: string;
		/** Выбранное и откадрированное фото — экран загрузит его после входа. */
		avatar?: CroppedImage;
		postLabel?: string;
		postPlaceholder?: string;
		/** Выбрали не картинку. */
		onerror: (message: string) => void;
	} = $props();

	let cropFile = $state<File | undefined>();
	let preview = $state('');
	let fileInput: HTMLInputElement | undefined = $state();

	function openPicker() {
		fileInput?.click();
	}

	function onSelected(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		if (file.type && !file.type.startsWith('image/')) {
			onerror('Нужно фото');
			return;
		}
		cropFile = file;
	}

	function onCropDone(crop: CroppedImage) {
		cropFile = undefined;
		if (preview) URL.revokeObjectURL(preview);
		avatar = crop;
		preview = URL.createObjectURL(new Blob([crop.data], { type: crop.type }));
	}

	onDestroy(() => {
		if (preview) URL.revokeObjectURL(preview);
	});
</script>

<ScreenTitle class="mt-22 lh-125">
	Как вас зовут<br />в этом круге?
</ScreenTitle>
<AddPhotoButton previewUrl={preview || undefined} onclick={openPicker} />
<Hint centered class="mt-8">
	<TextButton onclick={openPicker}>добавить фото</TextButton>
</Hint>
<input bind:this={fileInput} type="file" accept="image/*" hidden onchange={onSelected} />
<Label>Имя</Label>
<Input active type="text" autocomplete="name" bind:value={name} />
<Label class="label-row">
	<span>{postLabel}</span>
	<span class="label-aside">необязательно</span>
</Label>
<TextArea variant="area" active rows={3} bind:value={firstPost} placeholder={postPlaceholder} />

{#if cropFile}
	<AvatarCrop file={cropFile} {color} ondone={onCropDone} oncancel={() => (cropFile = undefined)} />
{/if}

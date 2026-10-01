<script lang="ts">
	import GenderPicker from '$ui/forms/GenderPicker.svelte';
	import FilePicker from '$ui/forms/FilePicker.svelte';
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
		gender = $bindable(''),
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
		/** Пол для строк журнала (A5): «Аня вступила в круг». */
		gender?: '' | 'm' | 'f';
		postLabel?: string;
		postPlaceholder?: string;
		/** Выбрали не картинку. */
		onerror: (message: string) => void;
	} = $props();

	let cropFile = $state<File | undefined>();
	let preview = $state('');
	let photoPicker: FilePicker | undefined = $state();

	function openPicker() {
		photoPicker?.open();
	}

	function onSelected(file: File) {
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
<FilePicker bind:this={photoPicker} accept="image/*" onfiles={([file]) => onSelected(file)} />
<Label>Имя</Label>
<Input active type="text" autocomplete="name" bind:value={name} />
<GenderPicker bind:value={gender} />
<Label aside="необязательно">{postLabel}</Label>
<TextArea variant="area" active rows={3} bind:value={firstPost} placeholder={postPlaceholder} />

{#if cropFile}
	<AvatarCrop file={cropFile} {color} ondone={onCropDone} oncancel={() => (cropFile = undefined)} />
{/if}

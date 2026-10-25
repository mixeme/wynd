<!-- Smoke-test: compose while a clip compresses, docs/screens.html #e4-2a -->
<script lang="ts">
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import ThumbStrip from '$ui/data/ThumbStrip.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';

	// Снимок готов; ролик сжимается (38 %); звук ждёт очереди — без числа;
	// второй звук сжимается.
	const still = '/icon-192.png';
</script>

<FormLayout
	compose
	color="terracotta"
	title="Семья"
	publishLabel="Опубликовать"
	canPublish={false}
	oncancel={() => {}}
	onpublish={() => {}}
>
	<TextArea variant="compose" placeholder="Что случилось?" rows={1} />
	<ThumbStrip>
		<MediaTile variant="compose" src={still} kind="photo" onclick={() => {}} />
		<MediaTile variant="compose" src={still} kind="video" progress={0.38} onclick={() => {}} />
		<MediaTile variant="compose" fileName="интервью.wav" progress={null} onclick={() => {}} />
		<MediaTile variant="compose" fileName="песня.flac" progress={0.6} onclick={() => {}} />
		<AddPhotoButton onclick={() => {}} />
	</ThumbStrip>
	<Hint>Сжимаем видео (1 из 3)…</Hint>
	<Hint>Не сворачивайте приложение, пока видео сжимается.</Hint>
</FormLayout>

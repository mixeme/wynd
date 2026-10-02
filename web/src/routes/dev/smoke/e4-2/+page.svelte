<!-- Smoke-test: compose from docs/screens.html #e4-2 -->
<script lang="ts">
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import PhotoPlaceholder from '$ui/data/PhotoPlaceholder.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
</script>

<FormLayout
	compose
	color="terracotta"
	title="Семья"
	publishLabel="Опубликовать"
	canPublish={false}
	oncancel={() => {}}
	onpublish={() => {}}
	footer={composeFooter}
>
	<div class="compose-body">
		<TextArea variant="compose" placeholder="Что случилось?" rows={1} />
		<PhotoGrid style="margin-top:12px">
			<PhotoPlaceholder variant="p1" photoCount={2} />
			<AddPhotoButton onclick={() => {}} />
		</PhotoGrid>
	</div>
</FormLayout>

{#snippet composeFooter()}
	<div class="compose-bar">
		<div class="tools">
			<IconButton name="photo" label="Фото или видео" onclick={() => {}} />
			<IconButton name="music" label="Звук" onclick={() => {}} />
			<IconButton name="file" label="Файл" onclick={() => {}} />
			<span class="who">до 32 КБ · как Мышь</span>
		</div>
		<div style="padding:0 18px 12px">
			<TextButton variant="link" onclick={() => {}}>Удалить черновик</TextButton>
		</div>
	</div>
{/snippet}

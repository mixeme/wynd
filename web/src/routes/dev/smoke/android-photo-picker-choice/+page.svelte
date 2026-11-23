<script lang="ts">
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import ComposeToolbar from '$ui/chrome/ComposeToolbar.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';

	let showing = $state(false);
	let choice = $state<'documents' | 'gallery'>('documents');
	let remember = $state(false);

	function openChoice() {
		choice = 'documents';
		remember = false;
		showing = true;
	}
	function closeChoice() {
		showing = false;
	}
</script>

<div class="preview">
	<FormLayout
		app
		compose
		compact
		color="terracotta"
		title="Семья"
		oncancel={() => {}}
		onpublish={() => {}}
		footer={composeFooter}
	>
		<TextArea variant="compose" placeholder="Что случилось?" rows={1} />
		<PhotoGrid style="margin-top:12px">
			<AddPhotoButton onclick={openChoice} />
		</PhotoGrid>
	</FormLayout>
</div>

{#snippet composeFooter()}
	<ComposeToolbar>
		{#snippet tools()}
			<IconButton name="music" label="Звук" onclick={() => {}} />
			<IconButton name="file" label="Файл" onclick={() => {}} />
		{/snippet}
		{#snippet note()}
			до 32 КБ · как Нина
		{/snippet}
	</ComposeToolbar>
	{#if showing}
		{@render pickerChoice()}
	{/if}
{/snippet}

{#snippet pickerChoice()}
	<ConfirmDialog
		title="Как выбирать фото и видео?"
		confirmLabel="Продолжить"
		onconfirm={closeChoice}
		oncancel={closeChoice}
	>
		<Hint>
			В этом круге включены «Места со снимков». Галерея на этом телефоне передаёт фото без
			координат.
		</Hint>
		<ChipGroup class="mt-8">
			<Chip selected={choice === 'documents'} onclick={() => (choice = 'documents')}>Файлы</Chip>
			<Chip selected={choice === 'gallery'} onclick={() => (choice = 'gallery')}>Галерея</Chip>
		</ChipGroup>
		<Hint class="mt-8">
			{choice === 'documents'
				? 'Снимки с координатами можно добавить прямо через «Фото и видео».'
				: 'Чтобы сохранить координаты снимка, добавьте его через «Файл».'}
		</Hint>
		<SettingsRow title="Запомнить выбор" chevron={false}>
			{#snippet control()}
				<Switch bind:checked={remember} label="Запомнить выбор" />
			{/snippet}
		</SettingsRow>
	</ConfirmDialog>
{/snippet}

<style>
	/* Снимает поля галереи /dev и даёт редактору весь экран. */
	.preview {
		position: fixed;
		inset: 0;
	}
</style>

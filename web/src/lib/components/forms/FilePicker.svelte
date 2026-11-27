<script lang="ts" module>
	// Где последний раз коснулись экрана: меню выбора файла на iPhone
	// («Медиатека / Снять / Выбор файлов») встаёт у самого поля, и у поля
	// с `hidden` места нет — меню висело посреди экрана (2026-10-23). Поле
	// ставится точкой под палец, которым нажали кнопку.
	let lastTouch = { x: 0, y: 0 };
	if (typeof window !== 'undefined') {
		window.addEventListener(
			'pointerdown',
			(e) => {
				lastTouch = { x: e.clientX, y: e.clientY };
			},
			{ capture: true, passive: true }
		);
	}
</script>

<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import {
		androidPhotoLocationAvailable,
		applyPhotoLocationChoice,
		originalPhotoAccept,
		photoPickerPreference,
		suppressPhotoLocation,
		type PhotoPickerMode
	} from '$lib/android/photos';

	// Скрытый выбор файлов, который открывает своя кнопка (план 47, 2.17):
	// фото записи, вложения, аватар, скриншот оплаты, фото с QR-кодом. Был в
	// семи местах, у каждого своя возня с bind:this и сбросом поля — а без
	// сброса тот же файл второй раз не выбирался. Экран зовёт open() через
	// bind:this и получает готовый массив.
	let {
		accept,
		multiple = false,
		capture,
		preservePhotoLocation = false,
		offerAndroidPickerChoice = false,
		onerror,
		onfiles
	}: {
		/** «image/*», «image/*,video/*», «*\/*». */
		accept: string;
		multiple?: boolean;
		/** «environment» — сразу камера на телефоне. */
		capture?: HTMLInputAttributes['capture'];
		/** Исходные геоданные нужны только фотографиям записи (план 50, C22). */
		preservePhotoLocation?: boolean;
		/** «Фото и видео» записи. На Android с галереей без геоданных предлагает выбор. */
		offerAndroidPickerChoice?: boolean;
		onerror?: (error: unknown) => void;
		/** Выбрали хотя бы один файл. Поле к этому времени уже сброшено. */
		onfiles: (files: File[]) => void;
	} = $props();

	let input: HTMLInputElement | undefined = $state();
	let arming = false;
	let withoutLocation = false;
	let showing = $state(false);
	let choice = $state<PhotoPickerMode>('documents');
	let remember = $state(false);

	export function open() {
		if (!showing) void selectFiles();
	}

	async function selectFiles(mode?: PhotoPickerMode, saveMode = false) {
		if (!input || arming) return;
		arming = true;
		try {
			const android = !capture && androidPhotoLocationAvailable();
			if (android && offerAndroidPickerChoice && preservePhotoLocation) {
				if (!mode) {
					const preference = await photoPickerPreference();
					if (!input) return;
					if (preservePhotoLocation && preference === 'ask') {
						choice = 'documents';
						remember = false;
						showing = true;
						return;
					}
					mode = preference === 'ask' ? undefined : preference;
				} else if (saveMode) {
					await photoPickerPreference(mode);
				}
			}
			const keepLocation = preservePhotoLocation && mode !== 'gallery';
			const nativeAccept = android && keepLocation ? await originalPhotoAccept(accept) : accept;
			if (!input) return;
			withoutLocation = android && offerAndroidPickerChoice && !keepLocation;
			input.accept = nativeAccept;
			input.style.left = `${lastTouch.x}px`;
			input.style.top = `${lastTouch.y}px`;
			input.click();
		} catch (error) {
			onerror?.(error);
		} finally {
			arming = false;
		}
	}

	function onchange() {
		const files = input?.files ? [...input.files] : [];
		const chosenAccept = input?.accept ?? accept;
		if (withoutLocation) suppressPhotoLocation(files);
		withoutLocation = false;
		if (input) {
			input.value = '';
			input.accept = accept;
		}
		if (files.length && chosenAccept !== accept) {
			void applyPhotoLocationChoice(files, chosenAccept)
				.then(() => onfiles(files))
				.catch((error) => onerror?.(error));
		} else if (files.length) onfiles(files);
	}
</script>

<input
	bind:this={input}
	class="file-picker"
	type="file"
	{accept}
	{multiple}
	{capture}
	tabindex="-1"
	aria-hidden="true"
	{onchange}
	oncancel={() => {
		if (input) input.accept = accept;
		withoutLocation = false;
	}}
/>

{#if showing}
	<ConfirmDialog
		title="Как выбирать фото и видео?"
		confirmLabel="Продолжить"
		onconfirm={() => {
			showing = false;
			void selectFiles(choice, remember);
		}}
		oncancel={() => (showing = false)}
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
{/if}

<style>
	/* Невидимая точка, а не `hidden`: меню iOS нужно место, у которого встать. */
	.file-picker {
		position: fixed;
		left: 0;
		top: 0;
		width: 1px;
		height: 1px;
		opacity: 0;
		pointer-events: none;
	}
</style>

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
	import {
		androidPhotoLocationAvailable,
		originalPhotoAccept,
		applyPhotoLocationChoice
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
		onerror?: (error: unknown) => void;
		/** Выбрали хотя бы один файл. Поле к этому времени уже сброшено. */
		onfiles: (files: File[]) => void;
	} = $props();

	let input: HTMLInputElement | undefined = $state();
	let arming = false;

	export function open() {
		if (!input) return;
		if (arming) return;
		if (preservePhotoLocation && !capture && androidPhotoLocationAvailable()) {
			arming = true;
			void originalPhotoAccept(accept)
				.then((nativeAccept) => {
					if (!input) return;
					input.accept = nativeAccept;
					input.click();
				})
				.catch((error) => onerror?.(error))
				.finally(() => {
					arming = false;
				});
			return;
		}
		input.accept = accept;
		input.style.left = `${lastTouch.x}px`;
		input.style.top = `${lastTouch.y}px`;
		input.click();
	}

	function onchange() {
		const files = input?.files ? [...input.files] : [];
		const chosenAccept = input?.accept ?? accept;
		if (input) {
			input.value = '';
			input.accept = accept;
		}
		if (files.length && preservePhotoLocation) {
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
	}}
/>

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

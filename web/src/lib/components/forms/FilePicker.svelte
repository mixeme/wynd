<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// Скрытый выбор файлов, который открывает своя кнопка (план 47, 2.17):
	// фото записи, вложения, аватар, скриншот оплаты, фото с QR-кодом. Был в
	// семи местах, у каждого своя возня с bind:this и сбросом поля — а без
	// сброса тот же файл второй раз не выбирался. Экран зовёт open() через
	// bind:this и получает готовый массив.
	let {
		accept,
		multiple = false,
		capture,
		onfiles
	}: {
		/** «image/*», «image/*,video/*», «*\/*». */
		accept: string;
		multiple?: boolean;
		/** «environment» — сразу камера на телефоне. */
		capture?: HTMLInputAttributes['capture'];
		/** Выбрали хотя бы один файл. Поле к этому времени уже сброшено. */
		onfiles: (files: File[]) => void;
	} = $props();

	let input: HTMLInputElement | undefined = $state();

	export function open() {
		input?.click();
	}

	function onchange() {
		const files = input?.files ? [...input.files] : [];
		if (input) input.value = '';
		if (files.length) onfiles(files);
	}
</script>

<input bind:this={input} type="file" {accept} {multiple} {capture} hidden {onchange} />

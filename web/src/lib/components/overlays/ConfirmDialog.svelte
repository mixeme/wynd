<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Dialog from '$ui/overlays/Dialog.svelte';
	import Scrim from '$ui/overlays/Scrim.svelte';
	import type { Snippet } from 'svelte';

	// Диалог подтверждения (план 47, 2.2): вопрос, пояснение и две кнопки
	// равной ширины — «отказ» слева, действие справа. Было шесть копий от руки.
	// Фон и «отказ» закрывают одинаково; пока идёт действие, закрыть нельзя.
	let {
		title,
		confirmLabel,
		cancelLabel = 'Отмена',
		loading = false,
		confirmDisabled = false,
		top,
		onconfirm,
		oncancel,
		children
	}: {
		title: string;
		confirmLabel: string;
		cancelLabel?: string;
		loading?: boolean;
		/** Действие не нажимается, пока не выполнено условие — например, не набрано имя (7.16). */
		confirmDisabled?: boolean;
		/** Положение утверждённого диалога, если оно отличается от обычного (7.47). */
		top?: number;
		onconfirm: () => void;
		oncancel: () => void;
		/** Пояснение под вопросом; может нести и поле (удаление круга). */
		children?: Snippet;
	} = $props();

	function cancel() {
		if (!loading) oncancel();
	}
</script>

<Scrim onclick={cancel} />
<Dialog label={title} ondismiss={cancel} style={top === undefined ? '' : `top:${top}px`}>
	<div class="dlgq">{title}</div>
	{@render children?.()}
	<div class="rowin ask">
		<Button variant="ghost" onclick={cancel}>{cancelLabel}</Button>
		<Button {loading} disabled={confirmDisabled} onclick={() => onconfirm()}>{confirmLabel}</Button>
	</div>
</Dialog>

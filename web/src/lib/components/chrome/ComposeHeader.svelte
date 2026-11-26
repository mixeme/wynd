<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';

	/** Шапка редактора записи (4.2, 4.3, 6.13): «Отмена», название, действие.
	   Одна на экран записи и кадрирование — высота и строка не расходятся. */
	let {
		title = '',
		cancelBack = false,
		publishLabel = 'Опубликовать',
		canPublish = false,
		publishing = false,
		compact = true,
		oncancel,
		onpublish
	}: {
		title?: string;
		/** Слева стрелка «Назад» вместо «Отмена» (обложка дня, C44). */
		cancelBack?: boolean;
		publishLabel?: string;
		canPublish?: boolean;
		publishing?: boolean;
		/** Убирает нижний запас шапки: однострочные полосы 48, редкие — 60. */
		compact?: boolean;
		oncancel?: () => void;
		onpublish?: () => void;
	} = $props();
</script>

<div class="cbar">
	<div class="top compose-top">
		{#if cancelBack}
			<IconButton name="back" label="Назад" class="back" onclick={() => oncancel?.()} />
		{:else}
			<TextButton variant="admin" onclick={() => oncancel?.()}>Отмена</TextButton>
		{/if}
		<span class="t">{title}</span>
		<TextButton
			variant="barAction"
			active
			disabled={!canPublish}
			loading={publishing}
			onclick={() => onpublish?.()}
		>
			{publishLabel}
		</TextButton>
	</div>
	{#if !compact}<div style="height:12px"></div>{/if}
</div>

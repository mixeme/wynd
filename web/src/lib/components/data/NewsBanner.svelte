<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';

	// Баннер «Что нового?» над списком кругов (7.7): стоит, пока свежую
	// запись не открыли или не закрыли крестиком. Вид — как у баннера сбора (10.5).
	let {
		version,
		title = 'Что нового?',
		text,
		onclick,
		ondismiss
	}: {
		/** Минорная версия свежей записи: «0.26». Без неё — свой текст. */
		version?: string;
		/** Тот же баннер для новой версии Android, без отдельной компоновки. */
		title?: string;
		text?: string;
		onclick: () => void;
		ondismiss: () => void;
	} = $props();

	const line = $derived(text ?? (version ? `Wynd ${version}: коротко о том, что изменилось` : ''));
</script>

<div class="pay-banner">
	<button type="button" class="pay-banner-main" onclick={() => onclick()}>
		<div class="pay-banner-copy">
			<div class="pay-banner-strong">{title}</div>
			<div class="pay-banner-muted">{line}</div>
		</div>
		<Icon name="chevr" size="sm" class="pay-banner-chev" />
	</button>
	<IconButton name="x" size="sm" label="Скрыть" onclick={() => ondismiss()} />
</div>

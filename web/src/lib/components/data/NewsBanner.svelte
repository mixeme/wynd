<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';

	// Баннер «Что нового?» над списком кругов (7.10): стоит, пока свежую
	// запись не открыли или не закрыли крестиком. Вид — как у баннера сбора (10.5).
	let {
		version,
		onclick,
		ondismiss
	}: {
		/** Минорная версия свежей записи: «0.26». */
		version: string;
		onclick: () => void;
		ondismiss: () => void;
	} = $props();
</script>

<div class="pay-banner">
	<button type="button" class="pay-banner-main" onclick={() => onclick()}>
		<div class="pay-banner-copy">
			<div class="pay-banner-strong">Что нового?</div>
			<div class="pay-banner-muted">Wynd {version}: коротко о том, что изменилось</div>
		</div>
		<Icon name="chevr" size="sm" class="pay-banner-chev" />
	</button>
	<IconButton name="x" size="sm" label="Скрыть" onclick={() => ondismiss()} />
</div>

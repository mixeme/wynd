<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';

	// Объявление над списком кругов без перехода — только прочитать и закрыть
	// (7.7а, подсказка про браузер). Вид — как у «Что нового?» (7.7).
	let {
		title,
		text,
		ondismiss
	}: {
		title: string;
		text: string;
		ondismiss: () => void;
	} = $props();
</script>

<div class="pay-banner">
	<div class="pay-banner-main">
		<div class="pay-banner-copy">
			<div class="pay-banner-strong">{title}</div>
			<div class="pay-banner-muted">{text}</div>
		</div>
	</div>
	<IconButton name="x" size="sm" label="Скрыть" onclick={() => ondismiss()} />
</div>

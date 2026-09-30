<!-- Smoke-test: overlay components from docs/screens.html #e4-14 -->
<script lang="ts">
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
</script>

<PhoneFrame>
	<Lightbox
		counter="3 из 12"
		caption="Аня · сегодня, 14:02 · Дача · 2048 × 1536"
		onclose={() => {}}
		ondownload={() => {}}
	>
		{#snippet media()}
			<div class="pic p2" style="width:100%;height:292px;border-radius:0;border:0"></div>
		{/snippet}
		{#snippet dots()}
			<u></u><u></u><u class="on"></u><u></u><u></u><u></u><u></u><u></u><u></u><u></u><u></u><u></u>
		{/snippet}
	</Lightbox>
</PhoneFrame>

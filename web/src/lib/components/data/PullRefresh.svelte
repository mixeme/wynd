<script lang="ts">
	import Mark from '$ui/Mark.svelte';
	import {
		pullHeight,
		pullMarkHeight,
		pullVisible,
		type PullState
	} from '$lib/gestures/pullToRefresh';

	// Полоса «потянуть, чтобы обновить» (3.5; план 47, 2.8): знак
	// дорисовывается вместе с жестом и держится, пока идёт обновление.
	// Жест ведёт контроллер `PullRefresh` из `$lib/gestures/pullRefresh.svelte`.
	let { pull }: { pull: PullState } = $props();
</script>

{#if pullVisible(pull)}
	<div class="ptr" style:height="{pullHeight(pull)}px" aria-hidden="true">
		<div class="ptr-mark" style:height="{pullMarkHeight(pull)}px">
			<Mark />
		</div>
	</div>
{/if}

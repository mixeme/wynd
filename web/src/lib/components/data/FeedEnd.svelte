<script lang="ts">
	import Mark from '$ui/Mark.svelte';
	import Hint from '$ui/forms/Hint.svelte';

	// Низ ленты круга (3.1, 3.6; план 47, 3.6): знак и откуда лента видна.
	// since задан — человек видит круг не с начала («Вы здесь с…», знак в
	// цвете круга); иначе лента дошла до начала круга (знак приглушён).
	let {
		since,
		started
	}: {
		/** С какого дня круг виден этому человеку: «12 сентября». */
		since?: string;
		/** Когда круг начался; при since — с годом. */
		started?: string;
	} = $props();
</script>

{#if since}
	<div class="feed-end cutoff">
		<Mark />
		<div class="feed-end-since">Вы здесь с {since}</div>
		<Hint centered class="feed-end-note">что было раньше — не ваше</Hint>
		{#if started}
			<div class="sep"></div>
			<Hint centered>круг живёт с {started}</Hint>
		{/if}
	</div>
{:else if started}
	<div class="feed-end start">
		<Mark />
		<Hint centered class="feed-end-title">Здесь начинается круг</Hint>
		<Hint centered class="mt-4 muted">{started}</Hint>
	</div>
{/if}

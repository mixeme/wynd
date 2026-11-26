<script lang="ts">
	import EmptyState from '$ui/data/EmptyState.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import type { Snippet } from 'svelte';

	// План 75: широкое окно — список кругов слева, открытый круг справа.
	// Каркас общий: колонки прокручиваются сами, ширина и сжатый вид списка
	// заданы в ui.css по тем же 720/1000 px. Телефон сюда не попадает:
	// вызывается только когда ширина позволяет.
	let {
		sidebar,
		empty = false,
		children
	}: {
		sidebar: Snippet;
		/** Круг не выбран: справа пусто, первый сам не открывается. */
		empty?: boolean;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame app class="split">
	<aside class="split-side">{@render sidebar()}</aside>
	<div class="split-main">
		{#if empty}
			<div class="split-empty">
				<EmptyState place="tab" title="Выберите круг">
					Откройте круг в списке слева — он появится здесь.
				</EmptyState>
			</div>
		{:else}
			{@render children()}
		{/if}
	</div>
</PhoneFrame>

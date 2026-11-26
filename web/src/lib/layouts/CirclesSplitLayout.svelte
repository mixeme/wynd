<script lang="ts">
	import { goto } from '$app/navigation';
	import EmptyState from '$ui/data/EmptyState.svelte';
	import Button from '$ui/forms/Button.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import type { Snippet } from 'svelte';

	// План 75: широкое окно — список кругов слева, открытый круг справа.
	// Каркас общий: колонки прокручиваются сами, ширина и сжатый вид списка
	// заданы в ui.css по тем же 720/1000 px. Телефон сюда не попадает:
	// вызывается только когда ширина позволяет.
	let {
		sidebar,
		empty = false,
		rail = false,
		children
	}: {
		sidebar: Snippet;
		/** Круг не выбран: справа пусто, первый сам не открывается. */
		empty?: boolean;
		/** Промежуточная ширина: слева только аватары, 80 px. */
		rail?: boolean;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame app class={rail ? 'split rail' : 'split'}>
	<aside class="split-side">{@render sidebar()}</aside>
	<div class="split-main">
		{#if empty}
			<div class="split-empty">
				<EmptyState place="tab" title="Выберите круг">
					Откройте круг в списке слева — он появится здесь.
					{#snippet actions()}
						<Button variant="ghost" onclick={() => goto('/search')}>Поиск по кругам</Button>
					{/snippet}
				</EmptyState>
			</div>
		{:else}
			{@render children()}
		{/if}
	</div>
</PhoneFrame>

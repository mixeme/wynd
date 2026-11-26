<script lang="ts">
	import EmptyState from '$ui/data/EmptyState.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import type { Snippet } from 'svelte';

	// План 76: широкое окно — у раздела своя левая колонка, справа выбранное.
	// Каркас общий: колонки прокручиваются сами, ширина задана в ui.css по тому
	// же порогу 1060 px. Телефон сюда не попадает: вызывается только когда
	// ширина позволяет.
	let {
		sidebar,
		empty = false,
		emptyTitle = 'Выберите круг',
		emptyText = 'Откройте круг в списке слева — он появится здесь.',
		children
	}: {
		sidebar: Snippet;
		/** Ничего не выбрано: справа пусто, первый сам не открывается. */
		empty?: boolean;
		emptyTitle?: string;
		emptyText?: string;
		children: Snippet;
	} = $props();
</script>

<PhoneFrame app class="split">
	<aside class="split-side">{@render sidebar()}</aside>
	<div class="split-main">
		{#if empty}
			<div class="split-empty">
				<EmptyState place="tab" title={emptyTitle}>{emptyText}</EmptyState>
			</div>
		{:else}
			{@render children()}
		{/if}
	</div>
</PhoneFrame>

<script lang="ts">
	import ReactionListRow from '$ui/data/ReactionListRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Scrim from '$ui/overlays/Scrim.svelte';
	import Sheet from '$ui/overlays/Sheet.svelte';
	import { authorInitial, reactionIconName } from '$lib/journal/present';
	import type { Reaction } from '$lib/journal/types';

	// Лист «Кто отреагировал» (4.8; план 47, 2.4): одинаков в ленте и на
	// экране записи — был скопирован дословно.
	let {
		reactions,
		color,
		ondismiss
	}: {
		reactions: Reaction[];
		/** Цвет круга: аватары в строках. */
		color: string;
		ondismiss: () => void;
	} = $props();
</script>

<Scrim onclick={ondismiss} />
<Sheet label="Реакции" {ondismiss}>
	<SectionLabel style="margin-top:2px">Реакция · {reactions.length}</SectionLabel>
	{#each reactions as rx (rx.id)}
		<ReactionListRow
			initial={authorInitial(rx.author_name)}
			name={rx.author_name}
			{color}
			icon={reactionIconName(rx.emoji)}
		/>
	{/each}
	<Hint class="mt-14">
		Реакция одна на человека и подчиняется окну правок. Хотите сказать больше — напишите словами.
	</Hint>
</Sheet>

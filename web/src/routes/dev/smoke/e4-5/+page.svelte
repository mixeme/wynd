<!-- Smoke-test: OverlayLayout from docs/screens.html #e4-5 -->
<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionListRow from '$ui/data/ReactionListRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
</script>

<CircleLayout title="Семья" identity="Мышь" avatar="М" commentBar={false}>
	<PostCard>
		{#snippet author()}
			<Avatar initial="А" color="#58673A" />
			<div>
				<div class="n">Аня</div>
				<div class="tm">сегодня, 14:02</div>
			</div>
		{/snippet}
		{#snippet text()}
			Были на даче, все живы. Яблони в этом году как с ума сошли.
		{/snippet}
		{#snippet media()}
			<div class="pic sq p1" style="aspect-ratio:4/3">
				<span class="tagr"><Icon name="loc" size="xs" />Дача</span>
				<span class="cnt">12</span>
			</div>
		{/snippet}
	</PostCard>
	<OverlayLayout>
		<SectionLabel style="margin-top:2px">Реакция · 2</SectionLabel>
		<ReactionListRow initial="К" name="Кот" color="#62452F" icon="heart" />
		<ReactionListRow initial="П" name="Петя" color="#357077" icon="heart" />
		<Hint style="margin-top:14px">
			Реакция одна на человека и подчиняется окну правок. Хотите сказать больше — напишите словами.
		</Hint>
	</OverlayLayout>
</CircleLayout>

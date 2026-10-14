<!-- Smoke-test: CircleLayout from docs/screens.html #e3-19 (?server — #e3-20, ?owner — у владельца) -->
<script lang="ts">
	import { page } from '$app/state';
	import Avatar from '$ui/data/Avatar.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import Icon from '$ui/Icon.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { spaceNotice } from '$lib/circles/space';

	const notice = $derived(
		spaceNotice({
			spaceFull: page.url.searchParams.has('server') ? 'server' : 'circle',
			ownerName: 'Кот',
			youOwn: page.url.searchParams.has('owner'),
			serverName: 'Дома Ани',
			circleId: 'smoke'
		})
	);
</script>

{#snippet note()}
	{#if notice}
		{notice.first}<br />
		{#if notice.href}
			<a class="under" href={notice.href}>{notice.second}</a>
		{:else}
			{notice.second}
		{/if}
	{/if}
{/snippet}

<CircleLayout title="Семья" identity="Мышь" avatar="М" commentBar={false} notice={note}>
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
			<div class="pic sq p1">
				<span class="tagr"><Icon name="loc" size="xs" />Дача</span>
				<span class="cnt">12</span>
			</div>
		{/snippet}
		{#snippet reactions()}
			<!-- Реакции ставить нельзя: «+» нет, поставленные видны. -->
			<ReactionBar
				groups={[{ icon: 'heart', names: 'Кот, Петя' }]}
				keys={['heart', 'laugh', 'surprise', 'anger']}
				onopenList={() => {}}
				onadd={() => {}}
				onpick={() => {}}
			/>
		{/snippet}
	</PostCard>
	<CommentPreview onclick={() => {}}>
		<div>Петя: а компот будет?</div>
		<div class="mo">ещё 11 комментариев</div>
	</CommentPreview>
</CircleLayout>

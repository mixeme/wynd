<!-- Smoke-test: CircleLayout from docs/screens.html #e3-1 -->
<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import Icon from '$ui/Icon.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
</script>

<CircleLayout
	title="Семья"
	identity="Мышь"
	avatar="М"
	onCommentSend={() => {}}
	onCommentCompose={() => {}}
	onCommentPhotos={() => {}}
>
	<EventDivider text="Мышка теперь Мышь" />
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
			<ReactionBar
				groups={[{ icon: 'heart', names: 'Кот, Петя' }]}
				keys={['heart', 'laugh', 'surprise', 'anger']}
				showAdd
				onopenList={() => {}}
				onadd={() => {}}
				onpick={() => {}}
			/>
		{/snippet}
	</PostCard>
	<CommentPreview onclick={() => {}}>
		<div>Петя: а компот будет? <span class="tm">сегодня, 14:22</span></div>
		<div class="mo">ещё 11 комментариев</div>
	</CommentPreview>
	<PostCard>
		{#snippet author()}
			<Avatar initial="М" color="#3C4D83" />
			<div>
				<div class="n">Мама</div>
				<div class="tm">вчера, 21:40</div>
			</div>
		{/snippet}
		{#snippet text()}
			Дед нашёл на чердаке коробку с плёнками. Будем проявлять.
		{/snippet}
	</PostCard>
</CircleLayout>

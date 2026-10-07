<!-- Smoke-test: открытка дня from docs/screens.html #e3-15, #e3-17 -->
<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import FeedDayCard from '$ui/data/FeedDayCard.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
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
	<PostCard>
		{#snippet author()}
			<Avatar initial="А" color="#58673A" />
			<div>
				<div class="n">Аня</div>
				<div class="tm">сегодня, 14:02</div>
			</div>
		{/snippet}
		{#snippet text()}
			Были на даче, все живы.
		{/snippet}
	</PostCard>
	<FeedDayCard
		date="6 августа"
		title="Плёнки"
		hasCover
		caption="Мама назвала день и выбрала обложку · вчера, 21:44"
		onclick={() => {}}
	/>
	<FeedDayCard date="2 августа" title="Река" caption="Петя назвал день · вчера, 18:10" onclick={() => {}} />
	<FeedDayCard
		date="30 июля"
		hasCover
		video
		caption="Кот выбрал обложку · вчера, 17:02"
		onclick={() => {}}
	/>
	<EventDivider text="Аня убрала название для 30 июля" />
</CircleLayout>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatPostTime } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { searchCircle } from '$lib/journal/search';
	import type { CircleSearchHit } from '$lib/journal/types';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let query = $state('');
	let debounced = $state('');
	let authorFilter = $state('');
	let loading = $state(false);
	let hits = $state<CircleSearchHit[]>([]);
	let timer: ReturnType<typeof setTimeout> | undefined;

	const authors = $derived(
		[...new Set(hits.map((h) => h.author_name).filter(Boolean) as string[])].sort((a, b) =>
			a.localeCompare(b, 'ru')
		)
	);

	const filtered = $derived(
		authorFilter ? hits.filter((h) => h.author_name === authorFilter) : hits
	);

	const stats = $derived.by(() => {
		const posts = filtered.filter((h) => h.kind === 'post').length;
		const comments = filtered.filter((h) => h.kind === 'comment').length;
		const parts: string[] = [];
		if (posts) parts.push(`${posts} ${posts === 1 ? 'запись' : posts < 5 ? 'записи' : 'записей'}`);
		if (comments)
			parts.push(
				`${comments} ${comments === 1 ? 'комментарий' : comments < 5 ? 'комментария' : 'комментариев'}`
			);
		return parts.join(' и ');
	});

	$effect(() => {
		// Read `query` synchronously: inside the timeout it is not a dependency
		// and the effect would never re-run as the user types.
		const next = query.trim();
		clearTimeout(timer);
		timer = setTimeout(() => {
			debounced = next;
		}, 300);
		return () => clearTimeout(timer);
	});

	$effect(() => {
		if (!debounced) {
			hits = [];
			authorFilter = '';
			return;
		}
		void runSearch(debounced);
	});

	async function runSearch(q: string) {
		loading = true;
		try {
			hits = await searchCircle(circle.origin, circle.circleId, q);
			authorFilter = '';
		} finally {
			loading = false;
		}
	}

	function highlight(text: string, term: string) {
		if (!term) return text;
		const idx = text.toLowerCase().indexOf(term.toLowerCase());
		if (idx < 0) return text;
		return `${text.slice(0, idx)}«${text.slice(idx, idx + term.length)}»${text.slice(idx + term.length)}`;
	}

	function goBack() {
		goto(`/circles/${circle.circleId}`);
	}

	function openHit(hit: CircleSearchHit) {
		goto(`/circles/${circle.circleId}/posts/${hit.post_id}`);
	}

	function toggleAuthor(name: string) {
		authorFilter = authorFilter === name ? '' : name;
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={circle.name}
	tabs={false}
	commentBar={false}
	onback={goBack}
>
	<SearchField
		style="margin:12px 16px 0"
		placeholder="Искать в круге"
		bind:value={query}
	/>
	{#if authors.length}
		<ChipGroup style="margin-top:14px">
			{#each authors as name (name)}
				<Chip selected={authorFilter === name} onclick={() => toggleAuthor(name)}>{name}</Chip>
			{/each}
		</ChipGroup>
	{/if}
	{#if loading}
		<Hint style="margin-top:16px">Ищем…</Hint>
	{:else if debounced && !filtered.length}
		<Hint style="margin-top:16px">Ничего не найдено</Hint>
	{:else if stats}
		<Hint>{stats}</Hint>
	{/if}
	{#each filtered as hit, i (`${hit.post_id}-${hit.comment_id ?? i}`)}
		<SearchResultRow
			author={hit.author_name ?? '—'}
			time={formatPostTime(hit.created_at, hit.entry_date)}
			onclick={() => openHit(hit)}
		>
			{#snippet preview()}
				{highlight(hit.snippet, debounced)}
				{#if hit.kind === 'comment'}
					<span style="color:var(--faint)"> · комментарий</span>
				{/if}
			{/snippet}
		</SearchResultRow>
	{/each}
	<Hint centered style="margin-top:22px">
		Фильтр по автору работает только здесь:<br />в других кругах это другие люди.
	</Hint>
</CircleLayout>

<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { circleNameMap, searchAllOrigins } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { formatPostTime } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';

	let query = $state('');
	let debounced = $state('');
	let loading = $state(false);
	let groups = $state<
		Array<{
			circleId: string;
			name: string;
			color: string;
			hits: Array<{
				postId: string;
				author: string;
				time: string;
				snippet: string;
			}>;
		}>
	>([]);
	let timer: ReturnType<typeof setTimeout> | undefined;

	onMount(async () => {
		const sessions = await loadSessions();
		if (!sessions.length) goto('/');
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
			groups = [];
			return;
		}
		void runSearch(debounced);
	});

	async function runSearch(q: string) {
		loading = true;
		try {
			const [results, names] = await Promise.all([searchAllOrigins(q), circleNameMap()]);
			const byCircle = new Map<
				string,
				{ name: string; color: string; hits: Array<{ postId: string; author: string; time: string; snippet: string }> }
			>();
			for (const { origin, hits } of results) {
				for (const hit of hits) {
					const key = `${origin}:${hit.circle_id}`;
					const meta = names.get(key);
					const entry = byCircle.get(key) ?? {
						name: meta?.name ?? 'Круг',
						color: meta?.color ?? 'var(--slate)',
						hits: []
					};
					entry.hits.push({
						postId: hit.post_id,
						author: hit.author_name ?? '—',
						time: formatPostTime(hit.created_at ?? '', hit.entry_date),
						snippet: hit.snippet
					});
					byCircle.set(key, entry);
				}
			}
			groups = [...byCircle.entries()].map(([circleId, value]) => ({
				circleId,
				...value
			}));
		} finally {
			loading = false;
		}
	}

	function openHit(group: (typeof groups)[number], hit: (typeof groups)[number]['hits'][number]) {
		const [origin, circleId] = group.circleId.split(':');
		if (!origin || !circleId) return;
		rememberCircleOrigin(circleId, origin);
		goto(`/circles/${circleId}/posts/${hit.postId}`);
	}

	function highlight(text: string, term: string) {
		if (!term) return text;
		const idx = text.toLowerCase().indexOf(term.toLowerCase());
		if (idx < 0) return text;
		return `${text.slice(0, idx)}«${text.slice(idx, idx + term.length)}»${text.slice(idx + term.length)}`;
	}
</script>

<FormLayout shell app title="Поиск" search={query || undefined} onback={() => goto('/circles')}>
	<SearchField
		style="margin:12px 16px 0"
		placeholder="Искать по всем кругам"
		bind:value={query}
	/>
	{#if loading}
		<Hint style="margin-top:16px">Ищем…</Hint>
	{:else if debounced && !groups.length}
		<Hint style="margin-top:16px">Ничего не найдено</Hint>
	{:else}
		{#each groups as group (group.circleId)}
			<SearchGroupHeader color={group.color} name={group.name} count={group.hits.length} />
			{#each group.hits as hit, i (`${group.circleId}-${i}`)}
				<SearchResultRow
					author={hit.author}
					time={hit.time}
					onclick={() => openHit(group, hit)}
				>
					{#snippet preview()}
						{highlight(hit.snippet, debounced)}
					{/snippet}
				</SearchResultRow>
			{/each}
		{/each}
	{/if}
	<Hint centered style="margin-top:22px">
		Ищется только то, что вы застали:<br />отрезки видимости работают и в поиске.
	</Hint>
</FormLayout>

<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { circleNameMap, searchAllOrigins } from '$lib/circles/circles';
	import { peekLastCircle, rememberCircleOrigin } from '$lib/circles/origin';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';
	import { searchChipsFromParams, searchHref } from '$lib/journal/search';

	const initialChips = searchChipsFromParams($page.url.searchParams);
	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let filterPhoto = $state(initialChips.hasPhoto);
	let filterLocation = $state(initialChips.hasLocation);
	let periodActive = $state(Boolean(initialChips.from || initialChips.to));
	let periodFrom = $state(initialChips.from);
	let periodTo = $state(initialChips.to);
	let lastCircleId = $state<string | null>(null);
	let loading = $state(false);
	let groups = $state<
		Array<{
			circleId: string;
			name: string;
			color: string;
			hits: Array<{
				postId: string;
				kind: string;
				entryDate: string;
				title?: string;
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
		lastCircleId = peekLastCircle();
	});

	$effect(() => {
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
		const from = periodActive && periodFrom ? periodFrom : undefined;
		const to = periodActive && periodTo ? periodTo : undefined;
		const photo = filterPhoto;
		const location = filterLocation;
		void (async () => {
			loading = true;
			try {
				const [results, names] = await Promise.all([
					searchAllOrigins(debounced, {
						from,
						to,
						hasPhoto: photo,
						hasLocation: location
					}),
					circleNameMap()
				]);
				const byCircle = new Map<
					string,
					{
						name: string;
						color: string;
						hits: Array<{
							postId: string;
							kind: string;
							entryDate: string;
							title?: string;
							author: string;
							time: string;
							snippet: string;
						}>;
					}
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
						const isDay = hit.kind === 'day';
						entry.hits.push({
							postId: hit.post_id,
							kind: hit.kind,
							entryDate: hit.entry_date,
							title: hit.title,
							author: isDay ? (hit.title ?? hit.snippet) : (hit.author_name ?? '—'),
							time: isDay
								? formatEntryDate(hit.entry_date)
								: formatPostTime(hit.created_at ?? '', hit.entry_date),
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
		})();
	});

	function splitCircleKey(key: string): { origin: string; circleId: string } {
		const sep = key.lastIndexOf(':');
		if (sep < 0) return { origin: '', circleId: key };
		return { origin: key.slice(0, sep), circleId: key.slice(sep + 1) };
	}

	function openHit(group: (typeof groups)[number], hit: (typeof groups)[number]['hits'][number]) {
		const { origin, circleId } = splitCircleKey(group.circleId);
		if (!circleId) return;
		rememberCircleOrigin(circleId, origin);
		if (hit.kind === 'day') {
			goto(`/circles/${circleId}/days/${hit.entryDate}`);
			return;
		}
		goto(`/circles/${circleId}/posts/${hit.postId}`);
	}

	function openCircleSearch() {
		if (!lastCircleId) return;
		goto(
			searchHref(`/circles/${lastCircleId}/search`, {
				q: debounced || query.trim(),
				periodActive,
				periodFrom,
				periodTo,
				hasPhoto: filterPhoto,
				hasLocation: filterLocation
			})
		);
	}

	function rowAuthor(hit: (typeof groups)[number]['hits'][number]) {
		if (hit.kind === 'day') return highlight(hit.title ?? hit.snippet, debounced);
		return hit.author;
	}

	function highlight(text: string, term: string) {
		if (!term) return text;
		const idx = text.toLowerCase().indexOf(term.toLowerCase());
		if (idx < 0) return text;
		return `${text.slice(0, idx)}«${text.slice(idx, idx + term.length)}»${text.slice(idx + term.length)}`;
	}

	function togglePeriod() {
		periodActive = !periodActive;
		if (!periodActive) {
			periodFrom = '';
			periodTo = '';
		}
	}
</script>

<FormLayout shell app title="Поиск" search={query || undefined} onback={() => goto('/circles')}>
	<SearchField
		style="margin:12px 16px 0"
		placeholder="Искать по всем кругам"
		bind:value={query}
	/>
	<ChipGroup style="margin-top:12px">
		<Chip disabled={!lastCircleId} onclick={openCircleSearch}>Этот круг</Chip>
		<Chip selected>Все круги</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip selected={periodActive} onclick={togglePeriod}>Период</Chip>
		<Chip selected={filterPhoto} onclick={() => (filterPhoto = !filterPhoto)}>С фото</Chip>
		<Chip selected={filterLocation} onclick={() => (filterLocation = !filterLocation)}
			>С местом</Chip
		>
	</ChipGroup>
	{#if periodActive}
		<div style="display:flex;gap:8px;margin:8px 16px 0">
			<Input type="date" bind:value={periodFrom} active style="flex:1" />
			<Input type="date" bind:value={periodTo} active style="flex:1" />
		</div>
	{/if}
	{#if loading}
		<Hint style="margin-top:16px">Ищем…</Hint>
	{:else if debounced && !groups.length}
		<Hint style="margin-top:16px">Ничего не найдено</Hint>
	{:else}
		{#each groups as group (group.circleId)}
			<SearchGroupHeader color={group.color} name={group.name} count={group.hits.length} />
			{#each group.hits as hit, i (`${group.circleId}-${i}`)}
				<SearchResultRow
					author={rowAuthor(hit)}
					time={hit.time}
					onclick={() => openHit(group, hit)}
				>
					{#snippet preview()}
						{#if hit.kind === 'day'}
							<span style="color:var(--faint)">день</span>
						{:else}
							{highlight(hit.snippet, debounced)}
							{#if hit.kind === 'comment'}
								<span style="color:var(--faint)"> · комментарий</span>
							{/if}
						{/if}
					{/snippet}
				</SearchResultRow>
			{/each}
		{/each}
	{/if}
	<Hint centered style="margin-top:22px">
		Ищется только то, что вы застали:<br />отрезки видимости работают и в поиске.
	</Hint>
</FormLayout>

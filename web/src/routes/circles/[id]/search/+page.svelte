<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { searchChipsFromParams, searchCircle, searchCircleAuthors, searchHref } from '$lib/journal/search';
	import type { CircleSearchHit } from '$lib/journal/types';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const initialChips = searchChipsFromParams($page.url.searchParams);
	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let authorFilter = $state('');
	let filterPhoto = $state(initialChips.hasPhoto);
	let filterLocation = $state(initialChips.hasLocation);
	let periodActive = $state(Boolean(initialChips.from || initialChips.to));
	let periodFrom = $state(initialChips.from);
	let periodTo = $state(initialChips.to);
	let loading = $state(false);
	let hits = $state<CircleSearchHit[]>([]);
	let authorNames = $state<string[]>([]);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let searchGen = 0;

	const stats = $derived.by(() => {
		const posts = hits.filter((h) => h.kind === 'post').length;
		const comments = hits.filter((h) => h.kind === 'comment').length;
		const days = hits.filter((h) => h.kind === 'day').length;
		const parts: string[] = [];
		if (posts) parts.push(`${posts} ${posts === 1 ? 'запись' : posts < 5 ? 'записи' : 'записей'}`);
		if (comments)
			parts.push(
				`${comments} ${comments === 1 ? 'комментарий' : comments < 5 ? 'комментария' : 'комментариев'}`
			);
		if (days) {
			const mod10 = days % 10;
			const mod100 = days % 100;
			if (days === 1) parts.push('день');
			else if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) parts.push(`${days} дня`);
			else parts.push(`${days} дней`);
		}
		return parts.join(', ').replace(/, ([^,]+)$/, ' и $1');
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
			hits = [];
			authorFilter = '';
			authorNames = [];
			return;
		}
		const from = periodActive && periodFrom ? periodFrom : undefined;
		const to = periodActive && periodTo ? periodTo : undefined;
		const photo = filterPhoto;
		const location = filterLocation;
		const author = authorFilter || undefined;
		const gen = ++searchGen;
		void (async () => {
			loading = true;
			try {
				const chipFilters = { from, to, hasPhoto: photo, hasLocation: location };
				const [next, authors] = await Promise.all([
					searchCircle(circle.origin, circle.circleId, debounced, 50, {
						...chipFilters,
						author
					}),
					searchCircleAuthors(circle.origin, circle.circleId, debounced, chipFilters)
				]);
				if (gen !== searchGen) return;
				hits = next;
				authorNames = authors;
			} finally {
				if (gen === searchGen) loading = false;
			}
		})();
	});

	function highlight(text: string, term: string) {
		if (!term) return text;
		const idx = text.toLowerCase().indexOf(term.toLowerCase());
		if (idx < 0) return text;
		return `${text.slice(0, idx)}«${text.slice(idx, idx + term.length)}»${text.slice(idx + term.length)}`;
	}

	function goBack() {
		goto(`/circles/${circle.circleId}`);
	}

	function openGlobalSearch() {
		goto(
			searchHref('/search', {
				q: debounced || query.trim(),
				periodActive,
				periodFrom,
				periodTo,
				hasPhoto: filterPhoto,
				hasLocation: filterLocation
			})
		);
	}

	function openHit(hit: CircleSearchHit) {
		if (hit.kind === 'day') {
			goto(`/circles/${circle.circleId}/days/${hit.entry_date}`);
			return;
		}
		goto(`/circles/${circle.circleId}/posts/${hit.post_id}`);
	}

	function rowAuthor(hit: CircleSearchHit) {
		if (hit.kind === 'day') return highlight(hit.title ?? hit.snippet, debounced);
		return hit.author_name ?? '—';
	}

	function rowTime(hit: CircleSearchHit) {
		if (hit.kind === 'day') return formatEntryDate(hit.entry_date);
		return formatPostTime(hit.created_at, hit.entry_date);
	}

	function toggleAuthor(name: string) {
		authorFilter = authorFilter === name ? '' : name;
	}

	function togglePeriod() {
		periodActive = !periodActive;
		if (!periodActive) {
			periodFrom = '';
			periodTo = '';
		}
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
	<ChipGroup style="margin-top:12px">
		<Chip selected>Этот круг</Chip>
		<Chip onclick={openGlobalSearch}>Все круги</Chip>
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
			<Input type="date" bind:value={periodFrom} active placeholder="с" style="flex:1" />
			<Input type="date" bind:value={periodTo} active placeholder="по" style="flex:1" />
		</div>
	{/if}
	{#if authorNames.length}
		<ChipGroup style="margin-top:14px">
			{#each authorNames as name (name)}
				<Chip selected={authorFilter === name} onclick={() => toggleAuthor(name)}>{name}</Chip>
			{/each}
		</ChipGroup>
	{/if}
	{#if loading}
		<Hint style="margin-top:16px">Ищем…</Hint>
	{:else if debounced && !hits.length}
		<Hint style="margin-top:16px">Ничего не найдено</Hint>
	{:else if stats}
		<Hint>{stats}</Hint>
	{/if}
	{#each hits as hit, i (`${hit.kind}-${hit.post_id}-${hit.comment_id ?? hit.entry_date}-${i}`)}
		<SearchResultRow author={rowAuthor(hit)} time={rowTime(hit)} onclick={() => openHit(hit)}>
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
	<Hint centered style="margin-top:22px">
		Фильтр по автору работает только здесь:<br />в других кругах это другие люди.
	</Hint>
</CircleLayout>

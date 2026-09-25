<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import {
		searchChipsFromParams,
		searchCircle,
		searchCircleAuthors,
		searchHref,
		replaceSearchUrl,
		searchChipsFromWindow,
		searchThumbVariant
	} from '$lib/journal/search';
	import { searchStats, toggleAuthor, togglePeriod } from '$lib/journal/searchState';
	import type { CircleSearchHit } from '$lib/journal/types';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const initialChips = searchChipsFromParams($page.url.searchParams);
	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let authorFilter = $state(initialChips.author);
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
	let thumbUrls = $state<Record<string, string>>({});
	let sourceError = $state('');

	const searchPath = $derived($page.url.pathname);

	function applyUrlChips() {
		const chips = searchChipsFromWindow();
		query = chips.q;
		debounced = chips.q;
		filterPhoto = chips.hasPhoto;
		filterLocation = chips.hasLocation;
		periodFrom = chips.from;
		periodTo = chips.to;
		periodActive = Boolean(chips.from || chips.to);
		authorFilter = chips.author;
	}

	const stats = $derived(searchStats(hits));

	$effect(() => {
		replaceSearchUrl(searchPath, {
			q: debounced,
			periodActive,
			periodFrom,
			periodTo,
			hasPhoto: filterPhoto,
			hasLocation: filterLocation,
			author: authorFilter
		});
	});

	onMount(() => {
		const onPop = () => applyUrlChips();
		window.addEventListener('popstate', onPop);
		return () => window.removeEventListener('popstate', onPop);
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
			thumbUrls = {};
			sourceError = '';
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
			sourceError = '';
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
				const urls: Record<string, string> = {};
				await Promise.all(
					next.map(async (hit) => {
						if (!hit.thumb_blob_id) return;
						const key = hit.post_id;
						try {
							urls[key] = await getMediaUrl(circle.origin, hit.thumb_blob_id);
						} catch {
							/* placeholder tint */
						}
					})
				);
				if (gen !== searchGen) return;
				thumbUrls = urls;
				if (authorFilter && !authors.includes(authorFilter)) {
					authorFilter = '';
				}
			} catch {
				if (gen !== searchGen) return;
				sourceError = 'Не удалось выполнить поиск.';
				hits = [];
				authorNames = [];
				thumbUrls = {};
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

	function chipState() {
		return { author: authorFilter, periodActive, periodFrom, periodTo };
	}

	function applyChips(next: ReturnType<typeof chipState>) {
		authorFilter = next.author;
		periodActive = next.periodActive;
		periodFrom = next.periodFrom;
		periodTo = next.periodTo;
	}

	function onToggleAuthor(name: string) {
		applyChips(toggleAuthor(chipState(), name));
	}

	function onTogglePeriod() {
		applyChips(togglePeriod(chipState()));
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={circle.name}
	tabs={false}
	commentBar={false}
	searchPlaceholder="Искать в круге"
	bind:searchQuery={query}
	onback={goBack}
>
	<ChipGroup style="margin-top:14px">
		<Chip selected>Этот круг</Chip>
		<Chip onclick={openGlobalSearch}>Все круги</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip selected={periodActive} onclick={onTogglePeriod}>Период</Chip>
		<Chip selected={filterPhoto} onclick={() => (filterPhoto = !filterPhoto)}>С фото</Chip>
		<Chip selected={filterLocation} onclick={() => (filterLocation = !filterLocation)}
			>С местом</Chip
		>
		{#if authorNames.length > 1}
			{#each authorNames as name (name)}
				<Chip selected={authorFilter === name} onclick={() => onToggleAuthor(name)}>{name}</Chip>
			{/each}
		{/if}
	</ChipGroup>
	{#if periodActive}
		<div style="display:flex;gap:8px;margin:8px 16px 0">
			<Input type="date" bind:value={periodFrom} active placeholder="с" style="flex:1" />
			<Input type="date" bind:value={periodTo} active placeholder="по" style="flex:1" />
		</div>
	{/if}
	{#if sourceError}
		<Hint style="margin-top:16px">{sourceError}</Hint>
	{:else if loading}
		<Hint style="margin-top:16px">Ищем…</Hint>
	{:else if debounced && !hits.length}
		<Hint style="margin-top:16px">Ничего не найдено</Hint>
	{:else if stats}
		<Hint>{stats}</Hint>
	{/if}
	{#each hits as hit, i (`${hit.kind}-${hit.post_id}-${hit.comment_id ?? hit.entry_date}-${i}`)}
		<SearchResultRow
			author={rowAuthor(hit)}
			time={rowTime(hit)}
			thumb={Boolean(hit.thumb_blob_id)}
			thumbUrl={thumbUrls[hit.post_id]}
			thumbVariant={searchThumbVariant(hit.post_id)}
			onclick={() => openHit(hit)}
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
	<Hint centered style="margin-top:22px">
		Фильтр по автору работает только здесь:<br />в других кругах это другие люди.
	</Hint>
</CircleLayout>

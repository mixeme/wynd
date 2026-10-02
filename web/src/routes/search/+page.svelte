<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import DateRange from '$ui/forms/DateRange.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { circleNameMap, searchAllOrigins } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { loadSessions } from '$lib/session/session.svelte';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { searchChipsFromParams, replaceSearchUrl, searchChipsFromWindow, searchThumbVariant, quoteMatch, searchHitHref } from '$lib/journal/search';

	type Hit = {
		postId: string;
		kind: string;
		entryDate: string;
		commentId?: string;
		mediaBlobId?: string;
		title?: string;
		author: string;
		time: string;
		snippet: string;
		thumbBlobId?: string;
	};

	type Group = {
		circleId: string;
		name: string;
		color: string;
		hits: Hit[];
	};

	function splitCircleKey(key: string): { origin: string; circleId: string } {
		const sep = key.lastIndexOf(':');
		if (sep < 0) return { origin: '', circleId: key };
		return { origin: key.slice(0, sep), circleId: key.slice(sep + 1) };
	}

	const initialChips = searchChipsFromParams($page.url.searchParams);
	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let filterPhoto = $state(initialChips.hasPhoto);
	let filterLocation = $state(initialChips.hasLocation);
	let periodActive = $state(Boolean(initialChips.from || initialChips.to));
	let periodFrom = $state(initialChips.from);
	let periodTo = $state(initialChips.to);
	let loading = $state(false);
	let thumbUrls = $state<Record<string, string>>({});
	let groups = $state<Group[]>([]);
	let sourceError = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let searchGen = 0;

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
	}

	$effect(() => {
		replaceSearchUrl(searchPath, {
			q: debounced,
			periodActive,
			periodFrom,
			periodTo,
			hasPhoto: filterPhoto,
			hasLocation: filterLocation
		});
	});

	// onMount обязан быть синхронным: async-вариант возвращает промис вместо
	// функции очистки, и слушатель popstate живёт после ухода с экрана (GUI-3).
	onMount(() => {
		const onPop = () => applyUrlChips();
		window.addEventListener('popstate', onPop);
		void (async () => {
			const sessions = await loadSessions();
			if (!sessions.length) goto('/');
		})();
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
			groups = [];
			thumbUrls = {};
			sourceError = '';
			return;
		}
		const from = periodActive && periodFrom ? periodFrom : undefined;
		const to = periodActive && periodTo ? periodTo : undefined;
		const photo = filterPhoto;
		const location = filterLocation;
		const gen = ++searchGen;
		void (async () => {
			loading = true;
			sourceError = '';
			try {
				const { results, failedInstanceNames } = await searchAllOrigins(debounced, {
					from,
					to,
					hasPhoto: photo,
					hasLocation: location
				});
				const names = await circleNameMap();
				if (gen !== searchGen) return;
				if (failedInstanceNames.length) {
					sourceError = `Не удалось обновить поиск: ${failedInstanceNames.join(', ')}.`;
				}
				const byCircle = new Map<string, Omit<Group, 'circleId'>>();
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
							commentId: hit.comment_id,
							mediaBlobId: hit.media_blob_id,
							title: hit.title,
							author: isDay ? (hit.title ?? hit.snippet) : (hit.author_name ?? '—'),
							time: isDay
								? formatEntryDate(hit.entry_date)
								: formatPostTime(hit.created_at ?? '', hit.entry_date),
							snippet: hit.snippet,
							thumbBlobId: hit.thumb_blob_id
						});
						byCircle.set(key, entry);
					}
				}
				const nextGroups: Group[] = [...byCircle.entries()].map(([circleId, value]) => ({
					circleId,
					...value
				}));
				const urls: Record<string, string> = {};
				await Promise.all(
					nextGroups.flatMap((group) =>
						group.hits.map(async (hit) => {
							if (!hit.thumbBlobId) return;
							const { origin } = splitCircleKey(group.circleId);
							const key = `${group.circleId}-${hit.postId}`;
							try {
								urls[key] = await getMediaUrl(origin, hit.thumbBlobId);
							} catch {
								/* placeholder tint */
							}
						})
					)
				);
				if (gen !== searchGen) return;
				thumbUrls = urls;
				groups = nextGroups;
			} catch {
				if (gen !== searchGen) return;
				sourceError = 'Не удалось выполнить поиск.';
				groups = [];
				thumbUrls = {};
			} finally {
				if (gen === searchGen) loading = false;
			}
		})();
	});

	function openHit(group: Group, hit: Hit) {
		const { origin, circleId } = splitCircleKey(group.circleId);
		if (!circleId) return;
		rememberCircleOrigin(circleId, origin);
		goto(
			searchHitHref(circleId, {
				kind: hit.kind,
				post_id: hit.postId,
				entry_date: hit.entryDate,
				comment_id: hit.commentId,
				media_blob_id: hit.mediaBlobId
			})
		);
	}

	function rowAuthor(hit: Hit) {
		if (hit.kind === 'day') return quoteMatch(hit.title ?? hit.snippet, debounced);
		return hit.author;
	}

	function togglePeriod() {
		periodActive = !periodActive;
		if (!periodActive) {
			periodFrom = '';
			periodTo = '';
		}
	}
</script>

{#snippet searchBar()}
	<SearchField autofocus style="flex:1" placeholder="Искать по всем кругам" bind:value={query} />
{/snippet}

<!-- Поле — в шапке, как в кадре 2.9. Раньше шапка повторяла запрос текстом,
     а настоящее поле стояло ниже: на экране было две строки поиска. -->
<FormLayout shell app bar={searchBar} compact onback={() => goUp('/circles')}>
	<ChipGroup style="margin-top:12px">
		<Chip selected={periodActive} onclick={togglePeriod}>Период</Chip>
		<Chip selected={filterPhoto} onclick={() => (filterPhoto = !filterPhoto)}>С фото</Chip>
		<Chip selected={filterLocation} onclick={() => (filterLocation = !filterLocation)}
			>С местом</Chip
		>
	</ChipGroup>
	{#if periodActive}
		<DateRange bind:from={periodFrom} bind:to={periodTo} />
	{/if}
	{#if sourceError}
		<Hint class="mt-16">{sourceError}</Hint>
	{/if}
	{#if loading}
		<Hint class="mt-16">Ищем…</Hint>
	{:else if debounced && !sourceError && !groups.length}
		<Hint class="mt-16">Ничего не найдено</Hint>
	{/if}
	{#each groups as group (group.circleId)}
		<SearchGroupHeader color={group.color} name={group.name} count={group.hits.length} />
		{#each group.hits as hit, i (`${group.circleId}-${i}`)}
			<SearchResultRow
				author={rowAuthor(hit)}
				time={hit.time}
				thumb={Boolean(hit.thumbBlobId)}
				thumbUrl={thumbUrls[`${group.circleId}-${hit.postId}`]}
				thumbVariant={searchThumbVariant(hit.postId)}
				kind={hit.kind}
				snippet={hit.snippet}
				query={debounced}
				onclick={() => openHit(group, hit)}
			/>
		{/each}
	{/each}
	<Hint class="mt-22" centered>
		Ищется только то, что вы застали:<br />записи до вашего прихода не найдутся.
	</Hint>
</FormLayout>

<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { detailEntryOptions } from '$lib/navigation/detailEntry';
	import DateRange from '$ui/forms/DateRange.svelte';
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { goToLogin } from '$lib/navigation/afterLogin';
	import { page } from '$app/stores';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { circleNameMap, searchAllOrigins } from '$lib/circles/circles';
	import type { SearchHit } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { isAttachedToOtherDay } from '$lib/journal/present';
	import { loadSessions } from '$lib/session/session.svelte';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { splitLayout } from '$lib/session/layout.svelte';
	import { clearWideSource, setSearchSource } from '$lib/navigation/wideSource.svelte';
	import {
		searchChipsFromParams,
		replaceSearchUrl,
		searchChipsFromWindow,
		searchThumbVariant,
		quoteMatch,
		searchHitHref,
		searchHref,
		localDayStart,
		localDayAfter
	} from '$lib/journal/search';

	// План 76: на широком окне поиск — левая колонка (поле, фильтры, результаты),
	// справа открывается найденная запись того же круга, что и на телефоне.
	// На телефоне это сам экран поиска.
	//
	// sourceUrl — адрес поиска, который принёс нас на круг из поиска. По нему
	// левая колонка восстанавливает запрос и фильтры; на самом /search источник
	// не нужен, там адрес текущий.
	let { sourceUrl }: { sourceUrl?: string } = $props();

	const SEARCH_PATH = '/search';
	// Источник задаёт первый кадр и больше не подменяет состояние: пока
	// колонка открыта, запрос и фильтры живут своей жизнью.
	const initialChips = untrack(() => {
		const initialUrl = sourceUrl ? new URL(sourceUrl, 'https://wynd.local') : $page.url;
		return searchChipsFromParams(initialUrl.searchParams);
	});
	const inPane = $derived($page.url.pathname !== SEARCH_PATH);

	type Hit = {
		postId: string;
		kind: string;
		entryDate: string;
		commentId?: string;
		mediaBlobId?: string;
		title?: string;
		author: string;
		time: string;
		entryDay?: string;
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

	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let filterPhoto = $state(initialChips.hasPhoto);
	let filterLocation = $state(initialChips.hasLocation);
	let periodActive = $state(Boolean(initialChips.from || initialChips.to));
	let periodFrom = $state(initialChips.from);
	let periodTo = $state(initialChips.to);
	let byDay = $state(initialChips.byDay);
	// Пришли из поиска круга: «Этот круг» возвращает туда с тем же запросом —
	// человек передумал искать везде (замечание владельца 2026-10-17).
	const fromCircle = initialChips.circle;
	let loading = $state(false);
	let thumbUrls = $state<Record<string, string>>({});
	let groups = $state<Group[]>([]);
	let sourceError = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let searchGen = 0;

	// С периодом, фото или местом поиск идёт и без слов (концепция, «Поиск»).
	const hasFilter = $derived(
		(periodActive && Boolean(periodFrom || periodTo)) || filterPhoto || filterLocation
	);

	function applyUrlChips() {
		const chips = searchChipsFromWindow();
		query = chips.q;
		debounced = chips.q;
		filterPhoto = chips.hasPhoto;
		filterLocation = chips.hasLocation;
		periodFrom = chips.from;
		periodTo = chips.to;
		periodActive = Boolean(chips.from || chips.to);
		byDay = chips.byDay;
	}

	function searchState() {
		return {
			q: debounced,
			periodActive,
			periodFrom,
			periodTo,
			byDay,
			hasPhoto: filterPhoto,
			hasLocation: filterLocation,
			circle: fromCircle
		};
	}

	$effect(() => {
		const state = searchState();
		if (inPane) setSearchSource(searchHref(SEARCH_PATH, state));
		else replaceSearchUrl(SEARCH_PATH, state);
	});

	function backToCircle() {
		goto(
			searchHref(`/circles/${fromCircle}/search`, {
				q: debounced || query.trim(),
				periodActive,
				periodFrom,
				periodTo,
				byDay,
				hasPhoto: filterPhoto,
				hasLocation: filterLocation
			}),
			{ replaceState: true }
		);
	}

	// onMount обязан быть синхронным: async-вариант возвращает промис вместо
	// функции очистки, и слушатель popstate живёт после ухода с экрана (GUI-3).
	onMount(() => {
		// На /search источник поиска уже перенесён в адрес; в левой колонке
		// круга он нужен, пока открыт круг.
		if (!inPane && sourceUrl) clearWideSource();
		const onPop = () => applyUrlChips();
		if (!inPane) window.addEventListener('popstate', onPop);
		void (async () => {
			const sessions = await loadSessions();
			if (!sessions.length) goToLogin();
		})();
		return () => {
			if (!inPane) window.removeEventListener('popstate', onPop);
		};
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
		if (!debounced && !hasFilter) {
			groups = [];
			thumbUrls = {};
			sourceError = '';
			return;
		}
		const from = periodActive && periodFrom ? periodFrom : undefined;
		const to = periodActive && periodTo ? periodTo : undefined;
		const day = byDay;
		const postedFrom = periodActive && !byDay && periodFrom ? localDayStart(periodFrom) : undefined;
		const postedTo = periodActive && !byDay && periodTo ? localDayAfter(periodTo) : undefined;
		const photo = filterPhoto;
		const location = filterLocation;
		const gen = ++searchGen;
		void (async () => {
			loading = true;
			sourceError = '';
			try {
				// Имена кругов — до поиска: группа рисуется, как только ответил её
				// сервер, не дожидаясь остальных (C95, «Поиск — позже»).
				const names = await circleNameMap();
				if (gen !== searchGen) return;
				const byCircle = new Map<string, Group>();
				const resolveThumb = (groupKey: string, origin: string, hit: Hit) => {
					if (!hit.thumbBlobId) return;
					const key = `${groupKey}-${hit.postId}`;
					void getMediaUrl(origin, hit.thumbBlobId)
						.then((url) => {
							if (gen !== searchGen) return;
							thumbUrls = { ...thumbUrls, [key]: url };
						})
						.catch(() => {
							/* placeholder tint */
						});
				};
				// Группа этого сервера рисуется, как только он ответил, не дожидаясь
				// остальных (C95, «Поиск — позже»); миниатюры едут по одной.
				const applyOrigin = (origin: string, hits: SearchHit[]) => {
					const built: Array<{ row: Hit; groupKey: string }> = [];
					for (const hit of hits) {
						const key = `${origin}:${hit.circle_id}`;
						const meta = names.get(key);
						const entry = byCircle.get(key) ?? {
							circleId: key,
							name: meta?.name ?? 'Круг',
							color: meta?.color ?? 'var(--slate)',
							hits: []
						};
						const isDay = hit.kind === 'day';
						const row: Hit = {
							postId: hit.post_id,
							kind: hit.kind,
							entryDate: hit.entry_date,
							commentId: hit.comment_id,
							mediaBlobId: hit.media_blob_id,
							title: hit.title,
							author: isDay ? (hit.title ?? hit.snippet) : (hit.author_name ?? '—'),
							time: isDay ? formatEntryDate(hit.entry_date) : formatPostTime(hit.created_at ?? ''),
							// Запись задним числом — справа её день, как в ленте (C49).
							entryDay:
								!isDay &&
								hit.kind !== 'comment' &&
								isAttachedToOtherDay({
									entry_date: hit.entry_date,
									created_at: hit.created_at ?? ''
								})
									? formatEntryDate(hit.entry_date)
									: undefined,
							snippet: hit.snippet,
							thumbBlobId: hit.thumb_blob_id
						};
						entry.hits.push(row);
						byCircle.set(key, entry);
						built.push({ row, groupKey: key });
					}
					groups = [...byCircle.values()];
					for (const { row, groupKey } of built) {
						resolveThumb(groupKey, splitCircleKey(groupKey).origin, row);
					}
				};
				const { failedInstanceNames } = await searchAllOrigins(
					debounced,
					{
						from,
						to,
						byDay: day,
						postedFrom,
						postedTo,
						hasPhoto: photo,
						hasLocation: location
					},
					debounced ? 50 : 500,
					(origin, hits) => {
						if (gen !== searchGen) return;
						applyOrigin(origin, hits);
					}
				);
				if (gen !== searchGen) return;
				if (failedInstanceNames.length) {
					sourceError = `Не удалось обновить поиск: ${failedInstanceNames.join(', ')}.`;
				}
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
		// На широком окне левая колонка остаётся поиском: запоминаем её адрес
		// до перехода на круг. Смена результата заменяет открытый круг.
		if (splitLayout()) setSearchSource(searchHref(SEARCH_PATH, searchState()));
		goto(
			searchHitHref(circleId, {
				kind: hit.kind,
				post_id: hit.postId,
				entry_date: hit.entryDate,
				comment_id: hit.commentId,
				media_blob_id: hit.mediaBlobId
			}),
			detailEntryOptions(inPane)
		);
	}

	function rowAuthor(hit: Hit) {
		if (hit.kind === 'day') return quoteMatch(hit.title ?? hit.snippet, debounced);
		return hit.author;
	}

	function onTogglePeriod(day: boolean) {
		if (periodActive && byDay === day) {
			periodActive = false;
			periodFrom = '';
			periodTo = '';
			byDay = false;
			return;
		}
		periodActive = true;
		byDay = day;
	}
</script>

{#snippet searchBar()}
	<SearchField
		class="grow"
		autofocus={!inPane}
		placeholder="Искать по всем кругам"
		bind:value={query}
	/>
{/snippet}

<!-- Поле — в шапке, как в кадре 2.19. Раньше шапка повторяла запрос текстом,
     а настоящее поле стояло ниже: на экране было две строки поиска. -->
<FormLayout shell app bar={searchBar} compact barMinHeight={60} onback={navigateBack}>
	{#if fromCircle}
		<ChipGroup class="mt-8">
			<Chip onclick={backToCircle}>Этот круг</Chip>
			<Chip selected>Все круги</Chip>
		</ChipGroup>
	{/if}
	<ChipGroup class={fromCircle ? undefined : 'mt-8'}>
		<Chip selected={periodActive && !byDay} onclick={() => onTogglePeriod(false)}>Период</Chip>
		<Chip selected={periodActive && byDay} onclick={() => onTogglePeriod(true)}>По дню</Chip>
		<Chip selected={filterPhoto} onclick={() => (filterPhoto = !filterPhoto)}>С фото</Chip>
		<Chip selected={filterLocation} onclick={() => (filterLocation = !filterLocation)}
			>С местом</Chip
		>
	</ChipGroup>
	{#if periodActive}
		<DateRange bind:from={periodFrom} bind:to={periodTo} />
		<Hint class="mt-8">{byDay ? 'Даты — по дню записи.' : 'Даты — по дате публикации.'}</Hint>
	{/if}
	{#if sourceError}
		<Hint class="mt-16">{sourceError}</Hint>
	{/if}
	{#if loading}
		<Hint class="mt-16">Ищем…</Hint>
	{:else if (debounced || hasFilter) && !sourceError && !groups.length}
		<Hint class="mt-16">Ничего не найдено</Hint>
	{/if}
	{#each groups as group (group.circleId)}
		<SearchGroupHeader color={group.color} name={group.name} count={group.hits.length} />
		{#each group.hits as hit, i (`${group.circleId}-${i}`)}
			<SearchResultRow
				author={rowAuthor(hit)}
				time={hit.time}
				entryDay={hit.entryDay}
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
	<Hint class="mt-16" centered>Ищется только то, что вы застали.</Hint>
</FormLayout>

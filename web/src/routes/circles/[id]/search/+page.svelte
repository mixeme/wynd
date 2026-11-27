<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import DateRange from '$ui/forms/DateRange.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { formatEntryDate, formatPostTime } from '$lib/format/time';
	import { isAttachedToOtherDay } from '$lib/journal/present';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import {
		searchChipsFromParams,
		searchCircle,
		searchCircleAuthors,
		searchHref,
		replaceSearchUrl,
		searchChipsFromWindow,
		searchThumbVariant,
		searchHitHref,
		localDayStart,
		localDayAfter
	} from '$lib/journal/search';
	import { searchStats, toggleAuthor, togglePeriod } from '$lib/journal/searchState';
	import {
		recallSearchScreen,
		rememberSearchScreen,
		searchScreenKey
	} from '$lib/journal/search-memory';
	import type { CircleSearchHit } from '$lib/journal/types';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const searchPath = $derived($page.url.pathname);
	const initialChips = searchChipsFromParams($page.url.searchParams);
	let query = $state(initialChips.q);
	let debounced = $state(initialChips.q);
	let authorFilter = $state(initialChips.author);
	let filterPhoto = $state(initialChips.hasPhoto);
	let filterLocation = $state(initialChips.hasLocation);
	let periodActive = $state(Boolean(initialChips.from || initialChips.to));
	let periodFrom = $state(initialChips.from);
	let periodTo = $state(initialChips.to);
	let byDay = $state(initialChips.byDay);

	function screenState() {
		return {
			q: debounced,
			periodActive,
			periodFrom,
			periodTo,
			byDay,
			hasPhoto: filterPhoto,
			hasLocation: filterLocation,
			author: authorFilter
		};
	}

	function memoryKey() {
		return searchScreenKey(circle.origin, circle.circleId, searchPath, screenState());
	}

	// Возврат с записи: экран монтируется заново и терял строки — на кадр
	// появлялись «Ищем…» и пустое место (C160). Восстанавливаем прежний кадр
	// до первой отрисовки; сеть уточнит его молча.
	const remembered = recallSearchScreen(memoryKey());
	let restoredFromMemory = Boolean(remembered);
	let loading = $state(remembered?.loading ?? false);
	let hits = $state<CircleSearchHit[]>(remembered?.hits ?? []);
	let authorNames = $state<string[]>(remembered?.authors ?? []);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let searchGen = 0;
	let thumbUrls = $state<Record<string, string>>(remembered?.thumbs ?? {});
	let sourceError = $state('');

	// Превью, которых в памяти ещё нет, — из кэша устройства: строки не ждут
	// самых медленных (C95), а кадр возврата не ждёт повторной выдачи.
	if (remembered) {
		for (const hit of remembered.hits) {
			if (!hit.thumb_blob_id || remembered.thumbs[hit.post_id]) continue;
			void getMediaUrl(circle.origin, hit.thumb_blob_id)
				.then((url) => {
					thumbUrls = { ...thumbUrls, [hit.post_id]: url };
				})
				.catch(() => {
					/* placeholder tint */
				});
		}
	}

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
		authorFilter = chips.author;
	}

	const stats = $derived(searchStats(hits));
	// Текст — один фильтр из многих (концепция, «Поиск»): с периодом, фото,
	// местом или автором поиск идёт и без слов — «все записи за период».
	const hasFilter = $derived(
		(periodActive && Boolean(periodFrom || periodTo)) ||
			filterPhoto ||
			filterLocation ||
			Boolean(authorFilter)
	);

	$effect(() => {
		replaceSearchUrl(searchPath, {
			q: debounced,
			periodActive,
			periodFrom,
			periodTo,
			byDay,
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
		// Тихий прогон — только первый после возврата: строки уже на экране,
		// «Ищем…» не показываем, сеть обновляет их молча. Дальше — как всегда.
		const silent = restoredFromMemory;
		restoredFromMemory = false;
		if (!debounced && !hasFilter) {
			// Пустой запрос отменяет и ответ, который ещё в пути: иначе он
			// вернул бы строки под уже очищенным поиском.
			searchGen++;
			loading = false;
			hits = [];
			authorFilter = '';
			authorNames = [];
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
		const author = authorFilter || undefined;
		const gen = ++searchGen;
		void (async () => {
			if (!silent) loading = true;
			sourceError = '';
			try {
				const chipFilters = {
					from,
					to,
					byDay: day,
					postedFrom,
					postedTo,
					hasPhoto: photo,
					hasLocation: location
				};
				const [next, authors] = await Promise.all([
					searchCircle(circle.origin, circle.circleId, debounced, debounced ? 50 : 500, {
						...chipFilters,
						author
					}),
					searchCircleAuthors(circle.origin, circle.circleId, debounced, chipFilters)
				]);
				if (gen !== searchGen) return;
				hits = next;
				authorNames = authors;
				// Миниатюры едут по одной: строка не ждёт самых медленных (C95).
				for (const hit of next) {
					if (!hit.thumb_blob_id) continue;
					const key = hit.post_id;
					void getMediaUrl(circle.origin, hit.thumb_blob_id)
						.then((url) => {
							if (gen !== searchGen) return;
							thumbUrls = { ...thumbUrls, [key]: url };
						})
						.catch(() => {
							/* placeholder tint */
						});
				}
				if (authorFilter && !authors.includes(authorFilter)) {
					authorFilter = '';
				}
			} catch {
				if (gen !== searchGen) return;
				if (silent) return;
				sourceError = 'Не удалось выполнить поиск.';
				hits = [];
				authorNames = [];
				thumbUrls = {};
			} finally {
				if (gen === searchGen) loading = false;
			}
		})();
	});

	// Что видно на экране, то и запоминаем: «Назад» с записи вернёт этот кадр
	// целиком — строки, авторов и превью, без «Ищем…» и пустого места (C160).
	$effect(() => {
		rememberSearchScreen(memoryKey(), { hits, authors: authorNames, thumbs: thumbUrls, loading });
	});

	function goBack() {
		void navigateBack();
	}

	function openGlobalSearch() {
		goto(
			searchHref('/search', {
				q: debounced || query.trim(),
				periodActive,
				periodFrom,
				periodTo,
				byDay,
				hasPhoto: filterPhoto,
				hasLocation: filterLocation,
				circle: circle.circleId
			}),
			// Переключение «Этот круг / Все круги» — замена экрана, не шаг вглубь.
			{ replaceState: true }
		);
	}

	function openHit(hit: CircleSearchHit) {
		goto(searchHitHref(circle.circleId, hit));
	}

	function rowAuthor(hit: CircleSearchHit) {
		if (hit.kind === 'day') return hit.title ?? hit.snippet;
		return hit.author_name ?? '—';
	}

	function rowTime(hit: CircleSearchHit) {
		if (hit.kind === 'day') return formatEntryDate(hit.entry_date);
		return formatPostTime(hit.created_at);
	}

	// Запись задним числом — справа её день, как в ленте (C49); у комментария нет.
	function rowEntryDay(hit: CircleSearchHit) {
		if (hit.kind === 'day' || hit.kind === 'comment') return undefined;
		return isAttachedToOtherDay(hit) ? formatEntryDate(hit.entry_date) : undefined;
	}

	function chipState() {
		return { author: authorFilter, periodActive, periodFrom, periodTo, byDay };
	}

	function applyChips(next: ReturnType<typeof chipState>) {
		authorFilter = next.author;
		periodActive = next.periodActive;
		periodFrom = next.periodFrom;
		periodTo = next.periodTo;
		byDay = next.byDay;
	}

	function onToggleAuthor(name: string) {
		applyChips(toggleAuthor(chipState(), name));
	}

	function onTogglePeriod(day: boolean) {
		applyChips(togglePeriod(chipState(), day));
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
	<ChipGroup class="mt-16">
		<Chip selected>Этот круг</Chip>
		<Chip onclick={openGlobalSearch}>Все круги</Chip>
	</ChipGroup>
	<ChipGroup>
		<Chip selected={periodActive && !byDay} onclick={() => onTogglePeriod(false)}>Период</Chip>
		<Chip selected={periodActive && byDay} onclick={() => onTogglePeriod(true)}>По дню</Chip>
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
		<DateRange bind:from={periodFrom} bind:to={periodTo} />
		<Hint class="mt-8">{byDay ? 'Даты — по дню записи.' : 'Даты — по дате публикации.'}</Hint>
	{/if}
	{#if sourceError}
		<Hint class="mt-16">{sourceError}</Hint>
	{:else if loading}
		<Hint class="mt-16">Ищем…</Hint>
	{:else if (debounced || hasFilter) && !hits.length}
		<Hint class="mt-16">Ничего не найдено</Hint>
	{:else if stats}
		<Hint>{stats}</Hint>
	{/if}
	{#each hits as hit, i (`${hit.kind}-${hit.post_id}-${hit.comment_id ?? hit.entry_date}-${i}`)}
		<SearchResultRow
			author={rowAuthor(hit)}
			time={rowTime(hit)}
			entryDay={rowEntryDay(hit)}
			thumb={Boolean(hit.thumb_blob_id)}
			thumbUrl={thumbUrls[hit.post_id]}
			thumbVariant={searchThumbVariant(hit.post_id)}
			kind={hit.kind}
			snippet={hit.snippet}
			query={debounced}
			onclick={() => openHit(hit)}
		/>
	{/each}
	<Hint class="mt-8" centered>
		Фильтр по автору работает только здесь:<br />в других кругах это другие люди.
	</Hint>
</CircleLayout>

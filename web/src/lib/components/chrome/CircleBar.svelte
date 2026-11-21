<script module lang="ts">
	// «Отклики» — пятая вкладка, а не пятый уровень (3.17): тот же журнал, другой срез.
	export const CIRCLE_TABS = ['Хронология', 'Дни', 'Сетка', 'Карта', 'Отклики'] as const;
	export type CircleTab = (typeof CIRCLE_TABS)[number];
	/** Путь вкладки от /circles/{id}. */
	export const CIRCLE_TAB_PATHS: Record<CircleTab, string> = {
		Хронология: '',
		Дни: '/days',
		Сетка: '/grid',
		Карта: '/map',
		Отклики: '/responses'
	};
	/** Вкладки по порядку; «Отклики» — только когда в круге есть другие. */
	export function visibleCircleTabs(responsesTab: boolean, active: CircleTab): CircleTab[] {
		return CIRCLE_TABS.filter((t) => t !== 'Отклики' || responsesTab || active === 'Отклики');
	}
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { openCircleTab, tabTap } from '$lib/navigation/circleTabs';
	import IconButton from '$ui/forms/IconButton.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import { Tabs } from 'bits-ui';
	import type { Snippet } from 'svelte';

	let {
		title,
		identity,
		avatar,
		avatarSrc,
		tabs = true,
		active = $bindable<CircleTab>('Хронология'),
		// Текущий экран — сама вкладка (C106): на экране дня «Дни» уже
		// подсвечены, но касание должно вести в список дней, а не наверх.
		atTab = true,
		circleId,
		onback,
		onsearch,
		searchPlaceholder,
		searchQuery = $bindable(''),
		subtitle,
		identitySettingsLink = true,
		responsesTab = true,
		previewTab,
		responsesUnread = 0,
		action,
		onaction
	}: {
		title: string;
		identity?: string;
		avatar?: string;
		avatarSrc?: string;
		tabs?: boolean;
		active?: CircleTab;
		atTab?: boolean;
		circleId?: string;
		onback?: () => void;
		onsearch?: () => void;
		searchPlaceholder?: string;
		searchQuery?: string;
		/** Строка под названием; snippet — когда в ней значок (шапка альбома). */
		subtitle?: string | Snippet;
		identitySettingsLink?: boolean;
		/** В круге из одного откликаться некому — вкладки нет (3.7). */
		responsesTab?: boolean;
		/** Свайп прошёл треть — полоска уже на вкладке, куда перейдём (C9). */
		previewTab?: CircleTab;
		/** Число на вкладке: новые отклики с прошлого просмотра. */
		responsesUnread?: number;
		/** Правое действие в шапке без прав — «Готово» в альбоме (план 61, В-2). */
		action?: string;
		onaction?: () => void;
	} = $props();

	const visibleTabs = $derived(visibleCircleTabs(responsesTab, active));

	const searchInBar = $derived(searchPlaceholder !== undefined);

	const tabPaths = CIRCLE_TAB_PATHS;

	// Повторное нажатие на открытую вкладку поднимает экран в самый верх —
	// как в любом приложении с вкладками. Прокручивается не окно, а список
	// внутри окна приложения, поэтому поднимаем всё, что внутри прокручено.
	function scrollToTop(from: HTMLElement) {
		const root = from.closest('.ph') ?? document.body;
		for (const node of root.querySelectorAll<HTMLElement>('*')) {
			if (node.scrollTop > 0) node.scrollTo({ top: 0, behavior: 'smooth' });
		}
	}

	// Открыта ли вкладка была до касания: Bits может переключить её ещё на
	// нажатии, и к click первое касание выглядело бы повторным.
	let tappedOpen = false;

	function onTabTap(tab: CircleTab, e: MouseEvent, bitsClick?: (e: MouseEvent) => void) {
		const same = tappedOpen && active === tab;
		tappedOpen = false;
		bitsClick?.(e);
		if (!same) return;
		// Повтор на уже открытой вкладке (bits значение не меняет и
		// onTabChange не зовёт): на самой вкладке — наверх, на чужом экране
		// (день, альбом дня) — в список этой вкладки (C106).
		if (atTab) scrollToTop(e.currentTarget as HTMLElement);
		else if (circleId) void openCircleTab(circleId, tabPaths[tab]);
	}

	function onTabChange(tab: string) {
		const t = tab as CircleTab;
		// Смена значения: повторная вкладка сюда не доходит (см. onTabTap).
		if (t === active) return;
		// Решение — до присваивания: tabTap отличает чужую вкладку от повтора
		// на уже открытой. После `active = t` он видел бы повтор всегда.
		const tap = tabTap(active, t, atTab, Boolean(circleId));
		active = t;
		// Замена записи, а не шаг: вкладки — виды одного экрана. Иначе «Назад»
		// идёт обратно по переключениям, а не туда, откуда открыли круг
		// (владелец, 18.11).
		if (tap !== 'navigate') return;
		void openCircleTab(circleId!, tabPaths[t]);
	}
</script>

<!-- Правило C98: у поиска в шапке поле стоит в центре полосы 60 (как у общего
     поиска), без запаса под строкой; без вкладок и подписи полоса — 48. -->
<div class="cbar" class:search-band={searchInBar}>
	<div class="top">
		{#if onback}
			<IconButton name="back" label="Назад" onclick={() => onback()} />
		{/if}
		{#if searchInBar}
			<SearchField
				class="inv"
				autofocus
				bind:value={searchQuery}
				placeholder={searchPlaceholder!}
			/>
		{:else}
			<span class="t">{title}</span>
			{#if onsearch}
				<span class="sp"></span>
				<IconButton name="search" label="Поиск" onclick={() => onsearch()} />
			{/if}
			{#if action && onaction}
				<span class="sp"></span>
				<button type="button" class="bar-act" onclick={() => onaction?.()}>{action}</button>
			{/if}
		{/if}
		{#if !searchInBar && identity}
			{#if circleId && avatar && identitySettingsLink}
				<button type="button" class="idn" onclick={() => goto(`/circles/${circleId}/settings`)}>
					<span class="idn-t">{identity}</span><Avatar
						initial={avatar}
						src={avatarSrc}
						style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
					/>
				</button>
			{:else}
				<span class="idn"
					><span class="idn-t">{identity}</span>{#if avatar}<Avatar
							initial={avatar}
							src={avatarSrc}
							style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
						/>{/if}</span
				>
			{/if}
		{/if}
	</div>
	{#if tabs}
		<Tabs.Root value={active} onValueChange={(v) => v && onTabChange(v)}>
			<Tabs.List class="tabs">
				{#each visibleTabs as tab (tab)}
					<Tabs.Trigger value={tab}>
						{#snippet child({ props })}
							<span
								{...props}
								class:on={(previewTab ?? active) === tab}
								onpointerdowncapture={() => (tappedOpen = active === tab)}
								onclick={(e) =>
									onTabTap(tab, e, props.onclick as ((e: MouseEvent) => void) | undefined)}
								>{tab}{#if tab === 'Отклики' && responsesUnread > 0 && active !== tab}<b
										class="tab-cnt"
										aria-label="новых: {responsesUnread}">{responsesUnread}</b
									>{/if}</span
							>
						{/snippet}
					</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
	{:else if subtitle}
		<!-- Правило C98 (вариант A): вторая строка по центру блока 24 — 3 сверху и снизу. -->
		<div style="padding:3px 0;font-size:12.5px;line-height:18px;color:rgba(255,255,255,.85)">
			{#if typeof subtitle === 'function'}{@render subtitle()}{:else}{subtitle}{/if}
		</div>
	{/if}
</div>

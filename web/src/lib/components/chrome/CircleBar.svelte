<script module lang="ts">
	// «Отклики» — пятая вкладка, а не пятый уровень (3.12): тот же журнал, другой срез.
	export const CIRCLE_TABS = ['Хронология', 'Дни', 'Сетка', 'Карта', 'Отклики'] as const;
	export type CircleTab = (typeof CIRCLE_TABS)[number];
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import IconButton from '$ui/forms/IconButton.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import { Tabs } from 'bits-ui';

	let {
		title,
		identity,
		avatar,
		avatarSrc,
		tabs = true,
		active = $bindable<CircleTab>('Хронология'),
		circleId,
		onback,
		onsearch,
		searchPlaceholder,
		searchQuery = $bindable(''),
		subtitle,
		identitySettingsLink = true,
		responsesTab = true,
		responsesUnread = 0
	}: {
		title: string;
		identity?: string;
		avatar?: string;
		avatarSrc?: string;
		tabs?: boolean;
		active?: CircleTab;
		circleId?: string;
		onback?: () => void;
		onsearch?: () => void;
		searchPlaceholder?: string;
		searchQuery?: string;
		subtitle?: string;
		identitySettingsLink?: boolean;
		/** В круге из одного откликаться некому — вкладки нет (3.7). */
		responsesTab?: boolean;
		/** Число на вкладке: новые отклики с прошлого просмотра. */
		responsesUnread?: number;
	} = $props();

	const visibleTabs = $derived(
		CIRCLE_TABS.filter((t) => t !== 'Отклики' || responsesTab || active === 'Отклики')
	);

	const searchInBar = $derived(searchPlaceholder !== undefined);

	const tabPaths: Record<CircleTab, string> = {
		Хронология: '',
		Дни: '/days',
		Сетка: '/grid',
		Карта: '/map',
		Отклики: '/responses'
	};

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
		const again = tappedOpen && active === tab;
		tappedOpen = false;
		bitsClick?.(e);
		if (again) scrollToTop(e.currentTarget as HTMLElement);
	}

	function onTabChange(tab: string) {
		const t = tab as CircleTab;
		active = t;
		if (circleId) {
			const suffix = tabPaths[t];
			goto(suffix ? `/circles/${circleId}${suffix}` : `/circles/${circleId}`);
		}
	}
</script>

<div class="cbar">
	<div class="top">
		{#if onback}
			<IconButton name="back" label="Назад" onclick={() => onback()} />
		{/if}
		{#if searchInBar}
			<SearchField class="inv" autofocus bind:value={searchQuery} placeholder={searchPlaceholder!} />
		{:else}
			<span class="t">{title}</span>
			{#if onsearch}
				<span class="sp"></span>
				<IconButton name="search" label="Поиск" onclick={() => onsearch()} />
			{/if}
		{/if}
		{#if !searchInBar && identity}
			{#if circleId && avatar && identitySettingsLink}
				<button type="button" class="idn" onclick={() => goto(`/circles/${circleId}/settings`)}>
					{identity}<Avatar
						initial={avatar}
						src={avatarSrc}
						style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
					/>
				</button>
			{:else}
				<span class="idn">{identity}{#if avatar}<Avatar
							initial={avatar}
							src={avatarSrc}
							style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
						/>{/if}</span>
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
								class:on={active === tab}
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
		<div style="padding-bottom:12px;font-size:12.5px;color:rgba(255,255,255,.85)">{subtitle}</div>
	{:else}
		<div style="height:12px"></div>
	{/if}
</div>

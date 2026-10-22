<script lang="ts">
	import PullRefreshBand from '$ui/data/PullRefresh.svelte';
	import { PullRefresh } from '$lib/gestures/pullRefresh.svelte';
	import EmptyState from '$ui/data/EmptyState.svelte';
	import { uuid } from '$lib/uuid';
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import FoldHeader from '$ui/data/FoldHeader.svelte';
	import GroupFoldCard from '$ui/data/GroupFoldCard.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import PayStreetBanner from '$ui/data/PayStreetBanner.svelte';
	import NewsBanner from '$ui/data/NewsBanner.svelte';
	import { hasUnseenNews, latestNewsVersion, markNewsSeen } from '$lib/news/seen';
	import IconButton from '$ui/forms/IconButton.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import {
		byFreshness,
		loadStreet,
		sortStreetCircles,
		type StreetCircle
	} from '$lib/circles/circles';
	import {
		applyStreetOpToView,
		groupOrigins,
		queueStreetOp,
		type StreetGroupView
	} from '$lib/circles/street';
	import type { StreetOpRecord } from '$lib/idb/db';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import {
		LONG_PRESS_MS,
		markLongPress,
		swallowsClick,
		type PressMark
	} from '$lib/gestures/longpress';
	import { displayHost } from '$lib/auth/origin';
	import { isServerDown } from '$lib/session/connection.svelte';
	import { deleteGroup, listGroups, putGroup } from '$lib/idb/db';
	import { registerRefetch } from '$lib/sync/sync';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import {
		dismissPayBanner,
		fetchPayStatus,
		formatPayDate,
		type PayStatus
	} from '$lib/pay/pay';
	import type { SessionRecord } from '$lib/idb/db';

	let circles = $state<StreetCircle[]>([]);
	// Группы со всех серверов, сложенные по названию (A2): название — ключ.
	let groups = $state<StreetGroupView[]>([]);
	let sessions = $state<SessionRecord[]>([]);
	let payStatus = $state<PayStatus | undefined>();
	let loading = $state(true);
	// «Что нового?» — пока свежую запись не открыли и не закрыли (7.7).
	let newsUnseen = $state(hasUnseenNews());
	// Отметка последнего длинного нажатия: клик в её хвосте — часть того же
	// жеста (GUI-6). Прежний общий флаг гасили в каждом обработчике, и
	// порядок pointerup/click решал исход.
	let pressMark = $state<PressMark>(null);
	let pinMenuKey = $state<string | null>(null);
	let creatingGroup = $state(false);
	let newGroupName = $state('');
	let groupMenuName = $state<string | null>(null);
	let editingGroupName = $state<string | null>(null);
	let editGroupName = $state('');
	let groupAddOpenName = $state<string | null>(null);
	let fabMenuOpen = $state(false);
	let groupLongPressTimer: ReturnType<typeof setTimeout> | undefined;
	let longPressTimer: ReturnType<typeof setTimeout> | undefined;
	let fabLongPressTimer: ReturnType<typeof setTimeout> | undefined;

	// Обновление жестом (3.5) — и здесь, не только в ленте круга: тянешь список
	// от верха, знак дорисовывается, отпустил — круги перечитываются.
	let listEl: HTMLDivElement | undefined = $state();

	function listScrollTop(): number {
		return (listEl?.closest('.shell-body') as HTMLElement | null)?.scrollTop ?? 0;
	}

	const ptr = new PullRefresh(listScrollTop, () => refresh());
	onDestroy(() => ptr.destroy());

	// Обновления идут вперемешку (жест, поток, действие на улочке); ответ
	// молчащего сервера приходит через секунды и не должен перетирать более
	// свежий — показывается только последнее.
	let refreshSeq = 0;

	async function refresh() {
		const seq = ++refreshSeq;
		const street = await loadStreet();
		if (seq !== refreshSeq) return;
		circles = street.circles;
		groups = street.groups;
		if (streetSession) {
			try {
				payStatus = await fetchPayStatus(streetSession.origin);
			} catch {
				payStatus = undefined;
			}
		}
		loading = false;
	}

	onMount(() => {
		let unsubs: Array<() => void> = [];
		void initSession()
			.then(() => loadSessions())
			.then((loaded) => {
				sessions = loaded;
				if (!loaded.length) {
					goto('/');
					return;
				}
				void refresh();
				unsubs = loaded.map((session) =>
					registerRefetch({ origin: session.origin, refetch: refresh })
				);
			});
		return () => unsubs.forEach((u) => u());
	});

	// Молчит один сервер из нескольких — общей полосы нет, сказано у его кругов (7.40).
	function silentHost(circle: StreetCircle): string | undefined {
		return isServerDown(circle.origin) ? displayHost(circle.origin) : undefined;
	}

	const circleMap = $derived(new Map(circles.map((c) => [circleKey(c), c])));
	const groupedKeys = $derived(new Set(groups.flatMap((g) => g.keys)));
	const pinned = $derived(circles.filter((c) => c.pinned));
	const rest = $derived(circles.filter((c) => !c.pinned && !groupedKeys.has(circleKey(c))));
	const empty = $derived(!loading && circles.length === 0);
	const streetSession = $derived(sessions[0]);

	// Рассказ пустой улочки (2.6): что такое Wynd и чем он не соцсеть.
	const streetFacts = $derived([
		{
			title: 'Круги вместо ленты',
			text: 'У каждого круга свой журнал и свои люди: семья, друзья, коллеги. Общей ленты нет — записи читают только внутри круга.'
		},
		{
			title: 'В каждом круге — своё имя',
			text: 'Как вас зовут в семье и как среди друзей, решаете вы. Имя из одного круга в другой не переходит.'
		},
		{
			title: 'Видно то, что вы застали',
			text: 'Кто пришёл позже, не видит того, что было до него. Кто ушёл — того, что стало после.'
		},
		{
			title: 'Посторонних нет',
			text: 'Ни подписчиков, ни рекомендаций, ни рекламы. В круг попадают только по ссылке от того, кто уже в нём.'
		},
		{
			title: `Всё хранится на ${streetSession ? displayHost(streetSession.origin) : 'этом сервере'}`,
			text: 'Сервер держит его администратор, данные на нём не зашифрованы. Пользуясь им, вы доверяете этому человеку.'
		}
	]);
	const showDonateBanner = $derived(
		payStatus?.banner && !payStatus.dismissed && payStatus.has_requisites
	);
	const showReminder = $derived(
		Boolean(
			payStatus?.reminder &&
				payStatus.expires_at &&
				payStatus.has_requisites &&
				!payStatus.pending
		)
	);
	const showPendingNotice = $derived(
		payStatus?.pending && !payStatus.expired && payStatus.pending_at
	);

	function circleKey(circle: StreetCircle): string {
		return `${circle.origin}:${circle.id}`;
	}

	// Закрепление — переезд, как «Отмеченные» в Slack (владелец, 2026-10-22:
	// «С переездом»): закреплённый круг стоит только сверху, группа его помнит
	// и покажет снова, когда открепят. Счётчик группы — по тому, что в ней
	// видно: раньше он считал и уехавшие, и группа с числом была пустой.
	function circlesInGroup(group: StreetGroupView): StreetCircle[] {
		return group.keys
			.map((key) => circleMap.get(key))
			.filter((c): c is StreetCircle => c !== undefined && !c.pinned)
			.sort(byFreshness);
	}

	function groupMemberCount(group: StreetGroupView): number {
		return circlesInGroup(group).length;
	}

	function circleGroupName(circle: StreetCircle): string | undefined {
		const key = circleKey(circle);
		return groups.find((g) => g.keys.includes(key))?.name;
	}

	// Круг, где ещё не выбрано имя, в группу не кладётся: участия пока нет.
	// Вышедший с правом читать — кладётся (A2): круг на его улочке.
	function addableCirclesForGroup(group: StreetGroupView): StreetCircle[] {
		// Закреплённый в чипах не предлагается: в группе его сейчас не видно бы.
		return circles.filter((c) => !c.pendingJoin && !c.pinned && !group.keys.includes(circleKey(c)));
	}

	function showGroupChips(group: StreetGroupView): boolean {
		if (!addableCirclesForGroup(group).length) return false;
		return groupMemberCount(group) === 0 || groupAddOpenName === group.name;
	}

	function openCircle(circle: StreetCircle) {
		// Нажатие при открытом меню закрывает его и на этом останавливается —
		// иначе тем же тапом человек проваливался бы в круг.
		const menuOpen = pinMenuKey !== null || groupMenuName !== null || fabMenuOpen;
		pinMenuKey = null;
		groupMenuName = null;
		fabMenuOpen = false;
		if (menuOpen || swallowsClick(pressMark)) return;
		rememberCircleOrigin(circle.id, circle.origin);
		goto(circle.pendingJoin ? `/circles/${circle.id}/join` : `/circles/${circle.id}`);
	}

	// Тап мимо открытого меню закрывает его. Отметка «жест поглощён» — как
	// после удержания: иначе тот же тап по строке ещё и открывал бы круг.
	function onWindowPointerDown(e: PointerEvent) {
		const target = e.target instanceof Element ? e.target : null;
		if (!target) return;
		let closed = false;
		if (fabMenuOpen && !target.closest('.fab-wrap')) {
			fabMenuOpen = false;
			closed = true;
		}
		if ((pinMenuKey !== null || groupMenuName !== null) && !target.closest('.circle-row-card')) {
			pinMenuKey = null;
			groupMenuName = null;
			closed = true;
		}
		if (closed) pressMark = markLongPress();
	}

	// Один круг может стоять и в «Закреплённых», и в группе: меню открывается
	// у той строки, которую держали, — ключ несёт раздел.
	function menuKey(section: string, circle: StreetCircle): string {
		return `${section}|${circleKey(circle)}`;
	}

	function startLongPress(circle: StreetCircle, section: string) {
		if (circle.pendingJoin) return;
		clearTimeout(longPressTimer);
		longPressTimer = setTimeout(() => {
			pressMark = markLongPress();
			pinMenuKey = menuKey(section, circle);
		}, LONG_PRESS_MS);
	}

	function cancelLongPress() {
		clearTimeout(longPressTimer);
	}

	// Закрепление и группы — изменения раскладки на сервере круга (A2): уходят
	// при обновлении списка, молчащий сервер получит их, когда ответит.
	// Изменение — сразу на экране, серверам — в фоне: молчащий отвечал бы
	// секундами, а раскладка до A2 менялась мгновенно.
	async function changeStreet(ops: StreetOpRecord[]) {
		for (const op of ops) {
			await queueStreetOp(op);
			groups = applyStreetOpToView(groups, op);
		}
		void refresh();
	}

	async function confirmPin(circle: StreetCircle) {
		const pinnedAt = circle.pinned ? 0 : Date.now();
		pinMenuKey = null;
		circles = sortStreetCircles(
			circles.map((c) =>
				c === circle ? { ...c, pinned: pinnedAt > 0, pinnedAt } : c
			)
		);
		await changeStreet([
			pinnedAt
				? { origin: circle.origin, op: 'pin', circle_id: circle.id, pinned_at: pinnedAt }
				: { origin: circle.origin, op: 'unpin', circle_id: circle.id }
		]);
	}

	let groupDownAt = 0;
	let groupMenuOpenedAt = 0;

	function onGroupPointerDown(name: string, e: PointerEvent) {
		if (e.pointerType === 'mouse' && e.button !== 0) return;
		groupDownAt = Date.now();
		startGroupLongPress(name);
	}

	function onGroupPointerUp(group: StreetGroupView) {
		const short = Date.now() - groupDownAt < LONG_PRESS_MS;
		const openedByThisHold = groupMenuOpenedAt >= groupDownAt && groupMenuName === group.name;
		cancelGroupLongPress();
		if (!short || openedByThisHold || swallowsClick(pressMark)) return;
		groupMenuName = null;
		void setGroupCollapsed(group, !group.collapsed);
	}

	// Свёрнута ли группа — только на этом устройстве: запись по названию.
	// Записи нет — группа пришла с сервера, значит, в ней бывали круги.
	async function setGroupCollapsed(group: StreetGroupView, collapsed: boolean) {
		groups = groups.map((item) => (item.name === group.name ? { ...item, collapsed } : item));
		const record = (await listGroups()).find((g) => g.name === group.name);
		await putGroup({
			id: record?.id ?? uuid(),
			name: group.name,
			circleIds: [],
			collapsed,
			filled: record ? record.filled : true
		});
	}

	async function addCircleToGroup(circle: StreetCircle, name: string) {
		await changeStreet([{ origin: circle.origin, op: 'group', circle_id: circle.id, name }]);
		const group = groups.find((g) => g.name === name);
		if (group && addableCirclesForGroup(group).length === 0) {
			groupAddOpenName = null;
		}
	}

	async function removeFromGroup(circle: StreetCircle) {
		if (!circleGroupName(circle)) return;
		pinMenuKey = null;
		await changeStreet([{ origin: circle.origin, op: 'ungroup', circle_id: circle.id }]);
	}

	// Новая группа пуста — серверам она не нужна, живёт на устройстве, пока
	// в неё не положили круг.
	async function createGroup() {
		const name = newGroupName.trim();
		creatingGroup = false;
		newGroupName = '';
		fabMenuOpen = false;
		if (!name || groups.some((g) => g.name === name)) return;
		await putGroup({ id: uuid(), name, circleIds: [], collapsed: false });
		groups = [...groups, { name, keys: [], collapsed: false }].sort((a, b) =>
			a.name.localeCompare(b.name, 'ru')
		);
	}

	function startGroupLongPress(name: string) {
		clearTimeout(groupLongPressTimer);
		groupLongPressTimer = setTimeout(() => {
			pressMark = markLongPress();
			groupMenuName = name;
			groupMenuOpenedAt = Date.now();
			editingGroupName = null;
		}, LONG_PRESS_MS);
	}

	function cancelGroupLongPress() {
		clearTimeout(groupLongPressTimer);
	}

	function startRenameGroup(group: StreetGroupView) {
		editingGroupName = group.name;
		editGroupName = group.name;
		groupMenuName = null;
	}

	// Переименование — одна запись на каждом сервере, где есть круги группы.
	async function saveRenameGroup() {
		const from = editingGroupName;
		const name = editGroupName.trim();
		editingGroupName = null;
		editGroupName = '';
		if (from === null || !name || name === from) return;
		const group = groups.find((g) => g.name === from);
		if (!group) return;
		const records = await listGroups();
		const record = records.find((g) => g.name === from);
		if (record) {
			if (records.some((g) => g.name === name)) await deleteGroup(record.id);
			else await putGroup({ ...record, circleIds: [...record.circleIds], name });
		}
		// Пустая новая группа серверам неизвестна — переименовать её на экране.
		const rename = { origin: '', op: 'rename', name: from, new_name: name } as const;
		groups = applyStreetOpToView(groups, rename);
		await changeStreet(groupOrigins(group).map((origin) => ({ ...rename, origin })));
	}

	async function deleteGroupByName(name: string) {
		groupMenuName = null;
		editingGroupName = null;
		const group = groups.find((g) => g.name === name);
		for (const record of await listGroups()) {
			if (record.name === name) await deleteGroup(record.id);
		}
		groups = groups.filter((g) => g.name !== name);
		await changeStreet(
			(group ? groupOrigins(group) : []).map((origin) => ({ origin, op: 'drop', name }) as const)
		);
	}

	function startCreateGroup() {
		fabMenuOpen = false;
		creatingGroup = true;
		newGroupName = '';
	}

	function startFabLongPress() {
		clearTimeout(fabLongPressTimer);
		fabLongPressTimer = setTimeout(() => {
			pressMark = markLongPress();
			fabMenuOpen = true;
			pinMenuKey = null;
			groupMenuName = null;
		}, LONG_PRESS_MS);
	}

	function endFabLongPress() {
		clearTimeout(fabLongPressTimer);
	}

	function openNew() {
		const menuOpen = fabMenuOpen;
		fabMenuOpen = false;
		if (menuOpen || swallowsClick(pressMark)) return;
		goto('/circles/new');
	}

	// Пункт меню плюса. Он нажимается как раз при открытом меню, а openNew
	// при открытом меню только закрывает его (тап по плюсу поверх меню) —
	// поэтому «Новый круг» из меню никуда не вёл.
	function openNewFromMenu() {
		fabMenuOpen = false;
		goto('/circles/new');
	}

	function doorRowActions(circle: StreetCircle) {
		const inGroup = Boolean(circleGroupName(circle));
		return {
			actionLabel: circle.pinned ? 'Открепить' : 'Закрепить',
			actionLabel2: inGroup ? 'Убрать из группы' : undefined,
			onaction2: inGroup ? () => void removeFromGroup(circle) : undefined
		};
	}

	// «У меня есть приглашение» — экран, куда вставить присланную ссылку (2.7).
	// Раньше вёл на «Без приглашения» (/join) — подпись и экран спорили.
	function openInvite() {
		goto('/invite');
	}

	function openSearch() {
		goto('/search');
	}

	function openSettings() {
		goto('/settings');
	}

	function hideNews() {
		markNewsSeen();
		newsUnseen = false;
	}

	async function hideDonateBanner() {
		if (!streetSession) return;
		await dismissPayBanner(streetSession.origin);
		payStatus = await fetchPayStatus(streetSession.origin);
	}
</script>

{#snippet plusFab()}
	<IconButton class="wh-full"
		name="plus"
		label="Новый круг"
		onclick={openNew}
		onpointerdown={startFabLongPress}
		onpointerup={endFabLongPress}
		onpointerleave={endFabLongPress}
		onpointercancel={endFabLongPress}
	/>
{/snippet}

<svelte:window onpointerdown={onWindowPointerDown} />

<ShellLayout
	app
	searchDisabled={empty}
	onsearch={empty ? undefined : openSearch}
	onsettings={openSettings}
	fab={empty ? undefined : plusFab}
	fabMenuOpen={fabMenuOpen}
	onfabmenuclose={() => (fabMenuOpen = false)}
	fabMenuItems={[
		{ label: 'Новый круг', onclick: openNewFromMenu },
		{ label: 'Новая группа', onclick: startCreateGroup },
		// Круги уже есть, а ссылка пришла в мессенджер и открылась в браузере —
		// вставить её можно здесь, как с пустой улочки (2.7).
		{ label: 'Присоединиться', onclick: () => ((fabMenuOpen = false), goto('/invite')) }
	]}
>
	<div
		class="minh-full"
		role="presentation"
		bind:this={listEl}
		ontouchstart={ptr.start}
		ontouchmove={ptr.move}
		ontouchend={ptr.end}
	>
	<PullRefreshBand pull={ptr.state} />

	{#if loading}
		<Loading />
	{:else}
		{#if showDonateBanner && payStatus?.banner}
			<PayStreetBanner
				variant="donate"
				text={payStatus.banner.text}
				dismissible={payStatus.banner.dismissible}
				onclick={() => goto('/pay/help')}
				ondismiss={() => void hideDonateBanner()}
			/>
		{/if}
		{#if showReminder && payStatus?.expires_at}
			<PayStreetBanner
				variant="reminder"
				expiresAtLabel={formatPayDate(payStatus.expires_at)}
				reminderDaysLeft={payStatus.reminder_days_left}
				onclick={() => goto('/pay/extend')}
			/>
		{/if}
		{#if showPendingNotice && payStatus?.pending_at}
			<PayStreetBanner
				variant="pending"
				pendingAtLabel={formatPayDate(payStatus.pending_at)}
				expiresAtLabel={payStatus.expires_at ? formatPayDate(payStatus.expires_at) : null}
			/>
		{/if}
		{#if newsUnseen && !empty}
			<NewsBanner
				version={latestNewsVersion()}
				onclick={() => goto('/settings/news?from=circles')}
				ondismiss={hideNews}
			/>
		{/if}
		{#if empty}
		<EmptyState title="Ни одного круга" place="list">
			Круг — это место, куда сворачивают. Заведите свой или откройте присланную ссылку.
			{#snippet actions()}
				<Button onclick={openNew}>Новый круг</Button>
				<Button variant="ghost" onclick={openInvite}>У меня есть приглашение</Button>
			{/snippet}
		</EmptyState>
		<!-- Пока кругов нет, объяснить некому: людей, чьим примером всё понятно,
		     ещё нет. Тогда оболочка коротко рассказывает обстановку. С первым
		     кругом этот рассказ уходит — дальше объясняют люди. -->
		<SectionLabel class="mt-24">Как устроен Wynd</SectionLabel>
		{#each streetFacts as fact (fact.title)}
			<SettingsRow title={fact.title} subtitle={fact.text} chevron={false} />
		{/each}
		{#if streetSession}
			<Hint class="mt-34" centered>
				Вы вошли как {streetSession.email}<br />в «{streetSession.name}» · {displayHost(
					streetSession.origin
				)}
			</Hint>
		{/if}
		{:else}
		{#if pinned.length}
			<SectionLabel>Закреплённые</SectionLabel>
			{#each pinned as circle (circleKey(circle))}
				{@const door = doorRowActions(circle)}
				<CircleRow
					initial={circle.initial}
					name={circle.name}
					preview={circle.preview}
					silent={silentHost(circle)}
					time={circle.time}
					badge={circle.unread || undefined}
					dot={circle.responses}
					color={circle.color}
					card={pinMenuKey === menuKey('pin', circle)}
					actionLabel={door.actionLabel}
					actionLabel2={door.actionLabel2}
					onaction={() => void confirmPin(circle)}
					onaction2={door.onaction2}
					onclick={() => openCircle(circle)}
					onmousedown={() => startLongPress(circle, 'pin')}
					onmouseup={cancelLongPress}
					onmouseleave={cancelLongPress}
					ontouchstart={() => startLongPress(circle, 'pin')}
					ontouchend={cancelLongPress}
					ontouchcancel={cancelLongPress}
				/>
			{/each}
			{#if groups.length || rest.length}
				<div class="sep"></div>
			{/if}
		{/if}
		{#each groups as group (group.name)}
			{#if editingGroupName === group.name}
				<Input class="m-8-16-0"
					bind:value={editGroupName}
					active
					placeholder="Название"
					onkeydown={(e) => {
						if (e.key === 'Enter') void saveRenameGroup();
						if (e.key === 'Escape') {
							editingGroupName = null;
							editGroupName = '';
						}
					}}
					onblur={() => void saveRenameGroup()}
				/>
			{:else if groupMenuName === group.name}
				<GroupFoldCard
					label={group.name}
					count={groupMemberCount(group)}
					expanded={!group.collapsed}
					foldStyle="padding-top:12px"
					onpointerdown={(e) => onGroupPointerDown(group.name, e)}
					onpointerup={() => onGroupPointerUp(group)}
					onpointercancel={cancelGroupLongPress}
					actionLabel="Переименовать"
					onaction={() => startRenameGroup(group)}
					actionLabel2="Удалить группу"
					onaction2={() => void deleteGroupByName(group.name)}
				/>
			{:else}
				<FoldHeader
					label={group.name}
					count={groupMemberCount(group)}
					expanded={!group.collapsed}
					onpointerdown={(e) => onGroupPointerDown(group.name, e)}
					onpointerup={() => onGroupPointerUp(group)}
					onpointercancel={cancelGroupLongPress}
				/>
			{/if}
			{#if !group.collapsed && groupMenuName !== group.name}
				{#each circlesInGroup(group) as circle (circleKey(circle))}
					{@const door = doorRowActions(circle)}
					<CircleRow
						initial={circle.initial}
						name={circle.name}
						preview={circle.preview}
						silent={silentHost(circle)}
						time={circle.time}
						badge={circle.unread || undefined}
					dot={circle.responses}
						color={circle.color}
						card={pinMenuKey === menuKey(`g:${group.name}`, circle)}
						actionLabel={door.actionLabel}
						actionLabel2={door.actionLabel2}
						onaction={() => void confirmPin(circle)}
						onaction2={door.onaction2}
						onclick={() => openCircle(circle)}
						onmousedown={() => startLongPress(circle, `g:${group.name}`)}
						onmouseup={cancelLongPress}
						onmouseleave={cancelLongPress}
						ontouchstart={() => startLongPress(circle, `g:${group.name}`)}
						ontouchend={cancelLongPress}
						ontouchcancel={cancelLongPress}
					/>
				{/each}
				{#if showGroupChips(group)}
					<ChipGroup class="m-10-16-0 p-0">
						{#each addableCirclesForGroup(group) as circle (circleKey(circle))}
							<Chip onclick={() => void addCircleToGroup(circle, group.name)}>{circle.name}</Chip>
						{/each}
					</ChipGroup>
				{:else if groupMemberCount(group) > 0 && addableCirclesForGroup(group).length}
					<TextButton class="m-8-16-0"
						variant="link"
						onclick={() => {
							groupAddOpenName = group.name;
						}}
					>
						положить ещё
					</TextButton>
				{/if}
			{/if}
		{/each}
		{#if creatingGroup}
			<Input class="m-8-16-0"
				bind:value={newGroupName}
				active
				placeholder="Название"
				onkeydown={(e) => {
					if (e.key === 'Enter') void createGroup();
					if (e.key === 'Escape') {
						creatingGroup = false;
						newGroupName = '';
					}
				}}
				onblur={() => void createGroup()}
			/>
		{/if}
		{#if rest.length}
			{#if pinned.length || groups.length}
				<SectionLabel>Остальные</SectionLabel>
			{/if}
			{#each rest as circle (circleKey(circle))}
				{@const door = doorRowActions(circle)}
				<CircleRow
					initial={circle.initial}
					name={circle.name}
					preview={circle.preview}
					silent={silentHost(circle)}
					time={circle.time}
					badge={circle.unread || undefined}
					dot={circle.responses}
					color={circle.color}
					card={pinMenuKey === menuKey('rest', circle)}
					actionLabel={door.actionLabel}
					actionLabel2={door.actionLabel2}
					onaction={() => void confirmPin(circle)}
					onaction2={door.onaction2}
					onclick={() => openCircle(circle)}
					onmousedown={() => startLongPress(circle, 'rest')}
					onmouseup={cancelLongPress}
					onmouseleave={cancelLongPress}
					ontouchstart={() => startLongPress(circle, 'rest')}
					ontouchend={cancelLongPress}
					ontouchcancel={cancelLongPress}
				/>
			{/each}
		{/if}
		{/if}
	{/if}
	</div>
</ShellLayout>

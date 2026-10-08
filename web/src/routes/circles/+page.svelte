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
	import { loadStreetCircles, type StreetCircle } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import {
		LONG_PRESS_MS,
		markLongPress,
		swallowsClick,
		type PressMark
	} from '$lib/gestures/longpress';
	import { displayHost } from '$lib/auth/origin';
	import {
		deleteGroup,
		deletePin,
		getPin,
		listGroups,
		putGroup,
		putPin,
		type GroupRecord
	} from '$lib/idb/db';
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
	let groups = $state<GroupRecord[]>([]);
	let sessions = $state<SessionRecord[]>([]);
	let payStatus = $state<PayStatus | undefined>();
	let loading = $state(true);
	// «Что нового?» — пока свежую запись не открыли и не закрыли (7.10).
	let newsUnseen = $state(hasUnseenNews());
	// Отметка последнего длинного нажатия: клик в её хвосте — часть того же
	// жеста (GUI-6). Прежний общий флаг гасили в каждом обработчике, и
	// порядок pointerup/click решал исход.
	let pressMark = $state<PressMark>(null);
	let pinMenuKey = $state<string | null>(null);
	let creatingGroup = $state(false);
	let newGroupName = $state('');
	let groupMenuId = $state<string | null>(null);
	let editingGroupId = $state<string | null>(null);
	let editGroupName = $state('');
	let groupAddOpenId = $state<string | null>(null);
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

	async function refresh() {
		circles = await loadStreetCircles();
		groups = await listGroups();
		// Новичку без кругов рассказывать об изменениях не о чем: для него
		// новое всё. Отметка ставится молча, баннер встанет со следующей записью.
		if (newsUnseen && circles.length === 0) hideNews();
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

	const circleMap = $derived(new Map(circles.map((c) => [circleKey(c), c])));
	const groupedKeys = $derived(new Set(groups.flatMap((g) => g.circleIds)));
	const pinned = $derived(circles.filter((c) => c.pinned));
	const rest = $derived(circles.filter((c) => !c.pinned && !groupedKeys.has(circleKey(c))));
	const empty = $derived(!loading && circles.length === 0);
	const streetSession = $derived(sessions[0]);

	// Рассказ пустой улочки (2.3): что такое Wynd и чем он не соцсеть.
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
	const showGroupHint = $derived(groups.length > 0);
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

	function circlesInGroup(group: GroupRecord): StreetCircle[] {
		return group.circleIds
			.map((id) => circleMap.get(id))
			// Закреплённый круг остаётся в своей группе: закрепление — ярлык
			// сверху, а не переезд. Раньше он пропадал из папки, и группа
			// показывала счётчик при пустом содержимом.
			.filter((c): c is StreetCircle => c !== undefined);
	}

	function groupMemberCount(group: GroupRecord): number {
		return group.circleIds.filter((id) => circleMap.has(id)).length;
	}

	function circleGroupId(circle: StreetCircle): string | undefined {
		const key = circleKey(circle);
		return groups.find((g) => g.circleIds.includes(key))?.id;
	}

	function addableCirclesForGroup(group: GroupRecord): StreetCircle[] {
		return circles.filter((c) => !group.circleIds.includes(circleKey(c)));
	}

	function showGroupChips(group: GroupRecord): boolean {
		if (!addableCirclesForGroup(group).length) return false;
		return groupMemberCount(group) === 0 || groupAddOpenId === group.id;
	}

	function openCircle(circle: StreetCircle) {
		// Нажатие при открытом меню закрывает его и на этом останавливается —
		// иначе тем же тапом человек проваливался бы в круг.
		const menuOpen = pinMenuKey !== null || groupMenuId !== null || fabMenuOpen;
		pinMenuKey = null;
		groupMenuId = null;
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
		if ((pinMenuKey !== null || groupMenuId !== null) && !target.closest('.circle-row-card')) {
			pinMenuKey = null;
			groupMenuId = null;
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

	async function confirmPin(circle: StreetCircle) {
		const pin = await getPin(circle.origin, circle.id);
		if (pin) await deletePin(circle.origin, circle.id);
		else await putPin(circle.origin, circle.id);
		pinMenuKey = null;
		await refresh();
	}

	let groupDownAt = 0;
	let groupMenuOpenedAt = 0;

	function plainGroup(group: GroupRecord, patch: Partial<GroupRecord> = {}): GroupRecord {
		return {
			id: group.id,
			name: patch.name ?? group.name,
			circleIds: [...(patch.circleIds ?? group.circleIds)],
			collapsed: patch.collapsed ?? group.collapsed
		};
	}

	function onGroupPointerDown(groupId: string, e: PointerEvent) {
		if (e.pointerType === 'mouse' && e.button !== 0) return;
		groupDownAt = Date.now();
		startGroupLongPress(groupId);
	}

	function onGroupPointerUp(group: GroupRecord) {
		const short = Date.now() - groupDownAt < LONG_PRESS_MS;
		const openedByThisHold = groupMenuOpenedAt >= groupDownAt && groupMenuId === group.id;
		cancelGroupLongPress();
		if (!short || openedByThisHold || swallowsClick(pressMark)) return;
		groupMenuId = null;
		void setGroupCollapsed(group, !group.collapsed);
	}

	async function setGroupCollapsed(group: GroupRecord, collapsed: boolean) {
		const next = plainGroup(group, { collapsed });
		groups = groups.map((item) => (item.id === group.id ? next : item));
		await putGroup(next);
	}

	async function addCircleToGroup(circle: StreetCircle, groupId: string) {
		const key = circleKey(circle);
		for (const group of groups) {
			if (group.id === groupId) {
				if (!group.circleIds.includes(key)) {
					await putGroup(plainGroup(group, { circleIds: [...group.circleIds, key] }));
				}
			} else if (group.circleIds.includes(key)) {
				await putGroup(
					plainGroup(group, { circleIds: group.circleIds.filter((id) => id !== key) })
				);
			}
		}
		await refresh();
		const group = groups.find((g) => g.id === groupId);
		if (group && addableCirclesForGroup(group).length === 0) {
			groupAddOpenId = null;
		}
	}

	async function removeFromGroup(circle: StreetCircle) {
		const key = circleKey(circle);
		const groupId = circleGroupId(circle);
		if (!groupId) return;
		const group = groups.find((g) => g.id === groupId);
		if (!group) return;
		await putGroup(plainGroup(group, { circleIds: group.circleIds.filter((id) => id !== key) }));
		pinMenuKey = null;
		await refresh();
	}

	async function createGroup() {
		const name = newGroupName.trim();
		creatingGroup = false;
		newGroupName = '';
		fabMenuOpen = false;
		if (!name) return;
		await putGroup({
			id: uuid(),
			name,
			circleIds: [],
			collapsed: false
		});
		groups = await listGroups();
	}

	function startGroupLongPress(groupId: string) {
		clearTimeout(groupLongPressTimer);
		groupLongPressTimer = setTimeout(() => {
			pressMark = markLongPress();
			groupMenuId = groupId;
			groupMenuOpenedAt = Date.now();
			editingGroupId = null;
		}, LONG_PRESS_MS);
	}

	function cancelGroupLongPress() {
		clearTimeout(groupLongPressTimer);
	}

	function startRenameGroup(group: GroupRecord) {
		editingGroupId = group.id;
		editGroupName = group.name;
		groupMenuId = null;
	}

	async function saveRenameGroup() {
		const id = editingGroupId;
		const name = editGroupName.trim();
		editingGroupId = null;
		editGroupName = '';
		if (!id || !name) return;
		const group = groups.find((g) => g.id === id);
		if (!group) return;
		await putGroup(plainGroup(group, { name }));
		groups = await listGroups();
	}

	async function deleteGroupById(id: string) {
		groupMenuId = null;
		editingGroupId = null;
		await deleteGroup(id);
		groups = await listGroups();
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
			groupMenuId = null;
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
		const inGroup = Boolean(circleGroupId(circle));
		return {
			actionLabel: circle.pinned ? 'Открепить' : 'Закрепить',
			actionLabel2: inGroup ? 'Убрать из группы' : undefined,
			onaction2: inGroup ? () => void removeFromGroup(circle) : undefined
		};
	}

	// «У меня есть приглашение» — экран, куда вставить присланную ссылку (2.16).
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
		// вставить её можно здесь, как с пустой улочки (2.16).
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
		{#each groups as group (group.id)}
			{#if editingGroupId === group.id}
				<Input class="m-8-16-0"
					bind:value={editGroupName}
					active
					placeholder="Название"
					onkeydown={(e) => {
						if (e.key === 'Enter') void saveRenameGroup();
						if (e.key === 'Escape') {
							editingGroupId = null;
							editGroupName = '';
						}
					}}
					onblur={() => void saveRenameGroup()}
				/>
			{:else if groupMenuId === group.id}
				<GroupFoldCard
					label={group.name}
					count={groupMemberCount(group)}
					expanded={!group.collapsed}
					foldStyle="padding-top:12px"
					onpointerdown={(e) => onGroupPointerDown(group.id, e)}
					onpointerup={() => onGroupPointerUp(group)}
					onpointercancel={cancelGroupLongPress}
					actionLabel="Переименовать"
					onaction={() => startRenameGroup(group)}
					actionLabel2="Удалить группу"
					onaction2={() => void deleteGroupById(group.id)}
				/>
			{:else}
				<FoldHeader
					label={group.name}
					count={groupMemberCount(group)}
					expanded={!group.collapsed}
					onpointerdown={(e) => onGroupPointerDown(group.id, e)}
					onpointerup={() => onGroupPointerUp(group)}
					onpointercancel={cancelGroupLongPress}
				/>
			{/if}
			{#if !group.collapsed && groupMenuId !== group.id}
				{#each circlesInGroup(group) as circle (circleKey(circle))}
					{@const door = doorRowActions(circle)}
					<CircleRow
						initial={circle.initial}
						name={circle.name}
						preview={circle.preview}
						time={circle.time}
						badge={circle.unread || undefined}
					dot={circle.responses}
						color={circle.color}
						card={pinMenuKey === menuKey(`g:${group.id}`, circle)}
						actionLabel={door.actionLabel}
						actionLabel2={door.actionLabel2}
						onaction={() => void confirmPin(circle)}
						onaction2={door.onaction2}
						onclick={() => openCircle(circle)}
						onmousedown={() => startLongPress(circle, `g:${group.id}`)}
						onmouseup={cancelLongPress}
						onmouseleave={cancelLongPress}
						ontouchstart={() => startLongPress(circle, `g:${group.id}`)}
						ontouchend={cancelLongPress}
						ontouchcancel={cancelLongPress}
					/>
				{/each}
				{#if showGroupChips(group)}
					<ChipGroup class="m-10-16-0 p-0">
						{#each addableCirclesForGroup(group) as circle (circleKey(circle))}
							<Chip onclick={() => void addCircleToGroup(circle, group.id)}>{circle.name}</Chip>
						{/each}
					</ChipGroup>
				{:else if groupMemberCount(group) > 0 && addableCirclesForGroup(group).length}
					<TextButton class="m-8-16-0"
						variant="link"
						onclick={() => {
							groupAddOpenId = group.id;
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
		{#if showGroupHint}
			<Hint centered class="mt-26">группы видны только на этом устройстве</Hint>
		{/if}
		{/if}
	{/if}
	</div>
</ShellLayout>

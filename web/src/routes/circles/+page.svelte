<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import CircleRow, { type CircleRowGroupChip } from '$ui/data/CircleRow.svelte';
	import FoldHeader from '$ui/data/FoldHeader.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import Input from '$ui/forms/Input.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import { loadStreetCircles, type StreetCircle } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { displayHost } from '$lib/auth/origin';
	import {
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
		pluralDays,
		type PayStatus
	} from '$lib/pay/pay';
	import type { SessionRecord } from '$lib/idb/db';

	let circles = $state<StreetCircle[]>([]);
	let groups = $state<GroupRecord[]>([]);
	let sessions = $state<SessionRecord[]>([]);
	let payStatus = $state<PayStatus | undefined>();
	let loading = $state(true);
	let suppressClick = $state(false);
	let pinMenuKey = $state<string | null>(null);
	let creatingGroup = $state(false);
	let newGroupName = $state('');
	let longPressTimer: ReturnType<typeof setTimeout> | undefined;

	async function refresh() {
		circles = await loadStreetCircles();
		groups = await listGroups();
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
	const showGroupHint = $derived(groups.length > 0);
	const showDonateBanner = $derived(
		payStatus?.banner && !payStatus.dismissed && payStatus.has_requisites
	);
	const showReminder = $derived(
		payStatus?.reminder && payStatus.expires_at && payStatus.has_requisites
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
			.filter((c): c is StreetCircle => c !== undefined && !c.pinned);
	}

	function circleGroupId(circle: StreetCircle): string | undefined {
		const key = circleKey(circle);
		return groups.find((g) => g.circleIds.includes(key))?.id;
	}

	function groupChipsFor(circle: StreetCircle): CircleRowGroupChip[] {
		const currentGroupId = circleGroupId(circle);
		return groups.map((g) => ({
			label: g.name,
			selected: g.id === currentGroupId,
			onclick: () => void assignToGroup(circle, g.id)
		}));
	}

	function openCircle(circle: StreetCircle) {
		pinMenuKey = null;
		if (suppressClick) {
			suppressClick = false;
			return;
		}
		rememberCircleOrigin(circle.id, circle.origin);
		goto(`/circles/${circle.id}`);
	}

	function startLongPress(circle: StreetCircle) {
		clearTimeout(longPressTimer);
		longPressTimer = setTimeout(() => {
			suppressClick = true;
			pinMenuKey = circleKey(circle);
		}, 500);
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

	async function toggleGroupCollapsed(group: GroupRecord) {
		await putGroup({ ...group, collapsed: !group.collapsed });
		groups = await listGroups();
	}

	async function assignToGroup(circle: StreetCircle, groupId: string) {
		const key = circleKey(circle);
		for (const group of groups) {
			const inGroup = group.circleIds.includes(key);
			if (group.id === groupId) {
				if (inGroup) {
					await putGroup({ ...group, circleIds: group.circleIds.filter((id) => id !== key) });
				} else {
					await putGroup({ ...group, circleIds: [...group.circleIds, key] });
				}
			} else if (inGroup) {
				await putGroup({ ...group, circleIds: group.circleIds.filter((id) => id !== key) });
			}
		}
		pinMenuKey = null;
		await refresh();
	}

	async function createGroup() {
		const name = newGroupName.trim();
		creatingGroup = false;
		newGroupName = '';
		if (!name) return;
		await putGroup({
			id: crypto.randomUUID(),
			name,
			circleIds: [],
			collapsed: false
		});
		groups = await listGroups();
	}

	function startCreateGroup() {
		creatingGroup = true;
		newGroupName = '';
	}

	function openNew() {
		goto('/circles/new');
	}

	function openInvite() {
		goto('/join');
	}

	function openSearch() {
		goto('/search');
	}

	function openSettings() {
		goto('/settings');
	}

	async function hideDonateBanner() {
		if (!streetSession) return;
		await dismissPayBanner(streetSession.origin);
		payStatus = await fetchPayStatus(streetSession.origin);
	}
</script>

<ShellLayout
	app
	searchDisabled={empty}
	onsearch={empty ? undefined : openSearch}
	onsettings={openSettings}
>
	{#if !empty}
		{#snippet fab()}
			<IconButton
				name="plus"
				label="Новый круг"
				style="width:26px;height:26px;stroke-width:1.5"
				onclick={openNew}
			/>
		{/snippet}
	{/if}

	{#if loading}
		<Hint style="margin-top:24px">Загрузка…</Hint>
	{:else if empty}
		<ScreenTitle centered style="margin-top:190px">Ни одного круга</ScreenTitle>
		<Hint centered style="margin:10px 30px 0">
			Круг — это место, куда сворачивают. Заведите свой или откройте присланную ссылку.
		</Hint>
		<Button style="margin-top:30px" onclick={openNew}>Новый круг</Button>
		<Button variant="ghost" onclick={openInvite}>У меня есть приглашение</Button>
		{#if streetSession}
			<Hint centered style="margin-top:34px">
				Вы вошли как {streetSession.email}<br />в «{streetSession.name}» · {displayHost(
					streetSession.origin
				)}
			</Hint>
		{/if}
	{:else}
		{#if showDonateBanner && payStatus?.banner}
			<div class="pay-banner">
				<button type="button" class="pay-banner-main" onclick={() => goto('/pay/help')}>
					<div style="flex:1;font-size:12.5px;line-height:1.45;text-align:left">
						{payStatus.banner.text}
					</div>
					<Icon name="chevr" size="sm" style="flex-shrink:0;color:var(--muted)" />
				</button>
				{#if payStatus.banner.dismissible}
					<IconButton
						name="x"
						label="Скрыть"
						style="flex-shrink:0"
						onclick={() => void hideDonateBanner()}
					/>
				{/if}
			</div>
		{/if}
		{#if showReminder && payStatus?.expires_at}
			<button type="button" class="pay-banner pay-reminder" onclick={() => goto('/pay/extend')}>
				<div style="flex:1;font-size:12.5px;line-height:1.45;text-align:left">
					<div style="font-weight:600">Подписка до {formatPayDate(payStatus.expires_at)}</div>
					{#if payStatus.reminder_days_left != null}
						<div style="color:var(--muted);margin-top:3px">
							Через {pluralDays(payStatus.reminder_days_left)} круги закроются.
						</div>
					{/if}
				</div>
				<Icon name="chevr" size="sm" style="flex-shrink:0;color:var(--muted)" />
			</button>
		{/if}
		{#if showPendingNotice && payStatus?.pending_at}
			<div class="pay-banner pay-pending">
				<div style="font-weight:600;font-size:12.5px">
					Заявку отправили {formatPayDate(payStatus.pending_at)}
				</div>
				<div style="font-size:12.5px;color:var(--muted);margin-top:3px;line-height:1.45">
					{#if payStatus.expires_at}
						Круги открыты до {formatPayDate(payStatus.expires_at)}. Пока администратор не ответит,
						новую заявку отправить нельзя.
					{:else}
						Пока администратор не ответит, новую заявку отправить нельзя.
					{/if}
				</div>
			</div>
		{/if}
		{#if pinned.length}
			<SectionLabel>Закреплённые</SectionLabel>
			{#each pinned as circle (circleKey(circle))}
				<CircleRow
					initial={circle.initial}
					name={circle.name}
					preview={circle.preview}
					time={circle.time}
					badge={circle.unread || undefined}
					color={circle.color}
					card={pinMenuKey === circleKey(circle)}
					actionLabel={circle.pinned ? 'Открепить' : 'Закрепить'}
					groupChips={pinMenuKey === circleKey(circle) && groups.length
						? groupChipsFor(circle)
						: undefined}
					onaction={() => void confirmPin(circle)}
					onclick={() => openCircle(circle)}
					onmousedown={() => startLongPress(circle)}
					onmouseup={cancelLongPress}
					onmouseleave={cancelLongPress}
					ontouchstart={() => startLongPress(circle)}
					ontouchend={cancelLongPress}
					ontouchcancel={cancelLongPress}
				/>
			{/each}
			{#if groups.length || rest.length}
				<div class="sep"></div>
			{/if}
		{/if}
		{#each groups as group (group.id)}
			<FoldHeader
				label={group.name}
				count={circlesInGroup(group).length}
				expanded={!group.collapsed}
				onclick={() => void toggleGroupCollapsed(group)}
			/>
			{#if !group.collapsed}
				{#each circlesInGroup(group) as circle (circleKey(circle))}
					<CircleRow
						initial={circle.initial}
						name={circle.name}
						preview={circle.preview}
						time={circle.time}
						badge={circle.unread || undefined}
						color={circle.color}
						card={pinMenuKey === circleKey(circle)}
						actionLabel={circle.pinned ? 'Открепить' : 'Закрепить'}
						groupChips={pinMenuKey === circleKey(circle) && groups.length
							? groupChipsFor(circle)
							: undefined}
						onaction={() => void confirmPin(circle)}
						onclick={() => openCircle(circle)}
						onmousedown={() => startLongPress(circle)}
						onmouseup={cancelLongPress}
						onmouseleave={cancelLongPress}
						ontouchstart={() => startLongPress(circle)}
						ontouchend={cancelLongPress}
						ontouchcancel={cancelLongPress}
					/>
				{/each}
			{/if}
		{/each}
		{#if rest.length}
			{#if pinned.length || groups.length}
				<SectionLabel>Остальные</SectionLabel>
			{/if}
			{#each rest as circle (circleKey(circle))}
				<CircleRow
					initial={circle.initial}
					name={circle.name}
					preview={circle.preview}
					time={circle.time}
					badge={circle.unread || undefined}
					color={circle.color}
					card={pinMenuKey === circleKey(circle)}
					actionLabel={circle.pinned ? 'Открепить' : 'Закрепить'}
					groupChips={pinMenuKey === circleKey(circle) && groups.length
						? groupChipsFor(circle)
						: undefined}
					onaction={() => void confirmPin(circle)}
					onclick={() => openCircle(circle)}
					onmousedown={() => startLongPress(circle)}
					onmouseup={cancelLongPress}
					onmouseleave={cancelLongPress}
					ontouchstart={() => startLongPress(circle)}
					ontouchend={cancelLongPress}
					ontouchcancel={cancelLongPress}
				/>
			{/each}
		{/if}
		{#if creatingGroup}
			<Input
				bind:value={newGroupName}
				active
				placeholder="Название"
				style="margin:8px 16px 0"
				onkeydown={(e) => {
					if (e.key === 'Enter') void createGroup();
					if (e.key === 'Escape') {
						creatingGroup = false;
						newGroupName = '';
					}
				}}
				onblur={() => void createGroup()}
			/>
		{:else}
			<TextButton variant="link" style="margin:8px 16px 0" onclick={startCreateGroup}>
				Новая группа
			</TextButton>
		{/if}
		{#if showGroupHint}
			<Hint centered style="margin-top:26px">группы видны только на этом устройстве</Hint>
		{/if}
	{/if}
</ShellLayout>

<style>
	.pay-banner {
		margin: 8px 14px 4px;
		border: 1px solid var(--line);
		background: var(--card);
		border-radius: 14px;
		padding: 12px 14px;
		display: flex;
		gap: 10px;
		align-items: flex-start;
	}
	.pay-banner-main,
	.pay-reminder {
		all: unset;
		box-sizing: border-box;
		cursor: pointer;
		display: flex;
		gap: 10px;
		align-items: flex-start;
		width: 100%;
	}
	.pay-reminder {
		align-items: center;
	}
	.pay-pending {
		display: block;
	}
</style>

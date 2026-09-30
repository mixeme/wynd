<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import ColorSwatches from '$ui/forms/ColorSwatches.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import { saveSharePlace } from '$lib/journal/place-pref';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		customHoursFromSec,
		deleteCircle,
		editWindowFromSec,
		editWindowToSec,
		fetchCircleInvites,
		fetchCircleSettings,
		fetchMembers,
		fetchQuota,
		isValidCustomHours,
		patchCircle,
		type EditWindowKey,
		type MemberInfo
	} from '$lib/circles/settings';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import { pluralPeople, pluralPosts } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { setCircleColor, circleInitial } from '$lib/circles/meta';
	import type { CircleColor } from '$lib/theme/colors';
	import { CIRCLE_COLORS } from '$lib/theme/colors';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let name = $state('');
	let savedName = $state('');
	let color = $state<CircleColor>('olive');
	let editWindow = $state<EditWindowKey>('1h');
	let customHours = $state(24);
	let canSettings = $state(false);
	let isOwner = $state(false);
	let inviteWho = $state<'all' | 'owner'>('all');
	let inviteKindDefault = $state<'single' | 'multi'>('single');
	let members = $state<MemberInfo[]>([]);
	let avatarUrls = $state<Record<string, string>>({});
	let usedBytes = $state(0);
	let quotaBytes = $state<number | undefined>();
	let loading = $state(true);
	let error = $state('');
	let colorReady = $state(false);
	let ownerLeaveOpen = $state(false);
	let deleteOpen = $state(false);
	let deleteConfirm = $state('');
	let deleteError = $state('');
	let deleteLoading = $state(false);
	let deletePostCount = $state<number | null>(null);
	let deleteUsedBytes = $state<number | null>(null);
	let liveCount = $state(0);
	let nameHint = $state('');
	const customHoursHintText = 'Укажите целое число часов от 1 до 8760';
	let customHoursHint = $state('');

	const activeMembers = $derived(members.filter((m) => m.status === 'active'));
	const previewMembers = $derived(activeMembers.slice(0, 3));
	const soloCircle = $derived(activeMembers.length === 1);

	function memberSubtitle(m: MemberInfo): string {
		if (m.identity_id === circle.identityId) {
			return 'это вы';
		}
		const parts: string[] = [];
		if (m.is_owner) parts.push('владелец');
		else if (m.can_settings) parts.push('может менять настройки');
		return parts.join(' · ');
	}

	function memberColor(m: MemberInfo, index: number): string {
		const palette = Object.values(CIRCLE_COLORS);
		return palette[index % palette.length].cssVar;
	}

	function memberSrc(m: MemberInfo): string | undefined {
		if (m.identity_id === circle.identityId && circle.avatarUrl) return circle.avatarUrl;
		return avatarUrls[m.identity_id];
	}

	async function loadAvatars(list: MemberInfo[]) {
		const byBlob = new Map<string, string[]>();
		for (const m of list) {
			if (!m.avatar_blob_id || avatarUrls[m.identity_id]) continue;
			if (m.identity_id === circle.identityId && circle.avatarUrl) continue;
			byBlob.set(m.avatar_blob_id, [...(byBlob.get(m.avatar_blob_id) ?? []), m.identity_id]);
		}
		if (byBlob.size === 0) return;
		const next = { ...avatarUrls };
		await resolveMediaUrls(circle.origin, [...byBlob.keys()], (blobId, url) => {
			for (const identityId of byBlob.get(blobId) ?? []) next[identityId] = url;
			avatarUrls = { ...next };
		});
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [settings, list] = await Promise.all([
				fetchCircleSettings(circle.origin, circle.circleId),
				fetchMembers(circle.origin, circle.circleId)
			]);
			name = settings.name;
			savedName = settings.name;
			color = circle.color;
			editWindow = editWindowFromSec(settings.edit_window_sec);
			customHoursHint = '';
			if (editWindow === 'custom' && settings.edit_window_sec != null) {
				const parsed = customHoursFromSec(settings.edit_window_sec);
				if (parsed === null) {
					customHours = Math.round(settings.edit_window_sec / 3600);
					customHoursHint = customHoursHintText;
				} else {
					customHours = parsed;
				}
			}
			canSettings = settings.can_settings ?? false;
			isOwner = settings.is_owner ?? false;
			inviteWho = settings.invite_who ?? 'all';
			inviteKindDefault = settings.invite_kind_default ?? 'single';
			members = list;
			try {
				await loadAvatars(list.filter((m) => m.status === 'active').slice(0, 3));
			} catch {
				/* фото не обязательны: остаётся буква */
			}
			liveCount = 0;
			if (inviteWho === 'all' || isOwner) {
				try {
					liveCount = (await fetchCircleInvites(circle.origin, circle.circleId)).length;
				} catch {
					/* list is optional on this screen */
				}
			}
			if (isOwner) {
				try {
					const quota = await fetchQuota(circle.origin, circle.circleId);
					usedBytes = quota.used_bytes;
					quotaBytes = quota.quota_bytes;
				} catch {
					/* non-owner race */
				}
			}
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
			colorReady = true;
		}
	}

	$effect(() => {
		if (!colorReady) return;
		void onColorChange(color);
	});

	async function saveName() {
		if (!canSettings) return;
		const trimmed = name.trim();
		if (!trimmed) {
			nameHint = 'Укажите название';
			return;
		}
		nameHint = '';
		try {
			await patchCircle(circle.origin, circle.circleId, { name: trimmed });
			name = trimmed;
			savedName = trimmed;
			circle.name = trimmed;
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onColorChange(c: CircleColor) {
		color = c;
		circle.color = c;
		circle.colorHex = CIRCLE_COLORS[c].cssVar;
		await setCircleColor(circle.origin, circle.circleId, c);
		if (canSettings) {
			try {
				await patchCircle(circle.origin, circle.circleId, { color: c });
			} catch (err) {
				error = authErrorHint(err);
			}
		}
	}

	async function onEditWindow(key: EditWindowKey) {
		if (!canSettings) return;
		editWindow = key;
		if (key === 'custom' && !isValidCustomHours(customHours)) {
			customHoursHint = customHoursHintText;
			return;
		}
		customHoursHint = '';
		try {
			await patchCircle(circle.origin, circle.circleId, {
				edit_window_sec: editWindowToSec(key, customHours)
			});
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onCustomHoursChange() {
		if (!canSettings || editWindow !== 'custom') return;
		if (!isValidCustomHours(customHours)) {
			customHoursHint = customHoursHintText;
			return;
		}
		customHoursHint = '';
		try {
			await patchCircle(circle.origin, circle.circleId, {
				edit_window_sec: editWindowToSec('custom', customHours)
			});
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onInviteWho(who: 'all' | 'owner') {
		if (!canSettings) return;
		inviteWho = who;
		try {
			await patchCircle(circle.origin, circle.circleId, { invite_who: who });
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onInviteKindDefault(kind: 'single' | 'multi') {
		if (!canSettings) return;
		inviteKindDefault = kind;
		try {
			await patchCircle(circle.origin, circle.circleId, { invite_kind_default: kind });
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	const canInvite = $derived(inviteWho === 'all' || isOwner);

	const deleteImpactHint = $derived.by(() => {
		if (deletePostCount === null || deleteUsedBytes === null) return '';
		return `${pluralPosts(deletePostCount)}, ${formatBytes(deleteUsedBytes)} фотографий и весь журнал ${pluralPeople(activeMembers.length)} исчезнут с сервера. Восстановить будет нечем.`;
	});

	function clearDeleteUrlParam() {
		const url = new URL($page.url);
		if (!url.searchParams.has('delete')) return;
		url.searchParams.delete('delete');
		const next = url.search ? `${url.pathname}${url.search}` : url.pathname;
		goto(next, { replaceState: true });
	}

	function closeDeleteDialog() {
		deleteOpen = false;
		deleteConfirm = '';
		deleteError = '';
		clearDeleteUrlParam();
	}

	async function openDeleteDialog() {
		if (!isOwner) return;
		deleteConfirm = '';
		deleteError = '';
		deleteOpen = true;
		if (deletePostCount !== null && deleteUsedBytes !== null) return;
		try {
			const quota = await fetchQuota(circle.origin, circle.circleId);
			deletePostCount = quota.post_count;
			deleteUsedBytes = quota.used_bytes;
		} catch (err) {
			deleteError = authErrorHint(err);
		}
	}

	async function onDeleteCircle() {
		if (deleteConfirm !== savedName.trim()) {
			deleteError = 'Введите название круга точно';
			return;
		}
		deleteLoading = true;
		deleteError = '';
		try {
			await deleteCircle(circle.origin, circle.circleId, deleteConfirm);
			goto('/circles');
		} catch (err) {
			deleteError = authErrorHint(err);
			deleteLoading = false;
		}
	}

	function goBack() {
		goto(`/circles/${circle.circleId}`);
	}

	async function onLeave() {
		if (isOwner) {
			ownerLeaveOpen = true;
			return;
		}
		goto(`/circles/${circle.circleId}/settings/leave`);
	}

	function onDangerItem(label: string) {
		if (label === 'Передать владение') {
			goto(`/circles/${circle.circleId}/settings/members?transfer=1`);
		} else if (label === 'Покинуть круг') {
			void onLeave();
		} else if (label === 'Удалить круг и все записи') {
			void openDeleteDialog();
		}
	}

	$effect(() => {
		if (loading || !isOwner || deleteOpen) return;
		if ($page.url.searchParams.get('delete') !== '1') return;
		void openDeleteDialog();
	});

	// Место со снимков (B3) — личная настройка в круге, хранится на сервере
	// и приходит на все устройства. Пишется по нажатию; отказ — положение
	// возвращается, причина в строке ошибки.
	async function savePlace(on: boolean) {
		try {
			circle.sharePlace = await saveSharePlace(circle.origin, circle.circleId, on);
		} catch (err) {
			circle.sharePlace = !on;
			error = authErrorHint(err);
		}
	}

	onMount(() => {
		void load();
	});
</script>

<FormLayout app color={circle.color} title="Настройки круга" onback={goBack}>
	{#if loading}
		<Hint style="margin:16px">Загрузка…</Hint>
	{:else if error}
		<Hint style="margin:16px">{error}</Hint>
	{:else}
		<Label>Название</Label>
		{#if canSettings}
			<Input active bind:value={name} onchange={() => void saveName()} />
			{#if nameHint}
				<Hint style="margin-top:8px">{nameHint}</Hint>
			{/if}
		{:else}
			<FieldDisplay value={name} />
		{/if}

		<Label>Цвет</Label>
		<ColorSwatches bind:value={color} />

		{#if canSettings}
			<Label>Окно правок</Label>
			<ChipGroup>
				<Chip selected={editWindow === 'chronicle'} onclick={() => void onEditWindow('chronicle')}>
					Летопись
				</Chip>
				<Chip selected={editWindow === '10m'} onclick={() => void onEditWindow('10m')}>10 мин</Chip>
				<Chip selected={editWindow === '1h'} onclick={() => void onEditWindow('1h')}>Час</Chip>
				<Chip selected={editWindow === '1d'} onclick={() => void onEditWindow('1d')}>Сутки</Chip>
			</ChipGroup>
			<ChipGroup style="margin-top:8px">
				<Chip selected={editWindow === 'unlimited'} onclick={() => void onEditWindow('unlimited')}>
					Без ограничения
				</Chip>
				<Chip selected={editWindow === 'custom'} onclick={() => void onEditWindow('custom')}>Своё…</Chip>
			</ChipGroup>
			{#if editWindow === 'custom'}
				<div class="rowin" style="margin-top:10px;align-items:center">
					<Input
						active
						type="number"
						min="1"
						max="8760"
						bind:value={customHours}
						onchange={() => void onCustomHoursChange()}
						style="width:72px;margin:0"
					/>
					<span class="hint" style="margin:0">часов</span>
				</div>
				{#if customHoursHint}
					<Hint style="margin-top:8px">{customHoursHint}</Hint>
				{/if}
			{/if}
			<Hint
				>Сколько времени после публикации запись можно править. Новое правило подействует только на новые записи.</Hint
			>

			<Label>Приглашения</Label>
			{#if !soloCircle}
			<ChipGroup>
				<Chip selected={inviteWho === 'all'} onclick={() => void onInviteWho('all')}>Могут все</Chip>
				<Chip selected={inviteWho === 'owner'} onclick={() => void onInviteWho('owner')}>
					Только владелец
				</Chip>
			</ChipGroup>
			{/if}
			<ChipGroup style={soloCircle ? undefined : 'margin-top:8px'}>
				<Chip
					selected={inviteKindDefault === 'single'}
					onclick={() => void onInviteKindDefault('single')}
				>
					Одноразовые
				</Chip>
				<Chip
					selected={inviteKindDefault === 'multi'}
					onclick={() => void onInviteKindDefault('multi')}
				>
					Многоразовые
				</Chip>
			</ChipGroup>
			<Hint
				>Какие ссылки смогут создавать участники. По многоразовой войдут несколько человек.</Hint
			>
		{/if}

		{#if canInvite}
			<SettingsRow
				title="Пригласить"
				style="margin-top:14px;border-top:1px solid var(--line)"
				onclick={() => goto(`/circles/${circle.circleId}/settings/invite`)}
			/>
			<SettingsRow
				title="Живые ссылки"
				value={liveCount > 0 ? String(liveCount) : undefined}
				onclick={() => goto(`/circles/${circle.circleId}/settings/invites`)}
			/>
		{/if}
		<SettingsRow
			title="Кто вы в этом круге"
			value={circle.identityName}
			onclick={() => goto(`/circles/${circle.circleId}/settings/identity`)}
		/>
		<SettingsRow
			title="Уведомления"
			subtitle={soloCircle ? undefined : 'записи и упоминания'}
			onclick={() => goto(`/circles/${circle.circleId}/settings/notify`)}
		/>
		<SettingsRow title="Место со снимков" subtitle="точка на карте круга">
			{#snippet control()}
				<Switch
					checked={circle.sharePlace}
					label="Место со снимков"
					onchange={(on) => void savePlace(on)}
				/>
			{/snippet}
		</SettingsRow>

		{#if isOwner && quotaBytes}
			<Label style="margin-top:20px">Место</Label>
			<Meter value={usedBytes / (1024 * 1024 * 1024)} max={quotaBytes / (1024 * 1024 * 1024)} />
			<Hint style="margin-top:8px"
				>{formatBytes(usedBytes)} из {formatBytes(quotaBytes)} · квоту задал администратор</Hint
			>
			<SettingsRow
				title="Архив и очистка"
				subtitle="освободить место, скачать архив"
				style="margin-top:8px"
				onclick={() => goto(`/circles/${circle.circleId}/quota`)}
			/>
		{/if}
		{#if circle.archiveCycle?.active}
			{#if isOwner}
				<SettingsRow
					title="Сроки архивации"
					subtitle="отсечка и дедлайн"
					style="margin-top:20px"
					onclick={() => goto(`/circles/${circle.circleId}/quota/deadlines`)}
				/>
			{/if}
			<SettingsRow
				title="Скачать архив"
				subtitle="персональная копия до отсечки"
				style={isOwner ? 'margin-top:8px' : 'margin-top:20px'}
				onclick={() => goto(`/circles/${circle.circleId}/archive`)}
			/>
		{/if}

		<Label style="margin-top:20px">Участники · {activeMembers.length}</Label>
		{#each previewMembers as m, i (m.identity_id)}
			<MemberRow
				initial={circleInitial(m.name)}
				name={m.name}
				subtitle={memberSubtitle(m)}
				color={memberColor(m, i)}
				src={memberSrc(m)}
				style={i === 0 ? 'padding-top:2px' : undefined}
			/>
		{/each}
		<SettingsRow
			title={activeMembers.length > 3 ? `ещё ${activeMembers.length - 3}` : 'Все участники'}
			link
			onclick={() => goto(`/circles/${circle.circleId}/settings/members`)}
		/>

		{#if !soloCircle || isOwner}
		<div style="margin-top:20px">
			<DangerZone
				items={[
					...(!soloCircle && isOwner ? ['Передать владение'] : []),
					...(!soloCircle ? ['Покинуть круг'] : []),
					...(isOwner ? ['Удалить круг и все записи'] : [])
				]}
				onitem={onDangerItem}
			/>
		</div>
		{/if}
	{/if}
</FormLayout>

{#if ownerLeaveOpen}
	<OverlayLayout variant="dialog" label="Сначала передайте владение" ondismiss={() => (ownerLeaveOpen = false)}>
		<div style="font-size:17px;font-weight:600;margin-bottom:10px">Сначала передайте владение</div>
		<Hint
			>Подвешенных кругов не бывает. Пока вы владелец «{circle.name}», уйти нельзя.</Hint
		>
		<div class="rowin" style="margin:18px 0 0">
			<Button variant="ghost" style="flex:1;margin:0" onclick={() => (ownerLeaveOpen = false)}>
				Отмена
			</Button>
			<Button
				style="flex:1;margin:0"
				onclick={() => {
					ownerLeaveOpen = false;
					goto(`/circles/${circle.circleId}/settings/members?transfer=1`);
				}}
			>
				Передать
			</Button>
		</div>
	</OverlayLayout>
{/if}

{#if deleteOpen}
	<OverlayLayout variant="dialog" label="Удалить «{savedName}»?" ondismiss={closeDeleteDialog}>
		<div style="font-size:17px;font-weight:600;margin-bottom:10px">Удалить «{savedName}»?</div>
		{#if deleteImpactHint}
			<Hint style="margin-bottom:14px">{deleteImpactHint}</Hint>
		{/if}
		<Label style="margin:0 0 7px">Напишите имя круга</Label>
		<Input active bind:value={deleteConfirm} />
		{#if deleteError}
			<Hint style="margin-top:8px">{deleteError}</Hint>
		{/if}
		<div class="rowin" style="margin:18px 0 0">
			<Button variant="ghost" style="flex:1;margin:0" onclick={closeDeleteDialog}>Отмена</Button>
			<Button style="flex:1;margin:0" loading={deleteLoading} onclick={() => void onDeleteCircle()}>
				Удалить
			</Button>
		</div>
	</OverlayLayout>
{/if}

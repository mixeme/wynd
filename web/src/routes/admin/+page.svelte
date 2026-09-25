<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import QuotaRequestRow from '$ui/admin/QuotaRequestRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import StackBar from '$ui/admin/StackBar.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { appVersion } from '$lib/appinfo';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import { loadSourceUrl, sourceUrl } from '$lib/instance/source.svelte';
	import { CIRCLE_COLOR_ORDER, CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import {
		fetchQuotaRequests,
		fetchStorage,
		resolveQuotaRequest,
		serverCaption,
		setCircleQuota,
		setDefaultCircleQuota,
		setStorageQuotaBytes,
		setStorageQuotaDiskPercent,
		type AdminStorageCircle,
		type QuotaRequest
	} from '$lib/admin/admin';

	const GB = 1024 * 1024 * 1024;
	const DEFAULT_CHIPS = [
		{ key: 'none', label: 'Нет', bytes: null as number | null },
		{ key: '5', label: '5 ГБ', bytes: 5 * GB },
		{ key: '10', label: '10 ГБ', bytes: 10 * GB },
		{ key: 'custom', label: 'Своё…', bytes: null as number | null }
	] as const;

	let used = $state(0);
	let quota = $state(0);
	let quotaInput = $state('');
	let quotaPercentInput = $state('');
	let defaultQuota = $state<number | null>(null);
	let defaultChip = $state<'none' | '5' | '10' | 'custom'>('none');
	let defaultCustomGb = $state('5');
	let circles = $state<AdminStorageCircle[]>([]);
	let requests = $state<QuotaRequest[]>([]);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let circleQuotaGb = $state('');
	let circleQuotaHint = $state('');
	let defaultCustomHint = $state('');
	let instanceQuotaHint = $state('');
	const quotaGbHintText = 'Квота — целые гигабайты';
	const quotaPercentHintText = 'Процент диска — целое число от 1 до 100';
	const instanceQuotaExplainHint =
		'Действует одно: гигабайты или доля диска. Сохранение одного очищает другое. При упоре текст пишется, медиа — нет.';

	const selectedCircleId = $derived($page.url.searchParams.get('circle') ?? '');
	const selectedCircle = $derived(circles.find((c) => c.id === selectedCircleId));
	const pendingForCircle = $derived(
		selectedCircleId ? requests.find((r) => r.circle_id === selectedCircleId) : undefined
	);

	const usedPct = $derived(quota > 0 ? Math.min(100, (used / quota) * 100) : 0);
	const free = $derived(Math.max(0, quota - used));
	const barSegments = $derived(
		circles
			.filter((c) => c.media_bytes > 0 && quota > 0)
			.map((c) => ({
				width: `${Math.max(0.4, (c.media_bytes / quota) * 100)}%`,
				color: colorForCircle(c)
			}))
	);

	function colorFor(id: string): string {
		const hash = id.split('').reduce((a, c) => a + c.charCodeAt(0), 0);
		const name = CIRCLE_COLOR_ORDER[hash % CIRCLE_COLOR_ORDER.length];
		return CIRCLE_COLORS[name].cssVar;
	}

	function colorForCircle(c: AdminStorageCircle): string {
		if (c.color in CIRCLE_COLORS) {
			return CIRCLE_COLORS[c.color as CircleColor].cssVar;
		}
		return colorFor(c.id);
	}

	function circleName(id: string): string {
		return circles.find((c) => c.id === id)?.name ?? id.slice(0, 8);
	}

	function effectiveQuotaBytes(c: AdminStorageCircle): number | null {
		if (c.quota_custom) return c.quota_bytes;
		return defaultQuota;
	}

	function formatQuota(bytes: number): string {
		const gb = bytes / GB;
		if (Math.abs(gb - Math.round(gb)) < 1e-9) return `${Math.round(gb)} ГБ`;
		return formatBytes(bytes);
	}

	function parsePercentInt(raw: string): number | null {
		const n = Number(raw.trim().replace(',', '.'));
		if (!Number.isInteger(n) || n < 1 || n > 100) return null;
		return n;
	}

	function syncInstanceQuotaFields(storage: {
		storage_quota_bytes: number;
		storage_quota_disk_percent: number | null;
	}) {
		if (storage.storage_quota_disk_percent != null) {
			quotaInput = '';
			quotaPercentInput = String(storage.storage_quota_disk_percent);
			return;
		}
		quotaPercentInput = '';
		quotaInput = String(Math.round(storage.storage_quota_bytes / GB));
	}

	function parseGbInt(raw: string, max?: number): number | null {
		const n = Number(raw.trim().replace(',', '.'));
		if (!Number.isInteger(n) || n < 1) return null;
		if (max != null && n > max) return null;
		return n;
	}

	function defaultLabel(): string {
		if (defaultQuota == null) return 'нет';
		return formatQuota(defaultQuota);
	}

	function syncDefaultChip(bytes: number | null) {
		if (bytes == null) {
			defaultChip = 'none';
			return;
		}
		if (bytes === 5 * GB) {
			defaultChip = '5';
			return;
		}
		if (bytes === 10 * GB) {
			defaultChip = '10';
			return;
		}
		defaultChip = 'custom';
		defaultCustomGb = String(Math.round(bytes / GB));
	}

	function requestDate(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' }).format(d);
	}

	function syncCircleField(c: AdminStorageCircle | undefined, pending?: QuotaRequest) {
		if (!c) {
			circleQuotaGb = '';
			return;
		}
		if (pending) {
			circleQuotaGb = String(Math.round(pending.requested_bytes / GB));
			return;
		}
		if (c.quota_custom && c.quota_bytes != null) {
			circleQuotaGb = String(Math.round(c.quota_bytes / GB));
			return;
		}
		const eff = effectiveQuotaBytes(c);
		circleQuotaGb = eff != null ? String(Math.round(eff / GB)) : '';
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [storage, list, caption] = await Promise.all([
				fetchStorage(),
				fetchQuotaRequests(),
				serverCaption()
			]);
			used = storage.used_bytes;
			quota = storage.quota_bytes;
			syncInstanceQuotaFields(storage);
			defaultQuota = storage.default_circle_quota_bytes;
			syncDefaultChip(storage.default_circle_quota_bytes);
			circles = storage.circles ?? [];
			requests = list;
			server = caption;
			syncCircleField(selectedCircle, pendingForCircle);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	async function saveQuotaGb() {
		const gb = parseGbInt(quotaInput);
		if (gb == null) {
			instanceQuotaHint = quotaGbHintText;
			return;
		}
		instanceQuotaHint = '';
		try {
			await setStorageQuotaBytes(gb * GB);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function saveQuotaPercent() {
		const pct = parsePercentInt(quotaPercentInput);
		if (pct == null) {
			instanceQuotaHint = quotaPercentHintText;
			return;
		}
		instanceQuotaHint = '';
		try {
			await setStorageQuotaDiskPercent(pct);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function saveDefaultQuota(bytes: number | null) {
		try {
			await setDefaultCircleQuota(bytes);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function pickDefaultChip(key: 'none' | '5' | '10' | 'custom') {
		defaultChip = key;
		if (key === 'custom') return;
		const chip = DEFAULT_CHIPS.find((c) => c.key === key);
		await saveDefaultQuota(chip?.bytes ?? null);
	}

	async function saveDefaultCustom() {
		const gb = parseGbInt(defaultCustomGb, 1024);
		if (gb == null) {
			defaultCustomHint = quotaGbHintText;
			return;
		}
		defaultCustomHint = '';
		await saveDefaultQuota(gb * GB);
	}

	function toggleCircle(id: string) {
		const params = new URLSearchParams($page.url.searchParams);
		if (params.get('circle') === id) {
			params.delete('circle');
		} else {
			params.set('circle', id);
		}
		const q = params.toString();
		void goto(q ? `/admin?${q}` : '/admin', { replaceState: true, keepFocus: true });
	}

	async function saveCircleQuota(mode: 'default' | 'none' | 'gb') {
		if (!selectedCircle) return;
		try {
			if (mode === 'default') {
				await setCircleQuota(selectedCircle.id, { custom: false });
			} else if (mode === 'none') {
				await setCircleQuota(selectedCircle.id, { custom: true, quota_bytes: null });
			} else {
				const gb = parseGbInt(circleQuotaGb);
				if (gb == null) {
					circleQuotaHint = quotaGbHintText;
					return;
				}
				circleQuotaHint = '';
				await setCircleQuota(selectedCircle.id, { custom: true, quota_bytes: gb * GB });
			}
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function onReject(id: string) {
		try {
			await resolveQuotaRequest(id, false);
			await load();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	function giveQuota(req: QuotaRequest) {
		const params = new URLSearchParams($page.url.searchParams);
		params.set('circle', req.circle_id);
		void goto(`/admin?${params.toString()}`, { keepFocus: true });
	}

	$effect(() => {
		syncCircleField(selectedCircle, pendingForCircle);
		circleQuotaHint = '';
	});

	onMount(() => {
		// Ссылка на исходники в футере — от этого же сервера (LIC-2).
		loadSourceUrl();
		void load();
	});
</script>

<AdminWideLayout app active="Хранилище" {server}>
	<AdminSection title="Хранилище">
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if error}
			<Hint>{error}</Hint>
		{:else}
			<StackBar
				threshold="90%"
				segments={barSegments.length ? barSegments : [{ width: `${usedPct}%`, color: 'var(--ink)' }]}
			/>
			<div style="font-size:12.5px;color:var(--muted);margin:10px 0 22px">
				{formatBytes(used)} из {formatBytes(quota)} · свободно {formatBytes(free)}
			</div>
			<div style="display:flex;align-items:center;gap:14px;margin-top:26px;flex-wrap:wrap">
				<span style="font-size:13.5px;font-weight:600;width:168px;flex:0 0 auto">Потолок инстанса</span>
				<Input
					admin
					style="width:88px"
					placeholder="40"
					bind:value={quotaInput}
					onchange={() => void saveQuotaGb()}
				/>
				<span style="font-size:12.5px;color:var(--muted)">ГБ</span>
				<span style="font-size:12.5px;color:var(--muted)">или</span>
				<Input
					admin
					style="width:88px"
					placeholder="80"
					bind:value={quotaPercentInput}
					onchange={() => void saveQuotaPercent()}
				/>
				<span style="font-size:12.5px;color:var(--muted)">% диска</span>
			</div>
			<Hint style="margin-top:8px;line-height:1.5">{instanceQuotaExplainHint}</Hint>
			{#if instanceQuotaHint}
				<Hint style="margin-top:8px">{instanceQuotaHint}</Hint>
			{/if}
			<div style="display:flex;align-items:flex-start;gap:14px;margin-top:14px">
				<span style="font-size:13.5px;font-weight:600;width:168px;flex:0 0 auto;padding-top:8px"
					>Квота круга по умолчанию</span
				>
				<div>
					<ChipGroup style="margin:0">
						{#each DEFAULT_CHIPS as chip (chip.key)}
							<Chip
								selected={defaultChip === chip.key}
								onclick={() => void pickDefaultChip(chip.key)}
							>
								{chip.label}
							</Chip>
						{/each}
					</ChipGroup>
					{#if defaultChip === 'custom'}
						<div style="display:flex;align-items:center;gap:10px;margin-top:8px">
							<Input
								admin
								style="width:88px"
								bind:value={defaultCustomGb}
								onchange={() => void saveDefaultCustom()}
							/>
							<span style="font-size:12.5px;color:var(--muted)">ГБ</span>
						</div>
						{#if defaultCustomHint}
							<Hint style="margin-top:8px">{defaultCustomHint}</Hint>
						{/if}
					{/if}
					<div style="font-size:11.5px;color:var(--faint);margin-top:8px;line-height:1.5">
						Новые круги и те, у кого в таблице не «своя». Потолок инстанса при этом никто не
						обходит.
					</div>
				</div>
			</div>
			{#if circles.length}
				<DataTable style="margin-top:20px">
					<thead>
						<tr>
							<th style="width:34%">Круг</th>
							<th>Записей</th>
							<th>Медиа</th>
							<th>Квота</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each circles as c (c.id)}
							{@const tint = colorForCircle(c)}
							{@const eff = effectiveQuotaBytes(c)}
							{@const fill = eff && eff > 0 ? Math.min(100, (c.media_bytes / eff) * 100) : 0}
							<tr
								style:background={selectedCircleId === c.id ? 'var(--ct)' : undefined}
								style="cursor:pointer"
								onclick={() => toggleCircle(c.id)}
							>
								<td class="n"><span class="dot" style="background:{tint}"></span>{c.name}</td>
								<td>{c.posts}</td>
								<td>{formatBytes(c.media_bytes)}</td>
								<td>
									{#if c.quota_custom && c.quota_bytes == null}
										<span style="color:var(--faint)">без квоты · своя</span>
									{:else if c.quota_custom && c.quota_bytes != null}
										<span class="qbar"><u style="width:{fill}%;background:{tint}"></u></span>
										{formatQuota(c.quota_bytes)}
										<span style="color:var(--faint);font-weight:400"> своя</span>
									{:else if eff != null}
										<span class="qbar"><u style="width:{fill}%;background:{tint}"></u></span>
										{formatQuota(eff)}
									{:else}
										<span style="color:var(--faint)">без квоты</span>
									{/if}
								</td>
								<td style="text-align:right;width:22px;padding-right:0"
									><Icon name="chevr" size="sm" /></td
								>
							</tr>
						{/each}
					</tbody>
				</DataTable>
			{/if}
			{#if selectedCircle}
				<div
					style="margin-top:16px;border:1px solid var(--line);background:var(--card);border-radius:12px;padding:14px 16px"
				>
					<div style="font-size:13.5px;font-weight:600">
						{selectedCircle.name}{selectedCircle.quota_custom ? ' · своя квота' : ''}
					</div>
					<div style="font-size:12.5px;color:var(--muted);margin:4px 0 10px">
						владелец · {selectedCircle.owner_email}
					</div>
					<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">
						<Input
							admin
							style="width:88px;font-weight:600"
							bind:value={circleQuotaGb}
							onchange={() => void saveCircleQuota('gb')}
						/>
						<span style="font-size:12.5px;color:var(--muted)">ГБ</span>
						<ChipGroup style="margin:0">
							<Chip onclick={() => void saveCircleQuota('default')}>
								Как умолчание · {defaultLabel()}
							</Chip>
							{#if pendingForCircle}
								<Chip selected onclick={() => void saveCircleQuota('gb')}>
									{Math.round(pendingForCircle.requested_bytes / GB)} ГБ
								</Chip>
							{/if}
							<Chip onclick={() => void saveCircleQuota('none')}>Без квоты</Chip>
						</ChipGroup>
					</div>
					{#if circleQuotaHint}
						<Hint style="margin-top:8px">{circleQuotaHint}</Hint>
					{/if}
					<div style="font-size:11.5px;color:var(--faint);margin-top:10px;line-height:1.5">
						{formatBytes(selectedCircle.media_bytes)} уже лежит. Ниже этого числа поставить можно —
						владелец увидит «место кончилось» и сам выберет отсечку. Панель её не ставит.
					</div>
				</div>
			{/if}
			{#if !selectedCircleId}
				<SectionLabel style="margin:26px 0 8px">Просят больше</SectionLabel>
				{#if requests.length === 0}
					<Hint>Запросов нет.</Hint>
				{:else}
					{#each requests as req (req.id)}
						{@const circle = circles.find((c) => c.id === req.circle_id)}
						{@const prev = circle ? effectiveQuotaBytes(circle) : null}
						<QuotaRequestRow
							circleColor={colorFor(req.circle_id)}
							circleName={circleName(req.circle_id)}
							requested={formatQuota(req.requested_bytes)}
							previous={prev != null ? String(Math.round(prev / GB)) : undefined}
							requester={req.requester_email}
							date={requestDate(req.created_at)}
							freeSpace={formatBytes(free)}
							onapprove={() => giveQuota(req)}
							onreject={() => void onReject(req.id)}
						/>
					{/each}
				{/if}
				<div style="font-size:11.5px;color:var(--faint);margin-top:14px;line-height:1.6">
					Отказ ничего не ломает: владелец круга сам выберет отсечку и срок, скажет об этом людям
					и освободит место. Ни отсечку, ни срок панель не двигает — этих кнопок здесь нет.
				</div>
			{/if}
		{/if}
	</AdminSection>
	<Hint centered style="margin-top:26px">
		Wynd {appVersion} · AGPL-3.0 ·
		<a class="under" href={sourceUrl()}>исходный код</a> ·
		<a class="under" href="/THIRD_PARTY_LICENSES.txt">лицензии компонентов</a>
	</Hint>
</AdminWideLayout>

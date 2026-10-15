<script lang="ts">
	import AboutFooter from '$ui/data/AboutFooter.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import Panel from '$ui/admin/Panel.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import QuotaRequestRow from '$ui/admin/QuotaRequestRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import StackBar from '$ui/admin/StackBar.svelte';
	import StackLegend from '$ui/admin/StackLegend.svelte';
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import { WORD, plural } from '$lib/format/plural';
	import { formatAdminDay } from '$lib/format/time';
	import { loadSourceUrl } from '$lib/instance/source.svelte';
	import { CIRCLE_COLOR_ORDER, CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import {
		clearReadyArchives,
		fetchQuotaRequests,
		fetchStorage,
		resolveQuotaRequest,
		serverCaption,
		setCircleQuota,
		setArchiveRetention,
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
		'Действует одно: гигабайты или доля диска. Сохранение одного очищает другое. При упоре писать нельзя ни в один круг: ни текст, ни фото.';

	const selectedCircleId = $derived($page.url.searchParams.get('circle') ?? '');
	const selectedCircle = $derived(circles.find((c) => c.id === selectedCircleId));
	const pendingForCircle = $derived(
		selectedCircleId ? requests.find((r) => r.circle_id === selectedCircleId) : undefined
	);

	const usedPct = $derived(quota > 0 ? Math.min(100, (used / quota) * 100) : 0);
	const free = $derived(Math.max(0, quota - used));
	// Готовые архивы на шкале (9.5) — штриховкой: в потолке они есть, но это
	// не круг.
	const ARCHIVE_HATCH = 'repeating-linear-gradient(135deg,#C9C0B2 0 6px,#DFD8CD 6px 12px)';
	let archivesBytes = $state(0);
	let archivesCount = $state(0);
	let retentionInput = $state('72');
	let retentionHint = $state('');
	let clearing = $state(false);
	const circlesBytes = $derived(circles.reduce((sum, c) => sum + c.media_bytes, 0));
	const barSegments = $derived([
		...circles
			.filter((c) => c.media_bytes > 0 && quota > 0)
			.map((c) => ({
				width: `${Math.max(0.4, (c.media_bytes / quota) * 100)}%`,
				color: colorForCircle(c)
			})),
		...(archivesBytes > 0 && quota > 0
			? [{ width: `${Math.max(0.4, (archivesBytes / quota) * 100)}%`, color: ARCHIVE_HATCH }]
			: [])
	]);

	async function saveRetention() {
		retentionHint = '';
		const hours = Number(String(retentionInput).trim());
		if (!Number.isInteger(hours) || hours < 1 || hours > 8760) {
			retentionHint = 'Срок — целое число часов, от 1 до 8760 (год).';
			return;
		}
		try {
			await setArchiveRetention(hours);
		} catch (err) {
			retentionHint = authErrorHint(err);
		}
	}

	async function clearArchives() {
		clearing = true;
		try {
			await clearReadyArchives();
			await load();
		} catch (err) {
			retentionHint = authErrorHint(err);
		} finally {
			clearing = false;
		}
	}

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
			archivesBytes = storage.archives_bytes ?? 0;
			archivesCount = storage.archives_count ?? 0;
			retentionInput = String(storage.archive_retention_hours ?? 72);
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
		// Круга ещё нет — править нечего: «Дать» одобряет просьбу, квоту круг
		// получит, когда оживёт.
		if (!req.circle_id) {
			void (async () => {
				try {
					await resolveQuotaRequest(req.id, true);
					await load();
				} catch (err) {
					error = authErrorHint(err);
				}
			})();
			return;
		}
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
			<Loading compact />
		{:else if error}
			<Hint>{error}</Hint>
		{:else}
			<StackBar
				threshold="90%"
				segments={barSegments.length ? barSegments : [{ width: `${usedPct}%`, color: 'var(--ink)' }]}
			/>
			<div class="note {archivesBytes > 0 ? 'm-10-0-6' : 'm-10-0-22'}">
				{formatBytes(used)} из {formatBytes(quota)} · свободно {formatBytes(free)}
			</div>
			{#if archivesBytes > 0}
				<!-- 9.5: круги и готовые архивы отдельно. -->
				<StackLegend
					items={[
						{
							label: `круги — ${formatBytes(circlesBytes)}, цвета как в таблице ниже`,
							swatch:
								'linear-gradient(90deg,var(--terracotta) 0 25%,var(--olive) 25% 50%,var(--indigo) 50% 75%,var(--ochre) 75%)'
						},
						{ label: `готовые архивы — ${formatBytes(archivesBytes)}`, swatch: ARCHIVE_HATCH }
					]}
				/>
			{/if}
			<div class="flex-mid gap-14 wrap mt-26">
				<span class="ttl flab">Потолок инстанса</span>
				<Input
					admin
					class="w88"
					placeholder="40"
					bind:value={quotaInput}
					onchange={() => void saveQuotaGb()}
				/>
				<span class="note">ГБ</span>
				<span class="note">или</span>
				<Input
					admin
					class="w88"
					placeholder="80"
					bind:value={quotaPercentInput}
					onchange={() => void saveQuotaPercent()}
				/>
				<span class="note">% диска</span>
			</div>
			<Hint class="mt-8">{instanceQuotaExplainHint}</Hint>
			{#if instanceQuotaHint}
				<Hint class="mt-8">{instanceQuotaHint}</Hint>
			{/if}
			<div class="flex-top gap-14 mt-14">
				<span class="ttl flab pt-8">Квота круга по умолчанию</span>
				<div>
					<ChipGroup class="m-0">
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
						<div class="flex-mid gap-10 mt-8">
							<Input
								admin
								class="w88"
								bind:value={defaultCustomGb}
								onchange={() => void saveDefaultCustom()}
							/>
							<span class="note">ГБ</span>
						</div>
						{#if defaultCustomHint}
							<Hint class="mt-8">{defaultCustomHint}</Hint>
						{/if}
					{/if}
					<div class="fine mt-8">
						Новые круги и те, у кого в таблице не «своя». Потолок инстанса при этом никто не
						обходит.
					</div>
				</div>
			</div>
			{#if circles.length}
				<DataTable class="mt-20">
					<thead>
						<tr>
							<th class="w-34p">Круг</th>
							<th>Записей</th>
							<th>Занято</th>
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
								class:bg-ct={selectedCircleId === c.id}
								class="pointer"
								onclick={() => toggleCircle(c.id)}
							>
								<td class="n"><span class="dot" style="background:{tint}"></span>{c.name}</td>
								<td>{c.posts}</td>
								<td>{formatBytes(c.media_bytes)}</td>
								<td>
									{#if c.quota_custom && c.quota_bytes == null}
										<span class="faint">без квоты · своя</span>
									{:else if c.quota_custom && c.quota_bytes != null}
										<Meter inline value={fill} color={tint} />
										{formatQuota(c.quota_bytes)}
										<span class="faint normal"> своя</span>
									{:else if eff != null}
										<Meter inline value={fill} color={tint} />
										{formatQuota(eff)}
									{:else}
										<span class="faint">без квоты</span>
									{/if}
								</td>
								<td class="chev"
									><Icon name="chevr" size="sm" /></td
								>
							</tr>
						{/each}
					</tbody>
				</DataTable>
			{/if}
			{#if selectedCircle}
				<Panel class="mt-16">
					<div class="ttl">
						{selectedCircle.name}{selectedCircle.quota_custom ? ' · своя квота' : ''}
					</div>
					<div class="note m-4-0-10">
						владелец · {selectedCircle.owner_email}
					</div>
					<div class="flex-mid gap-10 wrap">
						<Input
							admin
						 class="w88 bold"
							bind:value={circleQuotaGb}
							onchange={() => void saveCircleQuota('gb')}
						/>
						<span class="note">ГБ</span>
						<ChipGroup class="m-0">
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
						<Hint class="mt-8">{circleQuotaHint}</Hint>
					{/if}
					<div class="fine mt-10">
						{formatBytes(selectedCircle.media_bytes)} уже лежит. Ниже этого числа поставить можно —
						владелец увидит «место кончилось» и сам выберет отсечку. Панель её не ставит.
					</div>
				</Panel>
			{/if}
			{#if !selectedCircleId}
				<SectionLabel class="mt-26 mx-0">Просят больше</SectionLabel>
				{#if requests.length === 0}
					<Hint>Запросов нет.</Hint>
				{:else}
					{#each requests as req (req.id)}
						{@const circle = circles.find((c) => c.id === req.circle_id)}
						{@const prev = circle ? effectiveQuotaBytes(circle) : null}
						<QuotaRequestRow
							circleColor={req.circle_id ? colorFor(req.circle_id) : 'var(--line)'}
							circleName={req.circle_id
								? circleName(req.circle_id)
								: `${req.circle_name ?? 'Круг'} — оживляют из архива`}
							requested={formatQuota(req.requested_bytes)}
							previous={prev != null ? String(Math.round(prev / GB)) : undefined}
							requester={req.requester_email}
							date={formatAdminDay(req.created_at)}
							freeSpace={formatBytes(free)}
							onapprove={() => giveQuota(req)}
							onreject={() => void onReject(req.id)}
						/>
					{/each}
				{/if}
				<div class="faint lh-16 sz-11 mt-14">
					Отказ ничего не ломает: владелец круга сам выберет отсечку и срок, скажет об этом людям
					и освободит место. Ни отсечку, ни срок панель не двигает — этих кнопок здесь нет.
				</div>
				<!-- 9.12: готовые полные архивы кругов — срок хранения и аварийная очистка. -->
				<SectionLabel class="mt-26 mx-0">Готовые архивы</SectionLabel>
				<CheckRow
					name="Хранить готовый архив"
					description="Скачанный архив сервер удаляет сразу. Не скачанный — через это время после сборки."
				>
					{#snippet actions()}
						<span class="flex-mid gap-10">
							<Input admin class="w88" bind:value={retentionInput} onchange={() => void saveRetention()} />
							<span class="note">ч</span>
						</span>
					{/snippet}
				</CheckRow>
				<CheckRow
					name={`${formatBytes(archivesBytes)} · ${plural(archivesCount, WORD.file)}`}
					description="Полные архивы кругов: собраны по просьбе владельцев и ждут скачивания; в квоты кругов не входят. Удаление круги и записи не трогает: владелец увидит, что файла нет, и соберёт архив снова."
				>
					{#snippet actions()}
						<Button
							variant="ghost"
							loading={clearing}
							disabled={archivesCount === 0}
							onclick={() => void clearArchives()}
						>
							Удалить все
						</Button>
					{/snippet}
				</CheckRow>
				{#if retentionHint}
					<Hint class="mt-8">{retentionHint}</Hint>
				{/if}
			{/if}
		{/if}
	</AdminSection>
	<AboutFooter class="mt-26" />
</AdminWideLayout>

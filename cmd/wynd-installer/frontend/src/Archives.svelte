<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FieldLine from '$ui/workshop/FieldLine.svelte';
	import FileRow from '$ui/workshop/FileRow.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import Checkbox from '$ui/workshop/Checkbox.svelte';
	import {
		backend,
		type ArchiveResult,
		type ArchiveSummary,
		type FullInfo,
		type MergeInfo,
		type SplitInfo
	} from './api';

	// Вкладка «Архивы» (план 45, кадры 11а–11г): сервер не нужен — всё на
	// этом компьютере. Исходные файлы не меняются.
	type View = 'home' | 'merge' | 'split' | 'full' | 'done';
	let view = $state<View>('home');
	let working = $state(false);
	let error = $state('');
	let result = $state<ArchiveResult | undefined>();

	// 11б
	let mergePaths = $state<string[]>([]);
	let merge = $state<MergeInfo | undefined>();
	let mergeOut = $state('');
	// 11в
	let split = $state<SplitInfo | undefined>();
	let byYears = $state(true);
	let years = $state<Record<number, boolean>>({});
	let from = $state('');
	let to = $state('');
	let splitDir = $state('');
	// 11г
	let fullPath = $state('');
	let full = $state<FullInfo | undefined>();
	let password = $state('');
	let fullOut = $state('');

	const MONTHS = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'];
	function date(iso?: string): string {
		if (!iso) return '';
		const [y, m, d] = iso.split('-').map(Number);
		return y && m && d ? `${d} ${MONTHS[m - 1]} ${y}` : iso;
	}
	function size(bytes: number): string {
		if (bytes >= 1024 ** 3) return (bytes / 1024 ** 3).toFixed(1).replace('.', ',') + ' ГБ';
		return Math.max(1, Math.round(bytes / 1024 ** 2)) + ' МБ';
	}
	function posts(n: number): string {
		const m10 = n % 10, m100 = n % 100;
		const w = m10 === 1 && m100 !== 11 ? 'запись' : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14) ? 'записи' : 'записей';
		return `${n} ${w}`;
	}
	function range(s: ArchiveSummary): string {
		return s.from ? `${date(s.from)} — ${date(s.to)} · ${posts(s.posts)}` : posts(s.posts);
	}
	function dir(path: string): string {
		const i = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'));
		return i > 0 ? path.slice(0, i) : path;
	}

	function home() {
		view = 'home';
		error = '';
		result = undefined;
	}

	async function addArchives() {
		const picked = (await backend().PickArchives()) ?? [];
		mergePaths = [...new Set([...mergePaths, ...picked])];
		await refreshMerge();
	}
	async function removeArchive(path: string) {
		mergePaths = mergePaths.filter((p) => p !== path);
		await refreshMerge();
	}
	async function refreshMerge() {
		merge = mergePaths.length ? await backend().MergeInfo(mergePaths) : undefined;
		if (merge?.suggest && mergePaths.length && !mergeOut) {
			mergeOut = dir(mergePaths[0]) + '\\' + merge.suggest;
		}
	}
	async function chooseMergeOut() {
		const p = await backend().PickSave(merge?.suggest ?? 'архив.zip');
		if (p) mergeOut = p;
	}
	async function doMerge() {
		if (working || !merge?.ok || !mergeOut) return;
		await run(() => backend().Merge(mergePaths, mergeOut));
	}

	async function pickSplit() {
		const p = await backend().PickArchive(false);
		if (!p) return;
		split = await backend().SplitInfo(p);
		years = Object.fromEntries((split.years ?? []).map((y) => [y.year, true]));
		splitDir = dir(p);
	}
	async function chooseSplitDir() {
		const p = await backend().PickFolder();
		if (p) splitDir = p;
	}
	const chosenYears = $derived(
		(split?.years ?? []).filter((y) => years[y.year]).map((y) => y.year)
	);
	async function doSplit() {
		if (working || !split?.file.ok || !splitDir) return;
		const path = split.file.path;
		if (byYears) {
			if (!chosenYears.length) return;
			await run(() => backend().Split(path, chosenYears, '', '', splitDir));
		} else {
			if (!from || !to) return;
			await run(() => backend().Split(path, [], from, to, splitDir));
		}
	}

	async function pickFull() {
		const p = await backend().PickArchive(true);
		if (!p) return;
		fullPath = p;
		full = await backend().FullInfo(p);
		password = '';
		const base = p.slice(dir(p).length + 1).replace(/\.wynd$/i, '');
		fullOut = dir(p) + '\\' + base + ' — весь круг.zip';
	}
	async function chooseFullOut() {
		const p = await backend().PickSave(fullOut.slice(dir(fullOut).length + 1));
		if (p) fullOut = p;
	}
	async function doFull() {
		if (working || !fullPath || !fullOut) return;
		await run(() => backend().Readable(fullPath, password, fullOut));
	}

	async function run(op: () => Promise<ArchiveResult>) {
		working = true;
		error = '';
		try {
			const res = await op();
			if (res.ok) {
				result = res;
				view = 'done';
			} else {
				error = res.message ?? 'Не получилось';
			}
		} finally {
			working = false;
		}
	}
</script>

<div class="ins">
	<main class="ins-body">
		{#if view === 'home'}
			<!-- Кадр 11а. -->
			<h2>Архивы на этом компьютере</h2>
			<p class="ins-sub">Сервер для этого не нужен — всё делается здесь.</p>
			<MarkRows>
				<MarkRow mark="+">
					Объединить архивы
					<div class="ins-dim">архивы одного круга за разное время — одной лентой, без повторов</div>
					{#snippet action()}
						<TextButton class="link under" onclick={() => (view = 'merge')}>выбрать</TextButton>
					{/snippet}
				</MarkRow>
				<MarkRow mark="÷">
					Разделить архив
					<div class="ins-dim">по годам или за выбранный срок — отдельными файлами</div>
					{#snippet action()}
						<TextButton class="link under" onclick={() => (view = 'split')}>выбрать</TextButton>
					{/snippet}
				</MarkRow>
				<MarkRow mark="≡">
					Лента из полного архива круга
					<div class="ins-dim">файл .wynd — в ленту, которую можно читать без Wynd</div>
					{#snippet action()}
						<TextButton class="link under" onclick={() => (view = 'full')}>выбрать</TextButton>
					{/snippet}
				</MarkRow>
			</MarkRows>
		{:else if view === 'merge'}
			<!-- Кадр 11б. -->
			<h2>Объединить архивы</h2>
			<Label>Архивы</Label>
			{#each merge?.files ?? [] as f (f.path)}
				<FileRow name={f.name} detail={range(f.summary)} error={f.ok ? undefined : f.error}>
					{#snippet action()}
						<button class="ins-x" type="button" aria-label="Убрать" onclick={() => void removeArchive(f.path)}>✕</button>
					{/snippet}
				</FileRow>
			{/each}
			<div class="ins-more">
				<TextButton class="link under" onclick={() => void addArchives()}>+ добавить архив</TextButton>
			</div>
			{#if merge?.ok}
				<Notice>
					Получится «{merge.summary.circle_name}», {date(merge.summary.from)} — {date(merge.summary.to)}:
					{posts(merge.summary.posts)}, {size(merge.summary.bytes)}.
					{merge.summary.duplicates
						? `Повторов: ${merge.summary.duplicates}, взяты поздние.`
						: 'Повторов не нашлось.'}
				</Notice>
				<Label>Сохранить как</Label>
				<FieldLine>
					<Input mono class="ins-grow" bind:value={mergeOut} />
					<Button variant="ghost" onclick={() => void chooseMergeOut()}>Выбрать</Button>
				</FieldLine>
			{/if}
			{#if error}<Notice tone="bad" alert>{error}</Notice>{/if}
			<FootBar>
				<Button variant={merge?.ok && mergeOut ? 'colored' : 'off'} loading={working} onclick={() => void doMerge()}>
					Объединить
				</Button>
				<Button variant="ghost" onclick={home}>Назад</Button>
			</FootBar>
		{:else if view === 'split'}
			<!-- Кадр 11в. -->
			<h2>Разделить архив</h2>
			<Label>Архив</Label>
			{#if split}
				<FileRow name={split.file.name} detail={range(split.file.summary)} error={split.file.ok ? undefined : split.file.error}>
					{#snippet action()}
						<TextButton class="link under" onclick={() => void pickSplit()}>другой</TextButton>
					{/snippet}
				</FileRow>
			{:else}
				<Button variant="ghost" onclick={() => void pickSplit()}>Выбрать архив</Button>
			{/if}
			{#if split?.file.ok}
				<Label>Как делить</Label>
				<div class="ins-chips">
					<Chip selected={byYears} onclick={() => (byYears = true)}>По годам</Chip>
					<Chip selected={!byYears} onclick={() => (byYears = false)}>За срок</Chip>
				</div>
				{#if byYears}
					{#each split.years ?? [] as y (y.year)}
						<Checkbox bind:checked={years[y.year]}>
							{y.year} <span class="ins-dim">— {posts(y.summary.posts)}, {size(y.summary.bytes)}</span>
						</Checkbox>
					{/each}
				{:else}
					<div class="ins-pair">
						<div class="ins-grow"><Label>С</Label><Input type="date" bind:value={from} /></div>
						<div class="ins-grow"><Label>По</Label><Input type="date" bind:value={to} /></div>
					</div>
				{/if}
				<Label>Куда</Label>
				<FieldLine>
					<Input mono class="ins-grow" bind:value={splitDir} />
					<Button variant="ghost" onclick={() => void chooseSplitDir()}>Выбрать</Button>
				</FieldLine>
			{/if}
			{#if error}<Notice tone="bad" alert>{error}</Notice>{/if}
			<FootBar>
				<Button
					variant={split?.file.ok && splitDir && (byYears ? chosenYears.length : from && to) ? 'colored' : 'off'}
					loading={working}
					onclick={() => void doSplit()}>Разделить</Button
				>
				<Button variant="ghost" onclick={home}>Назад</Button>
			</FootBar>
		{:else if view === 'full'}
			<!-- Кадр 11г. -->
			<h2>Лента из полного архива круга</h2>
			{#if full?.ok}
				<FileRow name={full.name ?? ''} detail="весь круг: записи и комментарии всех участников · {size(full.bytes ?? 0)}">
					{#snippet action()}
						<TextButton class="link under" onclick={() => void pickFull()}>другой</TextButton>
					{/snippet}
				</FileRow>
				{#if full.encrypted}
					<Label>Пароль архива</Label>
					<Input type="password" autocomplete="off" bind:value={password} />
				{/if}
				<Label>Сохранить как</Label>
				<FieldLine>
					<Input mono class="ins-grow" bind:value={fullOut} />
					<Button variant="ghost" onclick={() => void chooseFullOut()}>Выбрать</Button>
				</FieldLine>
			{:else}
				<Button variant="ghost" onclick={() => void pickFull()}>Выбрать файл .wynd</Button>
			{/if}
			{#if error}<Notice tone="bad" alert>{error}</Notice>{/if}
			<FootBar>
				<Button
					variant={full?.ok && fullOut && (!full.encrypted || password) ? 'colored' : 'off'}
					loading={working}
					onclick={() => void doFull()}>Сделать ленту</Button
				>
				<Button variant="ghost" onclick={home}>Назад</Button>
			</FootBar>
		{:else if view === 'done' && result}
			<h2>Готово</h2>
			{#each result.files ?? [] as f (f)}
				<MarkRow mark="✓" tone="ok"><div class="ins-mono">{f}</div></MarkRow>
			{/each}
			<FootBar>
				<Button variant="colored" onclick={() => void backend().ShowInFolder(result?.files?.[0] ?? '')}>
					Показать в папке
				</Button>
				<Button variant="ghost" onclick={home}>К архивам</Button>
			</FootBar>
		{/if}
	</main>
</div>

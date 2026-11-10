<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FieldLine from '$ui/workshop/FieldLine.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import StepList from '$ui/workshop/StepList.svelte';
	import { backend, watchProgress, type Check, type Release, type State, type Step } from './api';
	import { ReleaseRequests } from './release-requests';
	import { uuid } from '$lib/uuid';
	const requests = new ReleaseRequests(uuid);
	let checkRequest = '';

	// Экраны (docs/visual/installer.html): 12а — ключа нет, 12б — выпуски,
	// 12в–12г — проверка, подпись и итог на одном экране, 12д — подписан файл;
	// file — пароль для файла на этом компьютере.
	type Screen = 'create' | 'list' | 'release' | 'file' | 'filedone';
	let screen = $state<Screen>('list');
	let body = $state<HTMLElement>();
	let st = $state<State | undefined>();
	let keyNote = $state('');
	let keyError = $state('');

	// 12а
	let newKey = $state('');
	let pass1 = $state('');
	let pass2 = $state('');
	let creating = $state(false);
	let createError = $state('');
	const mismatch = $derived(pass2 !== '' && pass1 !== pass2);
	const canCreate = $derived([...pass1].length >= 12 && pass1 === pass2 && newKey !== '');

	// 12б
	let releases = $state<Release[]>([]);
	let listing = $state(false);
	let listError = $state('');
	let listRequest = 0;

	// 12в, 12г
	let check = $state<Check | undefined>();
	let checking = $state(false);
	let checkError = $state('');
	let selectedRelease = $state<Release | undefined>();
	let checkSteps = $state<Step[]>([]);
	let signSteps = $state<Step[]>([]);
	let releaseDone = $state(false);
	let pass = $state('');
	let wrong = $state(false);
	let signing = $state(false);
	let signError = $state('');
	let published = $state(false);
	let releaseUrl = $state('');

	// file, 12д
	let filePath = $state('');
	let fileSize = $state(0);
	let sigPath = $state('');

	onMount(() => watchProgress((p) => {
		if (screen !== 'release' || !requests.acceptsProgress(p, selectedRelease?.tag, check?.check_id)) return;
		if (p.phase === 'check' && checking) checkSteps = p.steps;
		if (p.phase === 'sign' && signing) signSteps = p.steps;
	}));

	void start();
	async function start() {
		st = await backend().State();
		newKey = st.key_path;
		if (st.key_exists) {
			screen = 'list';
			void list();
		} else {
			screen = 'create';
		}
	}

	function mb(bytes: number): string {
		return Math.max(1, Math.round(bytes / 1024 ** 2)) + ' МБ';
	}
	function day(iso?: string): string {
		if (!iso) return '';
		const d = new Date(iso);
		return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
	}

	async function chooseNewKey() {
		createError = '';
		try {
			const p = await backend().ChooseKeyPlace(newKey);
			if (p) newKey = p;
		} catch (err) {
			createError = String(err);
		}
	}
	async function create() {
		if (!canCreate || creating) return;
		creating = true;
		createError = '';
		try {
			const res = await backend().CreateKey(newKey, pass1);
			if (!res.ok) {
				createError = res.message ?? 'Создать ключ не получилось';
				return;
			}
			pass1 = pass2 = '';
			keyNote = `Ключ создан: ${res.path}. Открытый ключ записан в ${res.pub_path} — закоммитьте его.`;
			st = await backend().State();
			screen = 'list';
			void list();
		} finally {
			creating = false;
		}
	}

	async function list() {
		const request = ++listRequest;
		listing = true;
		listError = '';
		try {
			const res = await backend().Releases();
			if (request !== listRequest) return;
			if (!res.ok) {
				listError = res.message ?? 'GitHub не ответил';
				return;
			}
			releases = res.releases ?? [];
		} catch (err) {
			if (request === listRequest) listError = String(err);
		} finally {
			if (request === listRequest) listing = false;
		}
	}
	async function otherKey() {
		keyError = '';
		try {
			st = await backend().ChooseKey();
		} catch (err) {
			keyError = String(err);
		}
	}
	function toList() {
		requests.clear();
		checking = signing = false;
		selectedRelease = undefined;
		check = undefined;
		pass = '';
		wrong = false;
		signError = '';
		keyNote = '';
		screen = 'list';
		void list();
	}

	async function openRelease(r: Release) {
		const request = requests.beginCheck();
		checkRequest = request;
		signing = false;
		selectedRelease = r;
		screen = 'release';
		check = undefined;
		checkError = '';
		pass = '';
		wrong = false;
		signError = '';
		checkSteps = [];
		signSteps = [];
		releaseDone = false;
		checking = true;
		void tick().then(() => body?.scrollTo({ top: 0 }));
		try {
			const result = await backend().CheckRelease(r.tag, request);
			if (!requests.isCurrentCheck(request)) return;
			if (result.request_id !== request || result.tag !== r.tag || !result.check_id) {
				throw new Error('Ответ проверки не соответствует выбранному выпуску');
			}
			check = result;
			checkSteps = check.steps;
		} catch (e) {
			if (!requests.isCurrentCheck(request)) return;
			checkError = String(e);
			checkSteps = checkSteps.map((s) => s.status === 'running' ? { ...s, status: 'failed' } : s);
		} finally {
			if (requests.isCurrentCheck(request)) checking = false;
		}
	}
	async function signRelease() {
		if (!check || !pass || signing) return;
		const tag = check.tag;
		const checkId = check.check_id;
		const originalCheckRequest = checkRequest;
		const request = requests.beginSign();
		signing = true;
		wrong = false;
		signError = '';
		signSteps = [];
		try {
			const res = await backend().SignRelease(checkId, tag, request, pass);
			if (!requests.isCurrentSign(originalCheckRequest, request)) return;
			signSteps = res.steps ?? signSteps;
			if (res.wrong_password) {
				wrong = true;
				return;
			}
			if (!res.ok) {
				signError = res.message ?? 'Подписать не получилось';
				return;
			}
			pass = '';
			published = Boolean(res.published);
			releaseUrl = res.url ?? '';
			// Уже подтверждённая подпись видна сразу, даже пока GitHub отвечает.
			++listRequest;
			listing = false;
			releases = releases.map((r) =>
				r.tag === tag ? { ...r, signed: true, draft: !res.published } : r
			);
			releaseDone = true;
			void list();
		} catch (err) {
			if (!requests.isCurrentSign(originalCheckRequest, request)) return;
			signError = String(err);
			signSteps = signSteps.map((s) => s.status === 'running' ? { ...s, status: 'failed' } : s);
		} finally {
			if (requests.isCurrentSign(originalCheckRequest, request)) signing = false;
		}
	}

	async function pickFile() {
		const p = await backend().PickExe();
		if (!p) return;
		filePath = p;
		fileSize = await backend().FileSize(p);
		pass = '';
		wrong = false;
		signError = '';
		screen = 'file';
	}
	async function signFile() {
		if (!filePath || !pass || signing) return;
		signing = true;
		wrong = false;
		signError = '';
		try {
			const res = await backend().SignFile(filePath, pass);
			if (res.wrong_password) {
				wrong = true;
				return;
			}
			if (!res.ok) {
				signError = res.message ?? 'Подписать не получилось';
				return;
			}
			pass = '';
			sigPath = res.path ?? '';
			screen = 'filedone';
		} finally {
			signing = false;
		}
	}

	const version = $derived(selectedRelease?.version ?? check?.tag.replace(/^(workshop|desktop)-v/, '') ?? '');
	// Программа подписываемого выпуска: одна и та же строка списка не должна
	// выдавать окно за Мастерскую (и наоборот).
	const product = $derived(selectedRelease?.product ?? check?.product ?? 'workshop');
	const productName = $derived(nameOf(product));
	const exeFile = $derived(exeOf(product));
	function nameOf(p?: string): string {
		return p === 'desktop' ? 'Wynd для компьютера' : 'Мастерская';
	}
	function exeOf(p?: string): string {
		return p === 'desktop' ? 'wynd-desktop.exe' : 'wynd-workshop.exe';
	}
	const fileName = $derived(filePath.slice(Math.max(filePath.lastIndexOf('\\'), filePath.lastIndexOf('/')) + 1));
</script>

<svelte:window onfocus={() => { if (screen === 'list' && st && !listing) void list(); }} />

<div class="ins">
	<main class="ins-body" bind:this={body}>
		{#if screen === 'create'}
			<!-- Кадр 12а. -->
			<h2>Ключ подписи Wynd</h2>
			<p class="ins-sub">
				Этим ключом подписываются выпуски Мастерской и Wynd для компьютера. Мастерская принимает обновление,
				только если новая версия подписана им. Ключ создаётся один раз.
			</p>
			<Label>Пароль ключа</Label>
			<Input type="password" autocomplete="off" bind:value={pass1} />
			<Label>Ещё раз</Label>
			<Input type="password" autocomplete="off" bind:value={pass2} />
			{#if mismatch}<p class="ins-note" role="alert">Пароли не совпадают</p>{/if}
			<p class="ins-note">Не короче 12 знаков. Ключ сохранится зашифрованным этим паролем.</p>
			<Label>Где хранить ключ</Label>
			<FieldLine>
				<Input mono class="ins-grow" bind:value={newKey} />
				<Button variant="ghost" onclick={() => void chooseNewKey()}>Выбрать</Button>
			</FieldLine>
			<Notice tone="bad">
				Сохраните файл ключа и пароль в надёжном месте. Без них ни одну уже розданную Мастерскую больше не
				обновить: новую версию она не примет.
			</Notice>
			{#if createError}<Notice tone="bad" alert>{createError}</Notice>{/if}
			<FootBar>
				<Button variant={canCreate ? 'colored' : 'off'} loading={creating} loadingLabel="Создаём…" onclick={() => void create()}>Создать ключ</Button>
			</FootBar>
		{:else if screen === 'list' && st}
			<!-- Кадр 12б. -->
			{#if keyNote}<Notice tone="note">{keyNote}</Notice>{/if}
			<FieldLine>
				<h2 class="ins-grow">Выпуски</h2>
				<Button variant="ghost" disabled={listing} onclick={() => void list()}>{listing ? 'Обновляем…' : 'Обновить'}</Button>
			</FieldLine>
			{#if st.key_exists}
				<p class="ins-sub">
					Ключ — {st.key_path}
					<TextButton class="link under" onclick={() => void otherKey()}>другой</TextButton>
				</p>
			{:else}
				<Notice tone="bad" title="Ключ не найден: {st.key_path}">
					<FootBar>
						<Button variant="ghost" onclick={() => void otherKey()}>Выбрать</Button>
						<TextButton class="link under" onclick={() => (screen = 'create')}>создать новый</TextButton>
					</FootBar>
				</Notice>
			{/if}
			{#if keyError}<Notice tone="bad" alert>{keyError}</Notice>{/if}
			{#if listError}<Notice tone="bad" alert>{listError}</Notice>{/if}
			<MarkRows>
				{#each releases as r (r.tag)}
					<MarkRow mark={r.draft ? '●' : '✓'} tone={r.draft ? 'note' : 'ok'}>
						{#if r.url}
							<TextButton class="link under" onclick={() => void backend().OpenLink(r.url ?? '')}>{nameOf(r.product)} {r.version}</TextButton>
						{:else}
							{nameOf(r.product)} {r.version}
						{/if}
						<span class="ins-dim"
							>— {r.draft
								? r.signed
									? 'черновик, подписан'
									: 'черновик, не подписан'
								: `опубликован${r.published_at ? ' ' + day(r.published_at) : ''}, ${r.signed ? 'с подписью' : 'без подписи'}`}</span
						>
						{#snippet action()}
							{#if (r.draft || !r.signed) && st?.key_exists}
								<TextButton class="link under" onclick={() => void openRelease(r)}>подписать</TextButton>
							{/if}
						{/snippet}
					</MarkRow>
				{/each}
			</MarkRows>
			{#if st.key_exists}
				<MarkRows class="mt-10">
					<MarkRow mark="↓">
						Подписать файл на этом компьютере
						{#snippet action()}
							<TextButton class="link under" onclick={() => void pickFile()}>выбрать</TextButton>
						{/snippet}
					</MarkRow>
				</MarkRows>
			{/if}
		{:else if screen === 'release'}
			<!-- Проверки, пароль, подпись и итог остаются на одном экране. -->
			<h2>{productName} {version}</h2>
			<StepList steps={checkSteps} />
			{#if checkError}<Notice tone="bad" alert>{checkError}</Notice>{/if}
			{#if check?.problem}<Notice tone="bad" alert>{check.problem}. Такой exe подписывать нельзя.</Notice>{/if}
			{#if signSteps.length}<StepList steps={signSteps} />{/if}
			{#if releaseDone}
				<Notice tone="note" title={published ? 'Выпуск подписан и опубликован' : 'Выпуск подписан'}>
					{check?.key_in_release ? 'Подпись проверена открытым ключом из репозитория.' : 'Подпись проверена открытой частью выбранного ключа.'}
					{published
						? `Файл ${exeFile}.sig выложен в выпуск на GitHub.`
						: `Файл ${exeFile}.sig выложен в черновик. Он не опубликован: в whatsnew.md нет раздела ${version}.`}
					Временные файлы на этом компьютере удалены.
				</Notice>
				<FootBar>
					<Button variant="colored" onclick={() => void backend().OpenLink(releaseUrl)}>Открыть выпуск</Button>
					<Button variant="ghost" onclick={toList}>К выпускам</Button>
				</FootBar>
			{:else if checkError || check?.problem}
				<FootBar>
					<Button variant="colored" onclick={() => { if (selectedRelease) void openRelease(selectedRelease); }}>Повторить проверку</Button>
					<Button variant="ghost" onclick={toList}>К выпускам</Button>
				</FootBar>
			{:else}
				{#if check && !check.key_in_release && product !== 'desktop'}
					<Notice tone="note">В этой сборке нет открытого ключа. Подпись добавится к выпуску, но сама эта Мастерская обновляться не сможет. Для самообновления нужна новая сборка со встроенным открытым ключом.</Notice>
				{/if}
				{#if checking}<p class="ins-note" role="status">Пароль можно будет ввести после проверки выпуска.</p>{/if}
				<Label>Пароль ключа</Label>
				<Input type="password" autocomplete="off" bind:value={pass} disabled={checking || signing || !check} />
				{#if wrong}<p class="ins-note" role="alert">Пароль не подошёл</p>{/if}
				{#if check?.draft && check.notes}
					<Label>Что нового — из выпуска</Label>
					<Notice>
						{#each check.notes.split('\n') as line, i (i)}<div>{line}</div>{/each}
					</Notice>
				{:else if check?.draft}
					<Notice>В whatsnew.md нет раздела {version} — выпуск подпишем, но не опубликуем.</Notice>
				{/if}
				{#if check && !check.draft}
					<Notice>Выпуск уже опубликован. Добавим подпись, описание и дата публикации сохранятся.</Notice>
				{/if}
				{#if signError}<Notice tone="bad" alert>{signError}</Notice>{/if}
				<FootBar>
					<Button variant={pass && check && !checking ? 'colored' : 'off'} disabled={checking || !check || !pass} loading={signing} loadingLabel="Подписываем…" onclick={() => void signRelease()}>
						{check?.draft && check.notes ? 'Подписать и опубликовать' : 'Подписать'}
					</Button>
					<Button variant="ghost" disabled={checking || signing} onclick={toList}>К выпускам</Button>
				</FootBar>
			{/if}
		{:else if screen === 'file'}
			<!-- Пароль для файла на этом компьютере (как 12в, без проверок выпуска). -->
			<h2>{fileName}</h2>
			<MarkRows>
				<MarkRow mark="·">{filePath} <span class="ins-dim">— {mb(fileSize)}</span></MarkRow>
			</MarkRows>
			<Label>Пароль ключа</Label>
			<Input type="password" autocomplete="off" bind:value={pass} />
			{#if wrong}<p class="ins-note" role="alert">Пароль не подошёл</p>{/if}
			{#if signError}<Notice tone="bad" alert>{signError}</Notice>{/if}
			<FootBar>
				<Button variant={pass ? 'colored' : 'off'} loading={signing} loadingLabel="Подписываем…" onclick={() => void signFile()}>Подписать</Button>
				<Button variant="ghost" disabled={signing} onclick={toList}>Назад</Button>
			</FootBar>
		{:else if screen === 'filedone'}
			<!-- Кадр 12д. -->
			<h2>Файл подписан</h2>
			<p class="ins-sub">Подпись проверена открытым ключом из репозитория.</p>
			<Label>Подпись</Label>
			<Input mono readonly value={sigPath} />
			<p class="ins-note">Рядом с {filePath}. Сам exe не изменился: обновление Мастерской принимается только вместе с этим файлом.</p>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().ShowInFolder(sigPath)}>Показать в папке</Button>
				<Button variant="ghost" onclick={toList}>К выпускам</Button>
			</FootBar>
		{/if}
	</main>
</div>

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
	import { readReleaseScroll, releaseScrollTarget, type ReleaseScroll } from './release-scroll';
	import { uuid } from '$lib/uuid';
	const requests = new ReleaseRequests(uuid);
	let checkRequest = '';

	// Экраны (docs/visual/workshop.html): 12а — ключа нет, 12б — выпуски,
	// 12в–12г — проверка, подпись и итог на одном экране, 12д — подписан файл;
	// file — пароль для файла на этом компьютере; keys — ключи подписи (план 65);
	// androidcreate — создание ключа Android (план 65).
	type Screen = 'create' | 'list' | 'keys' | 'androidcreate' | 'release' | 'file' | 'filedone';
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
	// detail — итог подписи словами из Go (Android); у exe пусто и работает шаблон ниже.
	let doneDetail = $state('');

	// file, 12д
	let filePath = $state('');
	let fileSize = $state(0);
	let sigPath = $state('');

	// Прокрутка экрана выпуска: подмена содержимого сбрасывает контейнер к началу
	// (WebView теряет позицию при смене высоты и удалении поля). Запоминаем
	// текущее место перед каждой подменой и возвращаем его после; если человек
	// читал у конца, ведём за новыми этапами. Снимок, взятый один раз при
	// нажатии «Подписать», возвращал окно к началу на каждом обновлении этапа.
	let releaseScroll = $state<ReleaseScroll>({ top: 0, atEnd: false });
	function rememberReleaseScroll() {
		if (!body || screen !== 'release') return;
		releaseScroll = readReleaseScroll(body.scrollTop, body.scrollHeight, body.clientHeight);
	}
	function anchorRelease() {
		void tick().then(() => {
			if (!body || screen !== 'release') return;
			body.scrollTop = releaseScrollTarget(
				releaseScroll,
				body.scrollHeight,
				body.clientHeight,
				releaseDone
			);
		});
	}

	onMount(() => watchProgress((p) => {
		if (screen !== 'release' || !requests.acceptsProgress(p, selectedRelease?.tag, check?.check_id)) return;
		if (p.phase === 'check' && checking) {
			rememberReleaseScroll();
			checkSteps = p.steps;
			anchorRelease();
		}
		if (p.phase === 'sign' && signing) {
			rememberReleaseScroll();
			signSteps = p.steps;
			anchorRelease();
		}
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
	// Ключ Android выбирается или создаётся (поле — только там, где применяется);
	// без него пароль и подпись не показываются.
	async function chooseKeystore() {
		keyError = '';
		try {
			st = await backend().ChooseKeystore();
		} catch (err) {
			keyError = String(err);
		}
	}
	let akeyPath = $state('');
	let createReturn = $state<Screen>('release');
	function goAndroidCreate() {
		createReturn = screen === 'keys' ? 'keys' : 'release';
		akeyPath = st?.key_store_path ?? '';
		pass1 = pass2 = '';
		createError = '';
		screen = 'androidcreate';
	}
	const canCreateKeystore = $derived([...pass1].length >= 12 && pass1 === pass2 && akeyPath !== '');
	async function chooseKeystorePlace() {
		createError = '';
		try {
			const p = await backend().ChooseKeystorePlace(akeyPath);
			if (p) akeyPath = p;
		} catch (err) {
			createError = String(err);
		}
	}
	async function createKeystore() {
		if (!canCreateKeystore || creating) return;
		creating = true;
		createError = '';
		try {
			const res = await backend().CreateKeystore(akeyPath, pass1);
			if (!res.ok) {
				createError = res.message ?? 'Создать ключ не получилось';
				return;
			}
			pass1 = pass2 = '';
			st = await backend().State();
			screen = createReturn;
		} finally {
			creating = false;
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
		doneDetail = '';
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
		doneDetail = '';
		checkSteps = [];
		signSteps = [];
		releaseDone = false;
		releaseScroll = { top: 0, atEnd: false };
		checking = true;
		void tick().then(() => body?.scrollTo({ top: 0 }));
		try {
			const result = await backend().CheckRelease(r.tag, request);
			if (!requests.isCurrentCheck(request)) return;
			if (result.request_id !== request || result.tag !== r.tag || !result.check_id) {
				throw new Error('Ответ проверки не соответствует выбранному выпуску');
			}
			rememberReleaseScroll();
			check = result;
			checkSteps = check.steps;
			anchorRelease();
		} catch (e) {
			if (!requests.isCurrentCheck(request)) return;
			checkError = String(e);
			rememberReleaseScroll();
			checkSteps = checkSteps.map((s) => s.status === 'running' ? { ...s, status: 'failed' } : s);
			anchorRelease();
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
		rememberReleaseScroll();
		signing = true;
		wrong = false;
		signError = '';
		signSteps = [];
		anchorRelease();
		try {
			const res = await backend().SignRelease(checkId, tag, request, pass);
			if (!requests.isCurrentSign(originalCheckRequest, request)) return;
			rememberReleaseScroll();
			signSteps = res.steps ?? signSteps;
			if (res.wrong_password) {
				wrong = true;
				anchorRelease();
				return;
			}
			if (!res.ok) {
				signError = res.message ?? 'Подписать не получилось';
				anchorRelease();
				return;
			}
			pass = '';
			published = Boolean(res.published);
			releaseUrl = res.url ?? '';
			doneDetail = res.detail ?? '';
			// Уже подтверждённая подпись видна сразу, даже пока GitHub отвечает.
			++listRequest;
			listing = false;
			releases = releases.map((r) =>
				r.tag === tag ? { ...r, signed: true, draft: !res.published } : r
			);
			releaseDone = true;
			void list();
			anchorRelease();
		} catch (err) {
			if (!requests.isCurrentSign(originalCheckRequest, request)) return;
			signError = String(err);
			rememberReleaseScroll();
			signSteps = signSteps.map((s) => s.status === 'running' ? { ...s, status: 'failed' } : s);
			anchorRelease();
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

	const version = $derived(selectedRelease?.version ?? check?.tag.replace(/^[a-z]+-v/, '') ?? '');
	// Имя программы, файл выпуска и наличие самообновления приходят из Go:
	// окно не знает, сколько программ в экосистеме и как они называются.
	const productName = $derived(check?.name ?? selectedRelease?.name ?? '');
	const exeFile = $derived(check?.exe ?? selectedRelease?.exe ?? '');
	// Android-подпись ведёт себя иначе (свой ключ, публикация по releaseReady);
	// остальное окно получает из Go.
	const isAndroid = $derived((check ?? selectedRelease)?.product === 'android');
	const fileName = $derived(filePath.slice(Math.max(filePath.lastIndexOf('\\'), filePath.lastIndexOf('/')) + 1));
</script>

<svelte:window onfocus={() => { if (screen === 'list' && st && !listing) void list(); }} />

<div class="ins">
	<main class="ins-body" bind:this={body}>
		{#if screen === 'create'}
			<!-- Кадр 12а. -->
			<h2>Ключ подписи Wynd</h2>
			<p class="ins-sub">
				Этим ключом подписываются выпуски программ Wynd, которые обновляются сами. Установленная копия
				принимает обновление, только если новая версия подписана этим ключом. Ключ создаётся один раз.
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
				Сохраните файл ключа и пароль в надёжном месте. Без них уже розданные программы больше не обновить:
				новую версию они не примут.
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
			<p class="ins-sub">
				Ключи подписи
				<TextButton class="link under" onclick={() => (screen = 'keys')}>настроить</TextButton>
			</p>
			{#if !st.key_exists}
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
							<TextButton class="link under" onclick={() => void backend().OpenLink(r.url ?? '')}>{r.name} {r.version}</TextButton>
						{:else}
							{r.name} {r.version}
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
				<MarkRows class="mt-8">
					<MarkRow mark="↓">
						Подписать файл на этом компьютере
						{#snippet action()}
							<TextButton class="link under" onclick={() => void pickFile()}>выбрать</TextButton>
						{/snippet}
					</MarkRow>
				</MarkRows>
			{/if}
		{:else if screen === 'keys' && st}
			<!-- Ключи подписи: оба ключа в одном месте (решение владельца 2026-11-19). -->
			<h2>Ключи подписи</h2>
			<MarkRows>
				<MarkRow mark="·">
					Ключ программ под Windows
					<span class="ins-dim">— {st.key_exists ? st.key_path : 'файл не найден'}</span>
					{#snippet action()}
						<TextButton class="link under" onclick={() => void otherKey()}>другой</TextButton>
						{#if !st?.key_exists}<TextButton class="link under" onclick={() => (screen = 'create')}>создать</TextButton>{/if}
					{/snippet}
				</MarkRow>
				<MarkRow mark="·">
					Ключ программ под Android
					<span class="ins-dim">— {st?.key_store_exists ? st.key_store_path : 'не выбран'}</span>
					{#snippet action()}
						<TextButton class="link under" onclick={() => void chooseKeystore()}>{st?.key_store_exists ? 'другой' : 'выбрать'}</TextButton>
						{#if !st?.key_store_exists}<TextButton class="link under" onclick={goAndroidCreate}>создать</TextButton>{/if}
					{/snippet}
				</MarkRow>
			</MarkRows>
			<p class="ins-note">
				Выпуски всех программ Wynd подписаны: установленная копия принимает обновление, только если
				новая версия подписана тем же ключом. Файлы ключей зашифрованы паролем, пароль спрашивается
				при каждой подписи.
			</p>
			<FootBar>
				<Button variant="ghost" onclick={toList}>К выпускам</Button>
			</FootBar>
		{:else if screen === 'androidcreate'}
			<!-- Создание ключа Android (как 12а, для .p12): пароль, повтор, место хранения. -->
			<h2>Ключ программ под Android</h2>
			<p class="ins-sub">
				Этим ключом подписываются выпуски Wynd Android (APK). Установленное приложение принимает
				обновление, только если оно подписано тем же ключом. Ключ создаётся один раз.
			</p>
			<Label>Пароль ключа</Label>
			<Input type="password" autocomplete="off" bind:value={pass1} />
			<Label>Ещё раз</Label>
			<Input type="password" autocomplete="off" bind:value={pass2} />
			{#if mismatch}<p class="ins-note" role="alert">Пароли не совпадают</p>{/if}
			<p class="ins-note">Не короче 12 знаков. Ключ сохранится в файле .p12 этим паролем.</p>
			<Label>Где хранить ключ</Label>
			<FieldLine>
				<Input mono class="ins-grow" bind:value={akeyPath} />
				<Button variant="ghost" onclick={() => void chooseKeystorePlace()}>Выбрать</Button>
			</FieldLine>
			<Notice tone="bad">
				Сохраните файл ключа и пароль в надёжном месте. Без них уже установленные приложения не обновить:
				новую версию они не примут.
			</Notice>
			{#if createError}<Notice tone="bad" alert>{createError}</Notice>{/if}
			<FootBar>
				<Button variant={canCreateKeystore ? 'colored' : 'off'} loading={creating} loadingLabel="Создаём…" onclick={() => void createKeystore()}>Создать ключ</Button>
			</FootBar>
		{:else if screen === 'release'}
			<!-- Проверки, пароль, подпись и итог остаются на одном экране. -->
			<h2>{productName} {version}</h2>
			<StepList steps={checkSteps} onlink={(url) => void backend().OpenLink(url)} />
			{#if check?.extra?.length}
				<MarkRows class="mt-8">
					{#each check.extra as a (a.name)}
						<MarkRow mark={a.sum_ok && a.attest_ok ? '✓' : '!'} tone={a.sum_ok && a.attest_ok ? 'ok' : 'block'}>
							{a.name}
							<span class="ins-dim">— {mb(a.size)}</span>
						</MarkRow>
					{/each}
				</MarkRows>
			{/if}
			{#if checkError}<Notice tone="bad" alert>{checkError}</Notice>{/if}
			{#if check?.problem}<Notice tone="bad" alert>{check.problem}. Такой выпуск подписывать нельзя.</Notice>{/if}
			{#if signSteps.length}<StepList steps={signSteps} />{/if}
			{#if releaseDone}
				<Notice tone="note" title={published ? 'Выпуск подписан и опубликован' : 'Выпуск подписан'}>
					{#if check?.product === 'android'}
						{doneDetail}
					{:else}
						{check?.key_in_release ? 'Подпись проверена открытым ключом из репозитория.' : 'Подпись проверена открытой частью выбранного ключа.'}
						{check?.extra?.length
							? `Файл ${exeFile}.sig и подписи Linux-пакетов выложены`
							: `Файл ${exeFile}.sig выложен`}{published ? ' в выпуск на GitHub.' : ' в черновик.'}
						{published ? '' : `Он не опубликован: в whatsnew.md нет раздела ${version}.`}
						Временные файлы на этом компьютере удалены.
					{/if}
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
			{:else if isAndroid && st && !st.key_store_exists}
				<!-- Ключ Android создаётся здесь или выбирается: пароль и подпись появятся после. -->
				<Notice title="Ключ программ под Android не выбран">
					Подпись APK выполняется постоянным ключом. Создайте его или укажите существующий файл .p12;
					ключ хранится только у вас, без него выпуск подписать нельзя.
					<FootBar>
						<Button variant="colored" onclick={goAndroidCreate}>Создать</Button>
						<TextButton class="link under" onclick={() => void chooseKeystore()}>выбрать</TextButton>
					</FootBar>
				</Notice>
				<FootBar>
					<Button variant="ghost" disabled={checking || signing} onclick={toList}>К выпускам</Button>
				</FootBar>
			{:else}
				{#if check && check.self_update && !check.key_in_release}
					<Notice tone="note">В этой сборке нет открытого ключа. Подпись добавится к выпуску, но обновляться сама она не сможет. Для самообновления нужна новая сборка со встроенным открытым ключом.</Notice>
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
				{#if check?.publish_note}<Notice>{check.publish_note}</Notice>{/if}
				{#if signError}<Notice tone="bad" alert>{signError}</Notice>{/if}
				<FootBar>
					<Button variant={pass && check && !checking ? 'colored' : 'off'} disabled={checking || !check || !pass} loading={signing} loadingLabel="Подписываем…" onclick={() => void signRelease()}>
						{check?.publishes ?? Boolean(check?.draft && check?.notes) ? 'Подписать и опубликовать' : 'Подписать'}
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
			<p class="ins-note">Рядом с {filePath}. Сам exe не изменился: программа принимает обновление только вместе с этим файлом.</p>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().ShowInFolder(sigPath)}>Показать в папке</Button>
				<Button variant="ghost" onclick={toList}>К выпускам</Button>
			</FootBar>
		{/if}
	</main>
</div>

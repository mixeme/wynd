<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FieldLine from '$ui/workshop/FieldLine.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import { backend, type Check, type Release, type State } from './api';

	// Экраны (docs/visual/installer.html): 12а — ключа нет, 12б — выпуски,
	// 12в — черновик, 12г — подписан и опубликован, 12д — подписан файл;
	// file — пароль для файла на этом компьютере.
	type Screen = 'create' | 'list' | 'release' | 'done' | 'file' | 'filedone';
	let screen = $state<Screen>('list');
	let st = $state<State | undefined>();
	let keyNote = $state('');

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

	// 12в, 12г
	let check = $state<Check | undefined>();
	let checking = $state(false);
	let checkError = $state('');
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
		const p = await backend().ChooseKeyPlace(newKey);
		if (p) newKey = p;
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
		listing = true;
		listError = '';
		try {
			const res = await backend().Releases();
			releases = res.releases ?? [];
			if (!res.ok) listError = res.message ?? 'GitHub не ответил';
		} finally {
			listing = false;
		}
	}
	async function otherKey() {
		st = await backend().ChooseKey();
	}
	function toList() {
		check = undefined;
		pass = '';
		wrong = false;
		signError = '';
		keyNote = '';
		screen = 'list';
		void list();
	}

	async function openRelease(r: Release) {
		screen = 'release';
		check = undefined;
		checkError = '';
		pass = '';
		wrong = false;
		signError = '';
		checking = true;
		try {
			check = await backend().CheckRelease(r.tag);
		} catch (e) {
			checkError = String(e);
		} finally {
			checking = false;
		}
	}
	async function signRelease() {
		if (!check || !pass || signing) return;
		signing = true;
		wrong = false;
		signError = '';
		try {
			const res = await backend().SignRelease(pass);
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
			screen = 'done';
		} finally {
			signing = false;
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

	const version = $derived(check?.tag.replace(/^workshop-v/, '') ?? '');
	const fileName = $derived(filePath.slice(Math.max(filePath.lastIndexOf('\\'), filePath.lastIndexOf('/')) + 1));
</script>

<div class="ins">
	<main class="ins-body">
		{#if screen === 'create'}
			<!-- Кадр 12а. -->
			<h2>Ключ подписи Мастерской</h2>
			<p class="ins-sub">
				Мастерская обновляется сама, только если новая версия подписана этим ключом. Ключ создаётся один раз.
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
				<Button variant={canCreate ? 'colored' : 'off'} loading={creating} onclick={() => void create()}>Создать ключ</Button>
			</FootBar>
		{:else if screen === 'list' && st}
			<!-- Кадр 12б. -->
			{#if keyNote}<Notice tone="note">{keyNote}</Notice>{/if}
			<h2>Выпуски Мастерской</h2>
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
			{#if listing && !releases.length}
				<p class="ins-dim">Спрашиваем GitHub…</p>
			{/if}
			{#if listError}<Notice tone="bad" alert>{listError}</Notice>{/if}
			<MarkRows>
				{#each releases as r (r.tag)}
					<MarkRow mark={r.draft ? '●' : '✓'} tone={r.draft ? 'note' : 'ok'}>
						Мастерская {r.version}
						<span class="ins-dim"
							>— {r.draft
								? r.signed
									? 'черновик, подписан'
									: 'черновик, не подписан'
								: `опубликован ${day(r.published_at)}, ${r.signed ? 'с подписью' : 'без подписи'}`}</span
						>
						{#snippet action()}
							{#if r.draft && st?.key_exists}
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
			<!-- Кадр 12в: проверки до пароля. -->
			<h2>Мастерская {version}</h2>
			{#if checking}
				<p class="ins-dim">Скачиваем и проверяем…</p>
			{:else if checkError}
				<Notice tone="bad" alert>{checkError}</Notice>
				<FootBar><Button variant="ghost" onclick={toList}>Назад</Button></FootBar>
			{:else if check}
				<MarkRows>
					<MarkRow mark="✓" tone="ok">Скачан wynd-workshop.exe <span class="ins-dim">— {mb(check.size)}</span></MarkRow>
					<MarkRow mark={check.sum_ok ? '✓' : '!'} tone={check.sum_ok ? 'ok' : 'block'}>
						{check.sum_ok ? 'Сумма сходится с той, что в выпуске' : 'Сумма не сходится с той, что в выпуске'}
					</MarkRow>
					<MarkRow mark={check.attest_ok ? '✓' : '!'} tone={check.attest_ok ? 'ok' : 'block'}>
						{#if check.attest_ok}
							Собран GitHub Actions из тега {check.tag} <span class="ins-dim">— коммит {check.commit?.slice(0, 8)}</span>
						{:else}
							Нет удостоверения, что exe собран GitHub Actions из тега {check.tag}
						{/if}
					</MarkRow>
				</MarkRows>
				{#if check.problem}
					<Notice tone="bad" alert>{check.problem}. Такой exe подписывать нельзя.</Notice>
					<FootBar><Button variant="ghost" onclick={toList}>Назад</Button></FootBar>
				{:else}
					<Label>Пароль ключа</Label>
					<Input type="password" autocomplete="off" bind:value={pass} />
					{#if wrong}<p class="ins-note" role="alert">Пароль не подошёл</p>{/if}
					{#if check.notes}
						<Label>Что нового — из выпуска</Label>
						<Notice>
							{#each check.notes.split('\n') as line, i (i)}<div>{line}</div>{/each}
						</Notice>
					{:else}
						<Notice>В whatsnew.md нет раздела {version} — выпуск подпишем, но не опубликуем.</Notice>
					{/if}
					{#if signError}<Notice tone="bad" alert>{signError}</Notice>{/if}
					<FootBar>
						<Button variant={pass ? 'colored' : 'off'} loading={signing} onclick={() => void signRelease()}>
							{check.notes ? 'Подписать и опубликовать' : 'Подписать'}
						</Button>
						<Button variant="ghost" disabled={signing} onclick={toList}>Назад</Button>
					</FootBar>
				{/if}
			{/if}
		{:else if screen === 'done'}
			<!-- Кадр 12г. -->
			<h2>Мастерская {version} {published ? 'опубликована' : 'подписана'}</h2>
			<p class="ins-sub">Подпись проверена открытым ключом из репозитория.</p>
			<Notice>
				{published
					? 'Подпись wynd-workshop.exe.sig выложена в выпуск на GitHub, рядом с exe и его суммой.'
					: `Подпись wynd-workshop.exe.sig выложена в черновик выпуска на GitHub. Черновик не опубликован: в whatsnew.md нет раздела ${version}.`}
				На этом компьютере ничего не осталось: exe скачивался только для проверки.
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().OpenLink(releaseUrl)}>Открыть выпуск</Button>
				<Button variant="ghost" onclick={toList}>К выпускам</Button>
			</FootBar>
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
				<Button variant={pass ? 'colored' : 'off'} loading={signing} onclick={() => void signFile()}>Подписать</Button>
				<Button variant="ghost" disabled={signing} onclick={toList}>Назад</Button>
			</FootBar>
		{:else if screen === 'filedone'}
			<!-- Кадр 12д. -->
			<h2>Файл подписан</h2>
			<p class="ins-sub">Подпись проверена открытым ключом из репозитория.</p>
			<Label>Подпись</Label>
			<Input mono readonly value={sigPath} />
			<p class="ins-note">Рядом с {filePath}. Сам exe не изменился: Мастерская примет его только вместе с этим файлом.</p>
			<FootBar>
				<Button variant="colored" onclick={() => void backend().ShowInFolder(sigPath)}>Показать в папке</Button>
				<Button variant="ghost" onclick={toList}>К выпускам</Button>
			</FootBar>
		{/if}
	</main>
</div>

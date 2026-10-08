<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import {
		backend,
		type ConnectResult,
		type DomainCheck,
		type Finding,
		type InspectResult
	} from './api';

	// Окна установщика (docs/visual/installer.html): 1 — подключение, 1а —
	// отпечаток, 2 — осмотр, 2а — помеха. План, установка и итог — следующие срезы.
	type Screen = 'connect' | 'fingerprint' | 'inspect';
	const STEPS = ['Сервер', 'Осмотр', 'План', 'Установка', 'Готово'];

	let screen = $state<Screen>('connect');
	let host = $state('');
	let user = $state('root');
	let password = $state('');
	let useKeys = $state(false);
	let remember = $state(false);
	let saved = $state(false);
	let busy = $state(false);
	let failure = $state<ConnectResult | undefined>();
	let fingerprint = $state('');

	let inspection = $state<InspectResult | undefined>();
	let showRaw = $state(false);
	let domain = $state('');
	let domainCheck = $state<DomainCheck | undefined>();
	let checkingDomain = $state(false);

	const findings = $derived<Finding[]>(inspection?.report.findings ?? []);
	const blocks = $derived(findings.filter((f) => f.level === 'block'));
	const stepIndex = $derived(screen === 'inspect' ? 1 : 0);
	const stepBad = $derived(screen === 'inspect' && (blocks.length > 0 || inspection?.ok === false));
	const domainOk = $derived(domainCheck?.level === 'ok');

	// Есть сохранённый пароль — поле можно оставить пустым.
	let savedFor = '';
	async function refreshSaved() {
		const key = `${user.trim()}@${host.trim()}`;
		if (key === savedFor) return;
		savedFor = key;
		saved = host.trim() && user.trim() ? await backend().HasSavedPassword(host, user) : false;
		if (saved) remember = true;
	}

	function applyConnect(res: ConnectResult) {
		failure = undefined;
		if (res.status === 'connected') {
			password = '';
			screen = 'inspect';
			void inspect();
		} else if (res.status === 'unknown_host') {
			fingerprint = res.fingerprint ?? '';
			screen = 'fingerprint';
		} else {
			failure = res;
			screen = 'connect';
		}
	}

	async function connect() {
		if (busy) return;
		busy = true;
		try {
			applyConnect(
				await backend().Connect({ host, user, password, use_keys: useKeys, remember })
			);
		} finally {
			busy = false;
		}
	}

	async function confirmHost() {
		if (busy) return;
		busy = true;
		try {
			applyConnect(await backend().ConfirmHost());
		} finally {
			busy = false;
		}
	}

	async function inspect() {
		busy = true;
		inspection = undefined;
		domainCheck = undefined;
		try {
			inspection = await backend().Inspect();
		} finally {
			busy = false;
		}
	}

	async function checkDomain() {
		if (checkingDomain || !domain.trim()) return;
		checkingDomain = true;
		try {
			domainCheck = await backend().CheckDomain(domain);
			if (domainCheck.level === 'ok') domain = domainCheck.domain;
		} finally {
			checkingDomain = false;
		}
	}

	async function toConnect() {
		await backend().Disconnect();
		inspection = undefined;
		domainCheck = undefined;
		failure = undefined;
		screen = 'connect';
	}

	function mark(level: Finding['level']): string {
		return level === 'ok' ? '✓' : '!';
	}
</script>

<div class="ins">
	<nav class="ins-side" aria-label="Шаги установки">
		{#each STEPS as step, i (step)}
			<div
				class="ins-step"
				class:done={i < stepIndex}
				class:on={i === stepIndex && !stepBad}
				class:bad={i === stepIndex && stepBad}
				aria-current={i === stepIndex ? 'step' : undefined}
			>
				<i>{i < stepIndex ? '✓' : i === stepIndex && stepBad ? '!' : i + 1}</i>{step}
			</div>
		{/each}
	</nav>

	<main class="ins-body">
		{#if screen === 'connect'}
			<h2>Куда ставим Wynd</h2>
			<p class="ins-sub">Адрес и доступ к серверу вам прислал хостинг после покупки.</p>
			<form
				class="ins-form"
				onsubmit={(e) => {
					e.preventDefault();
					void connect();
				}}
			>
				<Label>Адрес сервера</Label>
				<Input
					mono
					autocomplete="off"
					spellcheck={false}
					placeholder="185.12.34.56"
					bind:value={host}
					onblur={() => void refreshSaved()}
				/>
				<Label>Вход</Label>
				<div class="ins-chips">
					<Chip selected={!useKeys} onclick={() => (useKeys = false)}>Пароль</Chip>
					<Chip selected={useKeys} onclick={() => (useKeys = true)}>Ключ с этого компьютера</Chip>
				</div>
				<div class="ins-pair">
					<div class="ins-user">
						<Label>Пользователь</Label>
						<Input
							mono
							autocomplete="off"
							spellcheck={false}
							bind:value={user}
							onblur={() => void refreshSaved()}
						/>
					</div>
					{#if !useKeys}
						<div class="ins-grow">
							<Label>Пароль от сервера</Label>
							<Input
								type="password"
								autocomplete="off"
								placeholder={saved ? 'сохранён — можно не вводить' : ''}
								bind:value={password}
							/>
						</div>
					{/if}
				</div>
				{#if useKeys}
					<p class="ins-note">Возьмём ключ из папки .ssh вашего профиля — тот же, что у программы ssh.</p>
				{:else}
					<label class="ins-check">
						<input type="checkbox" bind:checked={remember} />
						Запомнить пароль
						<span class="ins-dim">— в хранилище паролей этого компьютера</span>
					</label>
				{/if}
				{#if failure}
					<div class="ins-box bad" role="alert">
						<div class="ins-strong">{failure.message}</div>
						{#if failure.fingerprint}
							<div class="ins-mono">{failure.fingerprint}</div>
						{/if}
						{#if failure.advice}
							<div class="ins-dim">{failure.advice}</div>
						{/if}
					</div>
				{/if}
				<div class="ins-foot">
					<Button variant="colored" loading={busy} onclick={() => void connect()}>
						Подключиться
					</Button>
					{#if busy}
						<span class="ins-dim">Подключаемся…</span>
					{/if}
				</div>
			</form>
		{:else if screen === 'fingerprint'}
			<h2>Это ваш сервер?</h2>
			<p class="ins-sub">
				Подключаемся к {host.trim()} впервые. Сервер назвал свой отпечаток — сверьте его с письмом
				хостинга или панелью, если там он есть.
			</p>
			<div class="ins-box ins-mono">{fingerprint}</div>
			<p class="ins-sub mt">
				Если отпечатка негде взять — это нормально для нового сервера. Мы запомним его, и при
				следующем подключении подмену заметим.
			</p>
			<div class="ins-foot">
				<Button variant="colored" loading={busy} onclick={() => void confirmHost()}>
					Да, подключиться
				</Button>
				<Button variant="ghost" disabled={busy} onclick={() => void toConnect()}>Отмена</Button>
			</div>
		{:else if !inspection}
			<h2>Что на сервере</h2>
			<p class="ins-sub">Только смотрим — ничего не меняем.</p>
			<p class="ins-dim">Осматриваем…</p>
		{:else if !inspection.ok}
			<h2>Осмотр не удался</h2>
			<div class="ins-box bad" role="alert">
				<div class="ins-strong">{inspection.message}</div>
				{#if inspection.advice}
					<div class="ins-dim">{inspection.advice}</div>
				{/if}
			</div>
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void toConnect()}>Подключиться ещё раз</Button>
			</div>
		{:else if blocks.length}
			<h2>Пока ставить нельзя</h2>
			<p class="ins-sub">
				{blocks.length === 1 ? 'Мешает одно:' : 'Мешает вот что:'}
			</p>
			{#each blocks as block (block.id)}
				<div class="ins-box">
					<div class="ins-strong">{block.text}</div>
					{#if block.advice}
						<div class="ins-dim">{block.advice}</div>
					{/if}
				</div>
			{/each}
			<div class="ins-foot">
				<Button variant="colored" loading={busy} onclick={() => void inspect()}>
					Проверить снова
				</Button>
				<Button variant="ghost" disabled={busy} onclick={() => void toConnect()}>
					Другой сервер
				</Button>
				<TextButton class="link under" onclick={() => (showRaw = !showRaw)}>
					подробнее, что проверили
				</TextButton>
			</div>
			{#if showRaw}
				<pre class="ins-raw">{inspection.report.raw}</pre>
			{/if}
		{:else}
			<h2>Что на сервере</h2>
			<p class="ins-sub">Только смотрим — ничего не меняем.</p>
			<div class="ins-rows">
				{#each findings as f (f.id)}
					<div class="ins-row">
						<span class="ins-mark {f.level}">{mark(f.level)}</span>
						<div>
							{f.text}{#if f.plan}&nbsp;— <span class="ins-dim">{f.plan}</span>{/if}
						</div>
					</div>
				{/each}
			</div>
			<Label>Адрес, по которому будут заходить</Label>
			<form
				class="ins-domain"
				onsubmit={(e) => {
					e.preventDefault();
					void checkDomain();
				}}
			>
				<Input
					mono
					class="ins-grow"
					autocomplete="off"
					spellcheck={false}
					placeholder="family.example.ru"
					bind:value={domain}
					oninput={() => (domainCheck = undefined)}
				/>
				{#if domainOk}
					<span class="ins-mark ok wide">✓ {domainCheck?.text}</span>
				{:else}
					<Button variant="ghost" loading={checkingDomain} onclick={() => void checkDomain()}>
						{domainCheck ? 'Проверить снова' : 'Проверить'}
					</Button>
				{/if}
			</form>
			{#if domainCheck && !domainOk}
				<div class="ins-box" role="alert">
					<div class="ins-strong">{domainCheck.text}</div>
					{#if domainCheck.advice}
						<div class="ins-dim">{domainCheck.advice}</div>
					{/if}
				</div>
			{/if}
			<div class="ins-foot">
				<Button variant="off" onclick={() => {}}>Дальше — план</Button>
				<Button variant="ghost" onclick={() => void toConnect()}>Другой сервер</Button>
				<span class="ins-dim">
					{domainOk ? 'План и установка — в следующей версии установщика' : 'Сначала укажите адрес сайта'}
				</span>
				<TextButton class="link under" onclick={() => (showRaw = !showRaw)}>
					подробнее, что проверили
				</TextButton>
			</div>
			{#if showRaw}
				<pre class="ins-raw">{inspection.report.raw}</pre>
			{/if}
		{/if}
	</main>
</div>

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
		type InspectResult,
		type LogEntry,
		type Plan,
		type Progress,
		type StepState
	} from './api';

	// Окна установщика (docs/visual/installer.html): 1 — подключение, 1а —
	// отпечаток, 2 — осмотр, 2а — помеха, 3 — план, 4 — установка, 4а — шаг не
	// прошёл, 5 — готово. Пульт сервера (6) — следующий срез.
	type Screen = 'connect' | 'fingerprint' | 'inspect' | 'plan' | 'install' | 'done';
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
	let plan = $state<Plan | undefined>();
	let planning = $state(false);
	let progress = $state<Progress | undefined>();
	let showLog = $state(false);
	let askRollback = $state(false);
	let dropData = $state(false);
	let rolling = $state(false);
	let rollbackNote = $state('');
	let rollbackError = $state('');
	let rollbackLog = $state<LogEntry[]>([]);
	let copied = $state(false);

	const SCREEN_STEP: Record<Screen, number> = {
		connect: 0,
		fingerprint: 0,
		inspect: 1,
		plan: 2,
		install: 3,
		done: 4
	};
	const stepIndex = $derived(SCREEN_STEP[screen]);
	const stepBad = $derived(
		(screen === 'inspect' && (blocks.length > 0 || inspection?.ok === false)) ||
			(screen === 'plan' && plan?.ok === false) ||
			(screen === 'install' && Boolean(progress?.failure))
	);
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
		stopPoll();
		plan = undefined;
		progress = undefined;
		await backend().Disconnect();
		inspection = undefined;
		domainCheck = undefined;
		failure = undefined;
		screen = 'connect';
	}

	function mark(level: Finding['level']): string {
		return level === 'ok' ? '✓' : '!';
	}

	async function toPlan() {
		if (planning || !domainOk) return;
		planning = true;
		rollbackNote = '';
		try {
			plan = await backend().MakePlan(domain);
			progress = undefined;
			screen = 'plan';
		} finally {
			planning = false;
		}
	}

	// Собрать Wynd на сервере или скачать готовым (окна 3а–3в).
	let choosing = $state(false);
	async function chooseBuild(on: boolean) {
		if (choosing) return;
		choosing = true;
		try {
			const next = await backend().ChooseBuild(on);
			if (next.ok) plan = next;
		} finally {
			choosing = false;
		}
	}

	// Установка идёт в Go сама; окно только спрашивает, как дела.
	let poll: ReturnType<typeof setInterval> | undefined;
	function stopPoll() {
		clearInterval(poll);
		poll = undefined;
	}
	function applyProgress(next: Progress) {
		progress = next;
		if (next.running) return;
		stopPoll();
		if (next.done) screen = 'done';
	}
	async function install() {
		askRollback = false;
		rollbackError = '';
		rollbackLog = [];
		screen = 'install';
		applyProgress(await backend().StartInstall());
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(applyProgress), 500);
		}
	}

	async function rollback() {
		if (rolling) return;
		rolling = true;
		rollbackError = '';
		try {
			const res = await backend().Rollback(!dropData);
			if (res.ok) {
				askRollback = false;
				rollbackLog = [];
				progress = undefined;
				if (res.report) inspection = { ok: true, report: res.report };
				if (res.plan) plan = res.plan;
				rollbackNote = dropData
					? 'Установка откачена: Wynd и его данные с сервера убраны, остался только Docker.'
					: 'Установка откачена: Wynd с сервера убран, его данные оставлены, Docker остался.';
				screen = 'plan';
			} else {
				rollbackError = [res.message, res.advice].filter(Boolean).join('. ');
				// Что откат успел и на чём встал — в тех же «подробностях».
				rollbackLog = res.log ?? [];
			}
		} finally {
			rolling = false;
		}
	}

	async function copyLink() {
		if (!progress?.link) return;
		copied = await backend().CopyText(progress.link);
		setTimeout(() => (copied = false), 2000);
	}

	function stepMark(step: StepState): string {
		if (step.status === 'done') return '✓';
		if (step.status === 'failed') return '!';
		return step.status === 'running' ? '…' : '○';
	}

	const logText = $derived(
		[...(progress?.log ?? []), ...rollbackLog].map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd()).join('\n\n')
	);
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
		{:else if screen === 'plan' && plan && !plan.ok}
			<h2>Пока ставить нельзя</h2>
			<div class="ins-box" role="alert">
				<div class="ins-strong">{plan.message}</div>
				{#if plan.advice}
					<div class="ins-dim">{plan.advice}</div>
				{/if}
			</div>
			<div class="ins-foot">
				<Button variant="ghost" onclick={() => (screen = 'inspect')}>Назад</Button>
			</div>
		{:else if screen === 'plan' && plan}
			<h2>Что сделаем</h2>
			<p class="ins-sub">До кнопки «Установить» на сервере ничего не меняется.</p>
			{#if rollbackNote}
				<div class="ins-box">{rollbackNote}</div>
			{/if}
			<div class="ins-sec">Поставим</div>
			{#each plan.install ?? [] as item (item)}
				<div class="ins-row"><span class="ins-mark ok">+</span>{item}</div>
			{/each}
			{#if plan.change?.length}
				<div class="ins-sec">Изменим — с резервной копией</div>
				{#each plan.change as item (item)}
					<div class="ins-row"><span class="ins-mark note">~</span>{item}</div>
				{/each}
			{/if}
			{#if plan.keep?.length}
				<div class="ins-sec">Не тронем</div>
				{#each plan.keep as item (item)}
					<div class="ins-row"><span class="ins-mark">·</span>{item}</div>
				{/each}
			{/if}
			{#if plan.unpublished}
				<p class="ins-sub">Эту версию ещё нельзя скачать готовой — сервер соберёт Wynd сам.</p>
			{:else if plan.canBuild}
				<p class="ins-sub">
					{plan.build ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
					<TextButton class="link under" onclick={() => void chooseBuild(!plan?.build)}>
						{plan.build ? 'Скачать готовый' : 'Собрать самому'}
					</TextButton>
				</p>
			{/if}
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void install()}>Установить</Button>
				<Button variant="ghost" onclick={() => (screen = 'inspect')}>Назад</Button>
				<span class="ins-dim">{plan.duration}</span>
			</div>
		{:else if screen === 'install' && progress?.failure}
			<h2>{progress.failure.message}</h2>
			<p class="ins-sub">{progress.failure.advice}</p>
			{#if progress.steps?.some((s) => s.status === 'done')}
				<div class="ins-box">
					Сделанное раньше осталось на месте — повтор начнётся с этого шага.
				</div>
			{/if}
			{#if askRollback}
				<div class="ins-box bad">
					<div class="ins-strong">Убрать Wynd с сервера?</div>
					<div class="ins-dim">
						Уберём Wynd и его папку /opt/wynd. Docker останется: сам по себе он ничего не меняет.
					</div>
					<label class="ins-check">
						<input type="checkbox" bind:checked={dropData} />
						Удалить и данные Wynd — записи, фотографии, учётные записи
					</label>
					{#if rollbackError}
						<div class="ins-dim" role="alert">{rollbackError}</div>
						<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
					{/if}
					<div class="ins-foot">
						<Button variant="colored" loading={rolling} onclick={() => void rollback()}>
							Да, откатить
						</Button>
						<Button variant="ghost" disabled={rolling} onclick={() => (askRollback = false)}>
							Отмена
						</Button>
					</div>
				</div>
			{:else}
				<div class="ins-foot">
					<Button variant="colored" onclick={() => void install()}>Повторить</Button>
					<Button variant="ghost" onclick={() => (askRollback = true)}>Откатить установку</Button>
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
				</div>
			{/if}
			{#if showLog}
				<pre class="ins-raw">{logText}</pre>
			{/if}
		{:else if screen === 'install'}
			<h2>Ставим</h2>
			<div class="ins-rows">
				{#each progress?.steps ?? [] as step (step.id)}
					<div class="ins-row" class:ins-dim={step.status === 'pending'}>
						<span class="ins-mark {step.status === 'done' ? 'ok' : ''}">{stepMark(step)}</span>
						<div>
							{step.title}{#if step.note}&nbsp;<span class="ins-dim">— {step.note}</span>{/if}
						</div>
					</div>
				{/each}
			</div>
			<div class="ins-foot">
				<TextButton class="link under" onclick={() => (showLog = !showLog)}>
					показать, что выполняется
				</TextButton>
				<span class="ins-dim">Окно можно свернуть</span>
			</div>
			{#if showLog}
				<pre class="ins-raw">{logText}</pre>
			{/if}
		{:else if screen === 'done' && progress}
			<h2>Wynd работает</h2>
			<p class="ins-sub">Проверка прошла: сайт открывается, сертификат действует.</p>
			{#if progress.link}
				<Label>Ссылка первого запуска — для вас, одна</Label>
				<div class="ins-box ins-mono">{progress.link}</div>
				<p class="ins-sub mt">
					По ней вы станете администратором сервера и создадите первый круг. Никому её не
					передавайте.
				</p>
				<div class="ins-foot">
					<Button variant="colored" onclick={() => void backend().OpenLink(progress?.link ?? '')}>
						Открыть в браузере
					</Button>
					<Button variant="ghost" onclick={() => void copyLink()}>
						{copied ? 'Скопировано' : 'Скопировать'}
					</Button>
				</div>
			{:else}
				<div class="ins-box">
					Этот сервер уже настроен: администратор у него есть, ссылки первого запуска нет.
					Откройте сайт и войдите как обычно.
				</div>
				<div class="ins-foot">
					<Button variant="colored" onclick={() => void backend().OpenLink(progress?.site ?? '')}>
						Открыть {plan?.domain}
					</Button>
				</div>
			{/if}
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
				<Button
					variant={domainOk ? 'colored' : 'off'}
					loading={planning}
					onclick={() => void toPlan()}
				>
					Дальше — план
				</Button>
				<Button variant="ghost" onclick={() => void toConnect()}>Другой сервер</Button>
				{#if !domainOk}
					<span class="ins-dim">Сначала укажите адрес сайта</span>
				{/if}
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

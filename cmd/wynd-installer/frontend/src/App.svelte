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
		type LocalBackupPlan,
		type MovePlan,
		type Plan,
		type Progress,
		type SelfUpdate,
		type ServerPage,
		type StepState
	} from './api';
	import { rawSections } from './raw';
	import Archives from './Archives.svelte';

	// Вкладки «Серверы» и «Архивы» (владелец, 2026-10-24: «Лучше отдельной вкладкой»).
	let tab = $state<'servers' | 'archives'>('servers');

	// Новая Мастерская (10а–10в): спрашиваем при запуске; «✕» прячет только
	// полосу — тихая строка внизу остаётся, пока не обновились (10а2).
	let selfUp = $state<SelfUpdate | undefined>();
	let selfHidden = $state(false);
	let selfOpen = $state(false);
	let selfBusy = $state(false);
	let selfBad = $state(false);
	let selfError = $state('');
	let ownVersion = $state('');
	void backend()
		.CheckSelfUpdate()
		.then((u) => (selfUp = u.available ? u : undefined))
		.catch(() => undefined);
	void backend()
		.Version()
		.then((v) => (ownVersion = v))
		.catch(() => undefined);
	async function applySelf() {
		if (selfBusy) return;
		selfBusy = true;
		selfError = '';
		try {
			const res = await backend().ApplySelfUpdate();
			if (res === 'signature') selfBad = true;
			else if (res) selfError = res;
		} finally {
			selfBusy = false;
		}
	}

	// Окна установщика (docs/visual/installer.html): 1 — подключение, 1а —
	// отпечаток, 2 — осмотр, 2а — помеха, 3 — план, 4 — установка, 4а — шаг не
	// прошёл, 5 — готово. Пульт сервера (6) — следующий срез.
	type Screen =
		| 'connect'
		| 'fingerprint'
		| 'inspect'
		| 'plan'
		| 'install'
		| 'done'
		| 'server'
		| 'uplan'
		| 'update'
		| 'udone'
		| 'bplan'
		| 'backup'
		| 'bdone'
		| 'mconnect'
		| 'mfinger'
		| 'mplan'
		| 'move'
		| 'mdone';
	const INSTALL_STEPS = ['Сервер', 'Осмотр', 'План', 'Установка', 'Готово'];
	// Обновление со страницы сервера (кадры 7б–7д).
	const UPDATE_STEPS = ['Сервер', 'План', 'Обновление', 'Готово'];
	// Бэкап на этот компьютер (кадры 8а–8г).
	const BACKUP_STEPS = ['Сервер', 'Бэкап', 'Готово'];
	// Переезд на другой сервер (кадры 9а–9г).
	const MOVE_STEPS = ['Новый сервер', 'План', 'Переезд', 'Готово'];

	let screen = $state<Screen>('connect');
	let host = $state('');
	let user = $state('root');
	let password = $state('');
	let useKeys = $state(false);
	let remember = $state(false);
	let makeKey = $state(false);
	let keyNote = $state('');
	let saved = $state(false);
	let busy = $state(false);
	let failure = $state<ConnectResult | undefined>();
	let fingerprint = $state('');

	let inspection = $state<InspectResult | undefined>();
	let showRaw = $state(false);
	let copiedRaw = $state(false);
	let domain = $state('');
	let domainCheck = $state<DomainCheck | undefined>();
	let checkingDomain = $state(false);

	const findings = $derived<Finding[]>(inspection?.report.findings ?? []);
	const blocks = $derived(findings.filter((f) => f.level === 'block'));
	const rawParts = $derived(
		inspection
			? rawSections(
					inspection.report.raw,
					blocks.map((b) => b.id)
				)
			: []
	);
	let plan = $state<Plan | undefined>();
	let planning = $state(false);
	let progress = $state<Progress | undefined>();
	// Страница сервера, где Wynd уже стоит (пульт).
	let page = $state<ServerPage | undefined>();
	// План переезда (9б).
	let mplan = $state<MovePlan | undefined>();
	let showLog = $state(false);
	let askRollback = $state(false);
	let dropData = $state(false);
	let rolling = $state(false);
	let rollbackNote = $state('');
	let rollbackError = $state('');
	let rollbackLog = $state<LogEntry[]>([]);
	let copied = $state(false);
	let copiedLine = $state('');

	const SCREEN_STEP: Record<Screen, number> = {
		connect: 0,
		fingerprint: 0,
		inspect: 1,
		plan: 2,
		install: 3,
		done: 4,
		server: 0,
		uplan: 1,
		update: 2,
		udone: 3,
		bplan: 1,
		backup: 1,
		bdone: 2,
		mconnect: 0,
		mfinger: 0,
		mplan: 1,
		move: 2,
		mdone: 3
	};
	const updating = $derived(screen === 'uplan' || screen === 'update' || screen === 'udone');
	const backingUp = $derived(screen === 'bplan' || screen === 'backup' || screen === 'bdone');
	const moving = $derived(['mconnect', 'mfinger', 'mplan', 'move', 'mdone'].includes(screen));
	const STEPS = $derived(
		updating ? UPDATE_STEPS : backingUp ? BACKUP_STEPS : moving ? MOVE_STEPS : INSTALL_STEPS
	);
	const stepIndex = $derived(SCREEN_STEP[screen]);
	const stepBad = $derived(
		(screen === 'inspect' && (blocks.length > 0 || inspection?.ok === false)) ||
			(screen === 'plan' && plan?.ok === false) ||
			((screen === 'install' || screen === 'update' || screen === 'backup' || screen === 'move') &&
				Boolean(progress?.failure)) ||
			(screen === 'mplan' && mplan?.ok === false) ||
			(screen === 'uplan' && noRoom(page))
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
			keyNote = res.key_note ?? '';
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
				await backend().Connect({ host, user, password, use_keys: useKeys, remember, make_key: makeKey })
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
			// Wynd здесь уже стоит — сразу страница сервера (владелец, 2026-10-24: 7а).
			if (inspection.ok && inspection.report.findings?.some((f) => f.id === 'wynd')) {
				const p = await backend().ServerPage();
				if (p.ok && p.state.installed && p.state.container) {
					page = p;
					screen = 'server';
				}
			}
		} finally {
			busy = false;
		}
	}

	// Пульт (кадры 7а–7ж2).
	const CHANGES = 'https://github.com/mixeme/wynd/blob/main/docs/CHANGELOG.md';
	let askStop = $state(false);
	let svcBusy = $state('');
	let svcError = $state('');
	let svcLog = $state<LogEntry[]>([]);
	let stopForCopy = $state(true);
	let startAfter = $state(true);
	let buildUpdate = $state(false);
	let returning = $state(false);
	// С какой версии и на какую идёт обновление: окно помнит, пока оно не кончилось.
	let upFrom = $state('');
	let upTo = $state('');

	const st = $derived(page?.state);
	// Кадр 7б2: места не хватает даже на копию базы.
	function noRoom(p: ServerPage | undefined): boolean {
		return Boolean(p && p.state.free_bytes > 0 && p.state.free_bytes < p.copy_bytes + 64 * 1024 * 1024);
	}
	const uplanNoRoom = $derived(noRoom(page));

	function gb(bytes: number): string {
		if (bytes >= 1024 ** 3) return (bytes / 1024 ** 3).toFixed(1).replace('.', ',') + ' ГБ';
		return Math.max(1, Math.round(bytes / 1024 ** 2)) + ' МБ';
	}
	function day(iso: string): string {
		const d = new Date(iso);
		return Number.isNaN(d.getTime())
			? iso
			: d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
	}

	async function refreshPage() {
		page = await backend().ServerPage();
	}

	// «К серверу» после обновления, бэкапа или переезда: состояние сервера
	// могло поменяться (обрыв посреди обновления — кадр 7е), читаем заново.
	async function toServer() {
		screen = 'server';
		svcError = '';
		const p = await backend().ServerPage();
		if (p.ok) page = p;
		else svcError = [p.message, p.advice].filter(Boolean).join('. ');
	}

	function dayTime(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return `${day(iso)} в ${d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}`;
	}

	async function service(action: 'start' | 'stop' | 'restart') {
		if (svcBusy) return;
		svcBusy = action;
		svcError = '';
		try {
			const res = await backend().Service(action);
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			askStop = false;
		} finally {
			svcBusy = '';
		}
	}

	function toUpdatePlan() {
		if (!page) return;
		stopForCopy = true;
		startAfter = !page.state.stopped_at;
		buildUpdate = false;
		screen = 'uplan';
	}

	async function startUpdate() {
		if (!page) return;
		upFrom = page.state.update?.from ?? page.state.version ?? '';
		upTo = page.state.update?.to ?? page.latest ?? '';
		screen = 'update';
		applyUpdate(
			await backend().StartUpdate({
				stop_for_copy: stopForCopy || !page.live_copy,
				start_after: page.state.running || startAfter,
				build: buildUpdate
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(applyUpdate), 500);
		}
	}
	function applyUpdate(next: Progress) {
		progress = next;
		if (next.running) return;
		stopPoll();
		if (next.done) {
			screen = 'udone';
			void refreshPage();
		}
	}

	// Бэкап на этот компьютер (8а–8г).
	let bFolder = $state('');
	let bSite = $state<'keep' | 'pause' | 'stop'>('keep');
	let bEncrypt = $state(false);
	let bPass = $state('');
	let bPass2 = $state('');
	let bPlan = $state<LocalBackupPlan | undefined>();

	async function toBackupPlan() {
		if (!page) return;
		const key = 'wynd-workshop-backup:' + (page.state.domain ?? '');
		let saved = '';
		try {
			saved = localStorage.getItem(key) ?? '';
		} catch {
			saved = '';
		}
		bFolder = saved || (await backend().DefaultBackupFolder(page.state.domain ?? 'wynd'));
		bSite = page.live_copy ? 'keep' : 'pause';
		bPass = bPass2 = '';
		screen = 'bplan';
		await planBackup();
	}
	async function planBackup() {
		bPlan = await backend().PlanBackup(bFolder);
		bEncrypt = Boolean(bPlan.encrypted);
	}
	async function chooseBackupFolder() {
		const p = await backend().PickFolder();
		if (p) {
			bFolder = p;
			await planBackup();
		}
	}
	const passOk = $derived(!bEncrypt || (bPass.length > 0 && (bPlan?.encrypted || bPass === bPass2)));
	async function startBackup() {
		if (!page || !bFolder || !passOk) return;
		try {
			localStorage.setItem('wynd-workshop-backup:' + (page.state.domain ?? ''), bFolder);
		} catch {
			/* без памяти папки — спросим её в следующий раз */
		}
		screen = 'backup';
		const apply = (next: Progress) => {
			progress = next;
			if (next.running) return;
			stopPoll();
			if (next.done) {
				screen = 'bdone';
				void refreshPage();
			}
		};
		apply(
			await backend().StartBackup({
				folder: bFolder,
				site: page.state.running ? bSite : 'keep',
				password: bEncrypt ? bPass : ''
			})
		);
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(apply), 500);
		}
	}

	// Переезд (9а–9г).
	let mHost = $state('');
	let mUser = $state('root');
	let mPass = $state('');
	let mUseKeys = $state(false);
	let mMakeKey = $state(false);
	let mFail = $state<ConnectResult | undefined>();
	let mFinger = $state('');
	let mRemoveOld = $state(false);
	let mBusy = $state(false);

	function toMove() {
		mFail = undefined;
		mPass = '';
		screen = 'mconnect';
	}
	async function applyMove(res: ConnectResult) {
		mFail = undefined;
		if (res.status === 'connected') {
			mPass = '';
			keyNote = res.key_note ?? '';
			screen = 'mplan';
			mplan = undefined;
			mplan = await backend().MovePlan();
		} else if (res.status === 'unknown_host') {
			mFinger = res.fingerprint ?? '';
			screen = 'mfinger';
		} else {
			mFail = res;
			screen = 'mconnect';
		}
	}
	async function moveConnect() {
		if (mBusy) return;
		mBusy = true;
		try {
			await applyMove(
				await backend().MoveConnect({ host: mHost, user: mUser, password: mPass, use_keys: mUseKeys, remember: false, make_key: mMakeKey })
			);
		} finally {
			mBusy = false;
		}
	}
	async function moveConfirm() {
		if (mBusy) return;
		mBusy = true;
		try {
			await applyMove(await backend().MoveConfirmHost());
		} finally {
			mBusy = false;
		}
	}
	async function startMove() {
		screen = 'move';
		const apply = (next: Progress) => {
			progress = next;
			if (next.running) return;
			stopPoll();
			if (next.done) screen = 'mdone';
		};
		apply(await backend().StartMove(mRemoveOld));
		if (progress?.running && !poll) {
			poll = setInterval(() => void backend().InstallProgress().then(apply), 500);
		}
	}
	async function cancelMove() {
		if (mBusy) return;
		mBusy = true;
		try {
			stopPoll();
			const res = await backend().CancelMove();
			page = res.page;
			svcLog = res.log ?? [];
			if (!res.ok) svcError = [res.message, res.advice].filter(Boolean).join('. ');
			progress = undefined;
			screen = 'server';
		} finally {
			mBusy = false;
		}
	}
	async function adoptNew() {
		page = await backend().MoveAdopt();
		screen = 'server';
	}

	async function returnPrevious() {
		if (returning) return;
		returning = true;
		svcError = '';
		try {
			const res = await backend().ReturnPrevious();
			page = res.page;
			svcLog = res.log ?? [];
			if (res.ok) {
				progress = undefined;
				screen = 'server';
			} else {
				svcError = [res.message, res.advice].filter(Boolean).join('. ');
			}
		} finally {
			returning = false;
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

	// Данные удаляли — сертификат отзывали: сказать, вышло ли.
	function certNote(cert: 'revoked' | 'failed' | undefined): string {
		if (cert === 'revoked') return ' Сертификат сайта отозван.';
		if (cert === 'failed') {
			return ' Отозвать сертификат сайта не получилось: он удалён вместе с ключом и перестанет действовать сам, когда выйдет его срок.';
		}
		return '';
	}

	// Чужой веб-сервер, за которым стоит Wynd: «nginx», «Apache», «Caddy» или пусто — свой Caddy.
	const proxyName = $derived(
({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[inspection?.report?.proxy ?? ''] ?? ''
	);

	async function rollback() {
		if (rolling) return;
		rolling = true;
		rollbackError = '';
		// Осмотр после отката тот же веб-сервер найдёт снова; имя берём заранее.
		const stays = proxyName ? `Docker и ${proxyName} остались.` : 'Docker остался.';
		// Кадр 5д: что именно помешало, знаем по проверке до удаления и после него.
		const stale = (res: { stale?: string }) => {
			if (!res.stale || !proxyName) return '';
			if (res.stale === 'before') {
				return ` Но ${proxyName} это изменение ещё не принял: в его настройке ошибка, и она была там ещё до удаления — о ней ниже. Когда её исправят, сайт Wynd пропадёт из ${proxyName} сам.`;
			}
			if (res.stale === 'after') {
				return ` Но ${proxyName} это изменение не принял: без файла сайта Wynd его настройка перестала проходить проверку — похоже, на этот файл ссылается чужая настройка. Что не так, сказано ниже.`;
			}
			return ` Но перечитать настройку ${proxyName} не удалось, хотя ошибок в ней нет. Сайт Wynd пропадёт из ${proxyName}, когда его попросят перечитать настройку или перезапустят.`;
		};
		try {
			const res = await backend().Rollback(!dropData);
			if (res.ok) {
				const failed = screen === 'install';
				askRollback = false;
				rollbackLog = [];
				progress = undefined;
				if (res.report) inspection = { ok: true, report: res.report };
				if (res.plan) plan = res.plan;
				if (failed) {
					rollbackNote =
						(dropData
							? 'Установка откачена: Wynd и его данные с сервера убраны. '
							: 'Установка откачена: Wynd с сервера убран, его данные оставлены. ') +
						stays +
						certNote(res.cert) +
						stale(res);
					screen = 'plan';
				} else {
					// Убрали работавший Wynd (окна 2б, 5а): назад к осмотру, сервер уже без него.
					rollbackNote =
						(dropData
							? 'Wynd, его данные и сертификат с сервера убраны. '
							: 'Wynd с сервера убран, его данные и сертификат оставлены. ') +
						stays +
						certNote(res.cert) +
						stale(res);
					screen = 'inspect';
				}
			} else {
				rollbackError = [res.message, res.advice].filter(Boolean).join('. ');
				// Что откат успел и на чём встал — в тех же «подробностях».
				rollbackLog = res.log ?? [];
			}
		} finally {
			rolling = false;
		}
	}

	async function copyRaw() {
		copiedRaw = await backend().CopyText(inspection?.report.raw ?? '');
		setTimeout(() => (copiedRaw = false), 2000);
	}

	async function copyLine(line: string) {
		if (!(await backend().CopyText(line))) return;
		copiedLine = line;
		setTimeout(() => (copiedLine = ''), 2000);
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
		[...(progress?.log ?? []), ...rollbackLog, ...svcLog].map((e) => ('$ ' + e.command + '\n' + e.output).trimEnd()).join('\n\n')
	);
</script>

{#snippet removeBox(yes: string, site: string)}
	<div class="ins-box bad">
		<div class="ins-strong">Убрать Wynd с сервера?</div>
		<div class="ins-dim">
			{#if site}Сайт {site} перестанет открываться.{/if}
			{#if proxyName}
				Уберём контейнер Wynd и его сайт из {proxyName}. Docker и {proxyName} останутся.
			{:else}
				Уберём контейнеры Wynd и Caddy. Docker останется.
			{/if}
		</div>
		<label class="ins-check">
			<input type="checkbox" bind:checked={dropData} />
			Удалить данные Wynd — записи, фотографии, учётные записи — и сертификат сайта
		</label>
		<div class="ins-dim">
			{#if proxyName}
				Без галочки папка /opt/wynd и сертификат останутся.
			{:else}
				Без галочки папка /opt/wynd останется как есть.
			{/if}
			Поставите Wynd снова — всё будет на месте.
		</div>
		{#if rollbackError}
			<div class="ins-dim" role="alert">{rollbackError}</div>
			<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
		{/if}
		<div class="ins-foot">
			<Button variant="colored" loading={rolling} onclick={() => void rollback()}>{yes}</Button>
			<Button variant="ghost" disabled={rolling} onclick={() => (askRollback = false)}>Отмена</Button>
		</div>
	</div>
{/snippet}

<div class="ins-tabs" role="tablist">
	<button type="button" role="tab" class:on={tab === 'servers'} aria-selected={tab === 'servers'} onclick={() => (tab = 'servers')}>
		Серверы
	</button>
	<button type="button" role="tab" class:on={tab === 'archives'} aria-selected={tab === 'archives'} onclick={() => (tab = 'archives')}>
		Архивы
	</button>
</div>

{#if selfOpen && selfUp}
	<!-- Кадры 10б, 10в. -->
	<div class="ins">
		<main class="ins-body">
			{#if selfBad}
				<h2>Эту версию Мастерская не поставит</h2>
				<div class="ins-box">
					<div class="ins-strong">Мастерская {selfUp.version} не подписана ключом Wynd</div>
					<div class="ins-dim">
						Файл мог испортиться при скачивании — или его подменили. Ставить такую версию опасно: у
						Мастерской есть доступ к вашему серверу. Текущая Мастерская {ownVersion} работает как прежде.
					</div>
				</div>
				<div class="ins-foot">
					<Button variant="colored" loading={selfBusy} onclick={() => void applySelf()}>Попробовать ещё раз</Button>
					<Button variant="ghost" onclick={() => (selfOpen = false)}>Закрыть</Button>
					<TextButton class="link under ins-push" onclick={() => void backend().ReleasesPage()}>страница выпусков</TextButton>
				</div>
			{:else}
				<h2>Мастерская {selfUp.version}</h2>
				{#if selfUp.notes?.length}
					<p class="ins-sub">Что нового:</p>
					<div class="ins-rows">
						{#each selfUp.notes as n (n)}<div class="ins-row"><span class="ins-mark">·</span><div>{n}</div></div>{/each}
					</div>
				{/if}
				<div class="ins-rows">
					<div class="ins-row">
						<span class="ins-mark ok">✓</span>
						<div>Подписана ключом Wynd <span class="ins-dim">— выложил тот, кто делает Wynd, файл не подменён</span></div>
					</div>
					<div class="ins-row">
						<span class="ins-mark ok">✓</span>
						<div>
							Собрана из открытого кода тега workshop-v{selfUp.version} в GitHub Actions
							<TextButton class="link under" onclick={() => void backend().OpenLink(selfUp?.attestation ?? '')}>проверить</TextButton>
						</div>
					</div>
				</div>
				{#if selfError}<div class="ins-box bad" role="alert">{selfError}</div>{/if}
				<div class="ins-foot">
					<Button
						variant={progress?.running ? 'off' : 'colored'}
						loading={selfBusy}
						onclick={() => void (progress?.running ? undefined : applySelf())}>Обновить и перезапустить</Button
					>
					<Button variant="ghost" onclick={() => (selfOpen = false)}>Не сейчас</Button>
					{#if progress?.running}<span class="ins-dim">Сначала дождитесь окончания</span>{/if}
				</div>
			{/if}
		</main>
	</div>
{:else if tab === 'archives'}
	<Archives />
{:else}
{#if selfUp && !selfHidden}
	<!-- Кадр 10а. -->
	<div class="ins-selfbar">
		<span>Вышла Мастерская {selfUp.version}</span>
		<Button variant="ghost" onclick={() => (selfOpen = true)}>Обновить</Button>
		<button class="ins-x" type="button" aria-label="Скрыть" onclick={() => (selfHidden = true)}>✕</button>
	</div>
{/if}
<div class="ins">
	{#if screen !== 'server'}
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
	{/if}

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
					<!-- Две независимые галочки, обе выключены (владелец, 2026-10-24). -->
					<label class="ins-check">
						<input type="checkbox" bind:checked={remember} />
						<span>Запомнить пароль на этом компьютере <span class="ins-dim">— Мастерская будет входить сама</span></span>
					</label>
					<label class="ins-check">
						<input type="checkbox" bind:checked={makeKey} />
						<span
							>Дальше входить по ключу <span class="ins-dim"
								>— Мастерская создаст ключ на этом компьютере и положит его на сервер</span
							></span
						>
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
		{:else if screen === 'server' && page && st}
			<!-- Пульт (кадры 7а, 7а2, 7а3, 7е). -->
			{#if keyNote}<div class="ins-box ins-note">{keyNote}</div>{/if}
			<h2>{st.domain}</h2>
			<p class="ins-sub">
				Wynd {st.version} · {st.running
					? 'работает'
					: st.stopped_at
						? `остановлен вами ${day(st.stopped_at)}`
						: 'не работает'} · свободно {gb(st.free_bytes)}
			</p>
			{#if st.update}
				<div class="ins-box">
					<div class="ins-strong">Обновление до {st.update.to} не закончено</div>
					<div class="ins-dim">
						<!-- Момент обрыва Мастерская не знает — время начала обновления (владелец, 2026-10-25). -->
						Обновление началось {dayTime(st.update.started)}. Связь с сервером прервалась{st.update.stopped
							? ', когда Wynd был остановлен,'
							: ''}{st.running ? '' : ' — с тех пор сайт не открывается'}. {st.update.copied
							? 'Копия данных перед обновлением на месте.'
							: 'Копию сделать не успели — данные не менялись.'}
					</div>
					<div class="ins-foot">
						<Button variant="colored" onclick={() => void startUpdate()}>Продолжить обновление</Button>
						<Button variant="ghost" loading={returning} onclick={() => void returnPrevious()}>
							Вернуть {st.update.from}
						</Button>
					</div>
				</div>
			{:else}
				<div class="ins-svcbar">
					{#if st.running}
						<Button
							variant="ghost"
							loading={svcBusy === 'restart'}
							disabled={Boolean(svcBusy)}
							onclick={() => void service('restart')}>Перезапустить</Button
						>
						<Button variant="ghost" disabled={Boolean(svcBusy)} onclick={() => (askStop = true)}>
							Остановить
						</Button>
					{:else}
						<Button variant="colored" loading={svcBusy === 'start'} onclick={() => void service('start')}>
							Запустить
						</Button>
					{/if}
				</div>
				{#if askStop}
					<div class="ins-box">
						<div class="ins-strong">Остановить сайт?</div>
						<div class="ins-dim">
							Пока он остановлен, круги не открываются ни у кого, письма с сервера не уходят. Данные
							остаются на месте — «Запустить» вернёт всё как было.
						</div>
						<div class="ins-foot">
							<Button variant="colored" loading={svcBusy === 'stop'} onclick={() => void service('stop')}>
								Остановить
							</Button>
							<Button variant="ghost" onclick={() => (askStop = false)}>Отмена</Button>
						</div>
					</div>
				{/if}
				{#if page.latest}
					<div class="ins-box ins-update">
						<div>
							<div class="ins-strong">Вышла версия {page.latest}</div>
							<div class="ins-dim">
								Перед обновлением сохраним копию данных на сервере.
								<TextButton class="link under" onclick={() => void backend().OpenLink(CHANGES)}>
									что нового
								</TextButton>
							</div>
						</div>
						<Button variant={st.running ? 'colored' : 'ghost'} onclick={toUpdatePlan}>Обновить</Button>
					</div>
				{:else}
					<p class="ins-dim">Стоит самая свежая версия.</p>
				{/if}
				<div class="ins-rows">
					<div class="ins-row">
						<span class="ins-mark">↓</span>
						<div class="ins-grow">Бэкап на этот компьютер</div>
						<TextButton class="link under" onclick={() => void toBackupPlan()}>сделать</TextButton>
					</div>
					<div class="ins-row">
						<span class="ins-mark">→</span>
						<div class="ins-grow">Переехать на другой сервер</div>
						<TextButton class="link under" onclick={toMove}>начать</TextButton>
					</div>
				</div>
			{/if}
			{#if svcError}
				<div class="ins-box bad" role="alert">
					{svcError}
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>подробности</TextButton>
				</div>
			{/if}
			{#if askRollback}
				{@render removeBox('Да, убрать', st.domain ?? '')}
			{/if}
			<div class="ins-foot">
				<Button variant="ghost" onclick={() => void toConnect()}>Другой сервер</Button>
				{#if selfUp && selfHidden}
					<span class="ins-dim ins-push"
						>Мастерская {ownVersion} · есть {selfUp.version} —
						<TextButton class="link under" onclick={() => (selfOpen = true)}>обновить</TextButton></span
					>
				{/if}
				{#if !askRollback}
					<TextButton class="link under {selfUp && selfHidden ? '' : 'ins-push'}" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
				{/if}
			</div>
			{#if showLog}
				<pre class="ins-raw">{logText}</pre>
			{/if}
		{:else if screen === 'uplan' && page && st && uplanNoRoom}
			<!-- Кадр 7б2. -->
			<h2>Пока обновить нельзя</h2>
			<div class="ins-box">
				<div class="ins-strong">На сервере мало места для копии</div>
				<div class="ins-dim">
					Перед обновлением сохраняем копию данных: без неё при неудаче их нечем вернуть. Копии нужно
					{gb(page.copy_bytes)}, свободно {gb(st.free_bytes)}. Освободите место на сервере и нажмите
					«Проверить снова».
				</div>
			</div>
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void refreshPage()}>Проверить снова</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</div>
		{:else if screen === 'uplan' && page && st}
			<!-- Кадры 7б, 7б3, 7б4. -->
			{@const get = buildUpdate ? `Соберём Wynd ${page.latest} из исходного кода` : `Скачаем Wynd ${page.latest}`}
			{@const stopLive = st.running && !(stopForCopy || !page.live_copy)}
			<h2>Обновим до {page.latest}</h2>
			<div class="ins-rows">
				<div class="ins-row">
					<span class="ins-mark">1</span>
					<div>{get}{#if st.running}&nbsp;<span class="ins-dim">— сайт пока работает</span>{/if}</div>
				</div>
				{#if st.running && !stopLive}
					<div class="ins-row">
						<span class="ins-mark">2</span>
						<div>Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span></div>
					</div>
				{/if}
				<div class="ins-row">
					<span class="ins-mark">{st.running && !stopLive ? 3 : 2}</span>
					<div>
						Сохраним копию данных на сервере
						<span class="ins-dim"
							>— займёт {gb(page.copy_bytes)}, свободно {gb(st.free_bytes)}. Фото и видео обновление не
							меняет: в копии они те же файлы и места не занимают</span
						>
					</div>
				</div>
				{#if stopLive}
					<div class="ins-row">
						<span class="ins-mark">3</span>
						<div>Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span></div>
					</div>
				{/if}
				<div class="ins-row">
					<span class="ins-mark">{st.running ? 4 : 3}</span>
					<div>{st.running || startAfter ? `Запустим ${page.latest} и проверим сайт` : `Поставим ${page.latest}`}</div>
				</div>
			</div>
			{#if st.running}
				<label class="ins-check">
					<input type="checkbox" bind:checked={stopForCopy} disabled={!page.live_copy} />
					<span>
						Остановить сайт на время копии
						<span class="ins-dim">
							{#if !page.live_copy}
								— Wynd {st.version} копию без остановки не умеет.
							{/if}
							Пока идёт копия, никто ничего не напишет — при возврате к {st.version} ничего не пропадёт.
							Без галочки сайт закрыт только на запуск новой версии, но то, что напишут во время копии,
							при возврате пропадёт.
						</span>
					</span>
				</label>
			{:else}
				<Label>После обновления</Label>
				<label class="ins-check">
					<input type="radio" name="after" value={true} bind:group={startAfter} />
					<span>Запустить новую версию</span>
				</label>
				<label class="ins-check">
					<input type="radio" name="after" value={false} bind:group={startAfter} />
					<span>
						Оставить сайт остановленным
						{#if st.stopped_at}<span class="ins-dim">— его остановили вы {day(st.stopped_at)}</span>{/if}
					</span>
				</label>
			{/if}
			<p class="ins-sub mt">Не тронем: Docker, {proxyName || 'Caddy'}, сертификат сайта.</p>
			{#if page.can_build}
				<p class="ins-sub">
					{buildUpdate ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
					<TextButton class="link under" onclick={() => (buildUpdate = !buildUpdate)}>
						{buildUpdate ? 'Скачать готовый' : 'Собрать самому'}
					</TextButton>
				</p>
			{/if}
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void startUpdate()}>Обновить</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
				{#if buildUpdate}<span class="ins-dim">около 10 минут</span>{/if}
			</div>
		{:else if screen === 'update' && progress?.failure}
			<!-- Кадр 7д: после остановки — вернуть прежнюю; раньше сайт работает, только повторить. -->
			{@const stopped = ['start', 'check'].includes(progress.failure.step)}
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<div class="ins-box">{progress.failure.advice}</div>{/if}
			{#if stopped}
				<p class="ins-sub mt">
					«Вернуть {upFrom}» запустит прежнюю версию с данными из копии — сайт заработает как до
					обновления.
				</p>
			{/if}
			{#if svcError}<div class="ins-box bad" role="alert">{svcError}</div>{/if}
			<div class="ins-foot">
				{#if stopped}
					<Button variant="colored" loading={returning} onclick={() => void returnPrevious()}>
						Вернуть {upFrom}
					</Button>
					<Button variant="ghost" disabled={returning} onclick={() => void startUpdate()}>Повторить</Button>
				{:else}
					<Button variant="colored" onclick={() => void startUpdate()}>Повторить</Button>
					<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
				{/if}
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</div>
			{#if showLog}
				<pre class="ins-raw">{logText}</pre>
			{/if}
		{:else if screen === 'update'}
			<h2>Обновляем</h2>
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
		{:else if screen === 'udone'}
			<!-- Кадр 7г. -->
			<h2>Wynd обновлён</h2>
			<p class="ins-sub">
				{page?.state.running
					? `Проверка прошла: работает ${upTo}, сайт открывается.`
					: `Wynd ${upTo} поставлен, сайт остановлен.`}
			</p>
			<div class="ins-box">
				Копия данных до обновления лежит на сервере в папке /opt/wynd/backups/{upFrom}{page
					? ` — ${gb(page.copy_bytes)}`
					: ''}.
			</div>
			<div class="ins-foot">
				{#if page?.state.running}
					<Button variant="colored" onclick={() => void backend().OpenLink(`https://${page?.state.domain}`)}>
						Открыть {page?.state.domain}
					</Button>
				{/if}
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</div>
		{:else if screen === 'bplan' && page && st}
			<!-- Кадры 8а, 8а2. -->
			<h2>Бэкап на этот компьютер</h2>
			<p class="ins-sub">
				Вся папка Wynd на сервере — из неё Wynd можно поставить заново: записи, фото и видео, учётные
				записи, настройки и сертификат сайта.
			</p>
			<Label>Папка</Label>
			<div class="ins-copyline">
				<Input mono class="ins-grow" bind:value={bFolder} onblur={() => void planBackup()} />
				<Button variant="ghost" onclick={() => void chooseBackupFolder()}>Выбрать</Button>
			</div>
			{#if bPlan?.ok}
				<div class="ins-row">
					<span class="ins-mark">↓</span>
					<div>
						Скачаем {gb(bPlan.fetch)}
						<span class="ins-dim"
							>{bPlan.prev
								? `— в папке уже есть бэкап от ${day(bPlan.prev)}, берём только новое. Всего ${gb(bPlan.total)}`
								: ''}</span
						>
					</div>
				</div>
			{:else if bPlan}
				<div class="ins-box bad" role="alert">{bPlan.message}</div>
			{/if}
			{#if st.running}
				<Label>Сайт</Label>
				<label class="ins-check">
					<input type="radio" name="bsite" value="keep" bind:group={bSite} disabled={!page.live_copy} />
					<span>Работает, пока идёт бэкап{#if !page.live_copy}&nbsp;<span class="ins-dim">— Wynd {st.version} так не умеет</span>{/if}</span>
				</label>
				<label class="ins-check">
					<input type="radio" name="bsite" value="pause" bind:group={bSite} />
					<span>Остановить на время бэкапа и запустить снова</span>
				</label>
				<label class="ins-check">
					<input type="radio" name="bsite" value="stop" bind:group={bSite} />
					<span>Остановить и не запускать <span class="ins-dim">— сервер переезжает или закрывается</span></span>
				</label>
			{/if}
			<label class="ins-check">
				<input type="checkbox" bind:checked={bEncrypt} disabled={bPlan?.encrypted} />
				<span>Зашифровать паролем</span>
			</label>
			{#if bEncrypt}
				<div class="ins-pair">
					<div class="ins-grow">
						<Label>Пароль</Label><Input type="password" autocomplete="off" bind:value={bPass} />
					</div>
					{#if !bPlan?.encrypted}
						<div class="ins-grow">
							<Label>Ещё раз</Label><Input type="password" autocomplete="off" bind:value={bPass2} />
						</div>
					{/if}
				</div>
				<p class="ins-dim">
					{bPlan?.encrypted
						? 'В этой папке бэкап с паролем — нужен тот же пароль, иначе бэкап скачается целиком заново в другую папку.'
						: 'Без пароля бэкап не открыть никак — запишите его.'}
				</p>
			{/if}
			<div class="ins-foot">
				<Button
					variant={bFolder && passOk && bPlan?.ok && (bSite !== 'keep' || page.live_copy || !st.running)
						? 'colored'
						: 'off'}
					onclick={() => void startBackup()}>Сделать бэкап</Button
				>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</div>
		{:else if screen === 'backup' && progress?.failure}
			<!-- Кадр 8г. -->
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<div class="ins-box">{progress.failure.advice}</div>{/if}
			{#if progress.failure.step === 'fetch' && bPlan?.prev}
				<p class="ins-sub mt">Прошлый бэкап в папке не тронут, пока новый не скачан целиком.</p>
			{/if}
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void startBackup()}>Повторить</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</div>
			{#if showLog}
				<pre class="ins-raw">{logText}</pre>
			{/if}
		{:else if screen === 'backup'}
			<!-- Кадр 8б. -->
			<h2>Делаем бэкап</h2>
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
		{:else if screen === 'bdone'}
			<!-- Кадр 8в. -->
			<h2>Бэкап готов</h2>
			<p class="ins-sub">В папке {bFolder}.</p>
			<div class="ins-box">
				{bEncrypt
					? 'Бэкап зашифрован. Без пароля его не открыть.'
					: 'В бэкапе — все записи и ключи сервера. Храните его так же бережно, как пароли: кто получит папку, прочтёт все круги.'}
			</div>
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void backend().ShowInFolder(bFolder + '\\wynd-backup.json')}>
					Открыть папку
				</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</div>
		{:else if screen === 'mconnect' && st}
			<!-- Кадр 9а. -->
			<h2>Куда переезжает {st.domain}</h2>
			<p class="ins-sub">Адрес сайта останется прежним — люди откроют Wynd как раньше, с тех же телефонов.</p>
			<form
				class="ins-form"
				onsubmit={(e) => {
					e.preventDefault();
					void moveConnect();
				}}
			>
				<Label>Адрес нового сервера</Label>
				<Input mono autocomplete="off" spellcheck={false} placeholder="203.0.113.7" bind:value={mHost} />
				<Label>Вход</Label>
				<div class="ins-chips">
					<Chip selected={!mUseKeys} onclick={() => (mUseKeys = false)}>Пароль</Chip>
					<Chip selected={mUseKeys} onclick={() => (mUseKeys = true)}>Ключ с этого компьютера</Chip>
				</div>
				<div class="ins-pair">
					<div class="ins-user">
						<Label>Пользователь</Label>
						<Input mono autocomplete="off" spellcheck={false} bind:value={mUser} />
					</div>
					{#if !mUseKeys}
						<div class="ins-grow">
							<Label>Пароль от сервера</Label>
							<Input type="password" autocomplete="off" bind:value={mPass} />
						</div>
					{/if}
				</div>
				{#if !mUseKeys}
					<label class="ins-check">
						<input type="checkbox" bind:checked={mMakeKey} />
						<span
							>Дальше входить по ключу <span class="ins-dim"
								>— Мастерская создаст ключ на этом компьютере и положит его на сервер</span
							></span
						>
					</label>
				{/if}
				{#if mFail}
					<div class="ins-box bad" role="alert">
						<div class="ins-strong">{mFail.message}</div>
						{#if mFail.advice}<div class="ins-dim">{mFail.advice}</div>{/if}
					</div>
				{/if}
				<div class="ins-foot">
					<Button variant="colored" loading={mBusy} onclick={() => void moveConnect()}>Подключиться</Button>
					<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
				</div>
			</form>
		{:else if screen === 'mfinger'}
			<h2>Это ваш новый сервер?</h2>
			<p class="ins-sub">
				Подключаемся к {mHost.trim()} впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга
				или панелью, если там он есть.
			</p>
			<div class="ins-box ins-mono">{mFinger}</div>
			<div class="ins-foot">
				<Button variant="colored" loading={mBusy} onclick={() => void moveConfirm()}>Да, подключиться</Button>
				<Button variant="ghost" disabled={mBusy} onclick={() => (screen = 'mconnect')}>Отмена</Button>
			</div>
		{:else if screen === 'mplan' && !mplan}
			<h2>Что сделаем</h2>
			<p class="ins-dim">Осматриваем новый сервер…</p>
		{:else if screen === 'mplan' && mplan && !mplan.ok}
			<h2>Пока переехать нельзя</h2>
			<div class="ins-box" role="alert">
				<div class="ins-strong">{mplan.message}</div>
				{#if mplan.advice}<div class="ins-dim">{mplan.advice}</div>{/if}
			</div>
			<div class="ins-foot">
				<Button variant="ghost" onclick={() => (screen = 'mconnect')}>Другой сервер</Button>
				<Button variant="ghost" onclick={() => void toServer()}>К серверу</Button>
			</div>
		{:else if screen === 'mplan' && mplan}
			<!-- Кадр 9б. -->
			<h2>Что сделаем</h2>
			<div class="ins-rows">
				<div class="ins-row"><span class="ins-mark">1</span><div>Поставим Wynd {mplan.version} на новый сервер <span class="ins-dim">— пока без данных и не запуская</span></div></div>
				<div class="ins-row"><span class="ins-mark">2</span><div>Остановим сайт на старом <span class="ins-dim">— с этой минуты он не открывается</span></div></div>
				<div class="ins-row"><span class="ins-mark">3</span><div>Перенесём папку Wynd на новый сервер <span class="ins-dim">— {gb(mplan.size)}, напрямую со старого, мимо этого компьютера</span></div></div>
				<div class="ins-row"><span class="ins-mark">4</span><div>Вы смените адрес <span class="ins-dim">— у записи A для {mplan.domain} поставите {mplan.new_ip}</span></div></div>
				<div class="ins-row"><span class="ins-mark">5</span><div>Запустим Wynd на новом и проверим сайт</div></div>
			</div>
			<Label>Старый сервер</Label>
			<label class="ins-check">
				<input type="radio" name="mold" value={false} bind:group={mRemoveOld} />
				<span>Оставить Wynd остановленным <span class="ins-dim">— убрать потом, когда убедитесь, что всё на месте</span></span>
			</label>
			<label class="ins-check">
				<input type="radio" name="mold" value={true} bind:group={mRemoveOld} />
				<span>Убрать Wynd после переезда <span class="ins-dim">— со старого сервера уйдут его данные; сертификат переедет на новый</span></span>
			</label>
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void startMove()}>Переехать</Button>
				<Button variant="ghost" onclick={() => (screen = 'server')}>Назад</Button>
			</div>
		{:else if screen === 'move' && progress?.failure}
			<h2>{progress.failure.message}</h2>
			{#if progress.failure.advice}<div class="ins-box">{progress.failure.advice}</div>{/if}
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void startMove()}>Повторить</Button>
				<Button variant="ghost" loading={mBusy} onclick={() => void cancelMove()}>Отменить переезд</Button>
				<TextButton class="link under ins-push" onclick={() => (showLog = !showLog)}>подробности</TextButton>
			</div>
			{#if showLog}<pre class="ins-raw">{logText}</pre>{/if}
		{:else if screen === 'move'}
			<!-- Кадр 9в. -->
			{@const waitingDNS = progress?.steps?.some((s) => s.id === 'dns' && s.status === 'running')}
			<h2>Переезжаем</h2>
			<div class="ins-rows">
				{#each progress?.steps ?? [] as step (step.id)}
					<div class="ins-row" class:ins-dim={step.status === 'pending'}>
						<span class="ins-mark {step.status === 'done' ? 'ok' : ''}">{stepMark(step)}</span>
						<div>{step.title}{#if step.note}&nbsp;<span class="ins-dim">— {step.note}</span>{/if}</div>
					</div>
				{/each}
			</div>
			{#if waitingDNS && mplan}
				<div class="ins-box">
					<div class="ins-dim">В панели, где покупали адрес {mplan.domain}, поменяйте у записи A адрес сервера на этот:</div>
					<div class="ins-copyline">
						<div class="ins-mono">{mplan.new_ip}</div>
						<Button variant="ghost" onclick={() => void copyLine(mplan?.new_ip ?? '')}>
							{copiedLine === mplan.new_ip ? 'Скопировано' : 'Скопировать'}
						</Button>
					</div>
					<div class="ins-dim">Мастерская проверяет сама каждую минуту — окно можно свернуть.</div>
				</div>
				<div class="ins-foot">
					<Button variant="ghost" onclick={() => void backend().MoveCheckNow()}>Проверить сейчас</Button>
					<TextButton class="link under ins-push" onclick={() => void cancelMove()}>Отменить переезд</TextButton>
				</div>
			{:else}
				<div class="ins-foot">
					<TextButton class="link under" onclick={() => (showLog = !showLog)}>показать, что выполняется</TextButton>
					<span class="ins-dim">Окно можно свернуть</span>
				</div>
			{/if}
			{#if showLog}<pre class="ins-raw">{logText}</pre>{/if}
		{:else if screen === 'mdone' && mplan}
			<!-- Кадр 9г. -->
			<h2>Wynd на новом сервере</h2>
			<p class="ins-sub">Проверка прошла: {mplan.domain} открывается с сервера {mplan.new_ip}.</p>
			<div class="ins-box">
				{mRemoveOld
					? `Wynd со старого сервера ${mplan.old_ip} убран.`
					: `Старый сервер ${mplan.old_ip}: Wynd остановлен, данные на месте. Когда убедитесь, что на новом всё есть, уберите его со страницы старого сервера.`}
			</div>
			<div class="ins-foot">
				<Button variant="colored" onclick={() => void backend().OpenLink(`https://${mplan?.domain}`)}>
					Открыть {mplan.domain}
				</Button>
				<Button variant="ghost" onclick={() => void adoptNew()}>К серверу</Button>
			</div>
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
			{#if rollbackNote}
				<div class="ins-box ins-note">{rollbackNote}</div>
			{/if}
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
				<div class="ins-sec">Изменим</div>
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
				{@render removeBox('Да, откатить', '')}
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
			{#if askRollback}
				{@render removeBox('Да, убрать', plan?.domain ?? '')}
				{#if showLog && rollbackLog.length}
					<pre class="ins-raw">{logText}</pre>
				{/if}
			{:else if progress.link}
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
					<TextButton class="link under ins-push" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
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
					<TextButton class="link under ins-push" onclick={() => (askRollback = true)}>
						Убрать Wynd
					</TextButton>
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
			<!-- Итог удаления не теряется, когда новый осмотр нашёл помеху (кадр 5д). -->
			{#if rollbackNote}
				<div class="ins-box ins-note">{rollbackNote}</div>
			{/if}
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
					{#if block.copy}
						<div class="ins-copyline">
							<div class="ins-mono">{block.copy}</div>
							<Button variant="ghost" onclick={() => void copyLine(block.copy ?? '')}>
								{copiedLine === block.copy ? 'Скопировано' : 'Скопировать'}
							</Button>
						</div>
					{/if}
					{#if block.after}
						<div class="ins-dim">{block.after}</div>
					{/if}
				</div>
			{/each}
			<div class="ins-more">
				<TextButton class="link under" onclick={() => (showRaw = true)}>
					подробнее, что проверили
				</TextButton>
			</div>
			<div class="ins-foot">
				{#if blocks.every((b) => b.final)}
					<Button variant="colored" disabled={busy} onclick={() => void toConnect()}>
						Другой сервер
					</Button>
					<Button variant="ghost" loading={busy} onclick={() => void inspect()}>
						Проверить снова
					</Button>
				{:else}
					<Button variant="colored" loading={busy} onclick={() => void inspect()}>
						Проверить снова
					</Button>
					<Button variant="ghost" disabled={busy} onclick={() => void toConnect()}>
						Другой сервер
					</Button>
				{/if}
			</div>
		{:else}
			<h2>Что на сервере</h2>
			<p class="ins-sub">Только смотрим — ничего не меняем.</p>
			{#if rollbackNote}
				<div class="ins-box">{rollbackNote}</div>
			{/if}
			<div class="ins-rows">
				{#each findings as f (f.id)}
					<div class="ins-row">
						<span class="ins-mark {f.level}">{mark(f.level)}</span>
						<div>
							{f.text}{#if f.plan}&nbsp;— <span class="ins-dim">{f.plan}{f.id === 'wynd' ? '.' : ''}</span>{/if}
							{#if f.id === 'wynd' && !askRollback}
								<TextButton class="link under" onclick={() => (askRollback = true)}>
									Убрать Wynd с сервера
								</TextButton>
							{/if}
						</div>
					</div>
				{/each}
			</div>
			{#if askRollback}
				{@render removeBox('Да, убрать', domainOk ? domain : '')}
				{#if showLog && rollbackLog.length}
					<pre class="ins-raw">{logText}</pre>
				{/if}
			{/if}
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
				<TextButton class="link under" onclick={() => (showRaw = true)}>
					подробнее, что проверили
				</TextButton>
			</div>
		{/if}
	</main>
	{#if showRaw && inspection}
		<!-- Окно поверх (кадр 2е): читать ответы сервера, а не подглядывать в три строки. -->
		<div class="ins-over" role="presentation" onclick={() => (showRaw = false)}></div>
		<div class="ins-sheet" role="dialog" aria-modal="true" aria-label="Что проверили">
			<div class="ins-sheet-head">
				<b>Что проверили</b>
				<span class="ins-dim">ответы сервера на команды осмотра</span>
				<button class="ins-x" type="button" aria-label="Закрыть" onclick={() => (showRaw = false)}>
					✕
				</button>
			</div>
			<div class="ins-raw">
				{#each rawParts as s, i (i)}
					<div class:ins-cause-part={s.cause}>
						{#if s.cause}<b class="ins-cause">## {s.name} — из-за этого отказ</b>{:else}## {s.name}{/if}
						{#if s.body}<div>{s.body}</div>{/if}
					</div>
				{/each}
			</div>
			<div class="ins-sheet-foot">
				<Button variant="ghost" onclick={() => void copyRaw()}>
					{copiedRaw ? 'Скопировано' : 'Скопировать всё'}
				</Button>
				<Button variant="ghost" onclick={() => (showRaw = false)}>Закрыть</Button>
			</div>
		</div>
	{/if}
</div>
{/if}

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && showRaw) showRaw = false;
	}}
/>

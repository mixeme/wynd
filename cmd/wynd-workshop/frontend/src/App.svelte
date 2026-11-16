<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Checkbox from '$ui/workshop/Checkbox.svelte';
	import CommandLog from '$ui/workshop/CommandLog.svelte';
	import CopyLine from '$ui/workshop/CopyLine.svelte';
	import FieldLine from '$ui/workshop/FieldLine.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import Radio from '$ui/workshop/Radio.svelte';
	import RawSheet from '$ui/workshop/RawSheet.svelte';
	import StepList from '$ui/workshop/StepList.svelte';
	import type { ServerProfile, ProfileRequest, ProfileViewState } from './api';
	import { Panel } from './lib/panel.svelte';
	import { day, dayTime, gb, mark } from './lib/display';

	let {
		profile,
		profiles,
		request,
		reconnect = 0,
		onconsumed,
		onstate,
		onprofileschange,
		onedit,
		onforget,
		onaddtarget,
		onselect,
		onmoved
	}: {
		profile: ServerProfile;
		profiles: ServerProfile[];
		request?: ProfileRequest;
		reconnect?: number;
		onconsumed: () => void;
		onstate: (s: ProfileViewState) => void;
		onprofileschange: () => Promise<void>;
		onedit: () => void;
		onforget: () => void;
		onaddtarget: () => void;
		onselect: (id: string) => void;
		onmoved: (id: string) => Promise<void>;
	} = $props();

	const host = $derived(profile.host);

	// Логика панели — в контроллере (план 60); здесь разметка и wiring.
	const p = new Panel({
		profile: () => profile,
		profiles: () => profiles,
		request: () => request,
		reconnect: () => reconnect,
		onconsumed: () => onconsumed(),
		onstate: (s) => onstate(s),
		onprofileschange: () => onprofileschange(),
		onselect: (id) => onselect(id),
		onmoved: (id) => onmoved(id)
	});
	const { api } = p;
</script>

{#snippet removeBox(yes: string, site: string)}
	<Notice tone="bad" title="Убрать Wynd с сервера?">
		<div class="ins-dim">
			{#if site}Сайт {site} перестанет открываться.{/if}
			{#if p.foreignSite.length}
				Уберём контейнер Wynd. Docker и {p.proxyName} останутся.
			{:else if p.proxyName}
				Уберём контейнер Wynd и его сайт из {p.proxyName}. Docker и {p.proxyName} останутся.
			{:else}
				Уберём контейнеры Wynd и Caddy. Docker останется.
			{/if}
		</div>
		{#if p.foreignSite.length}
			<!-- Кадр 5е: файл сайта ставили не мы — не трогаем, называем. -->
			<div class="ins-dim">
				Файл сайта в {p.proxyName} ставили не мы, его не тронем. Пока он есть, {p.proxyName}
				отвечает по адресу
				{site || 'сайта'} ошибкой. Уберите его сами:
			</div>
			{#each p.foreignSite as f (f)}
				<CopyLine text={f} copied={p.copiedLine === f} oncopy={() => void p.copyLine(f)} />
			{/each}
		{/if}
		<Checkbox bind:checked={p.dropData}>
			Удалить данные Wynd — записи, фотографии, учётные записи{p.foreignSite.length
				? ''
				: ' — и сертификат сайта'}
		</Checkbox>
		<div class="ins-dim">
			{#if p.foreignSite.length}
				Без галочки папка /opt/wynd останется.
			{:else if p.proxyName}
				Без галочки папка /opt/wynd и сертификат останутся.
			{:else}
				Без галочки папка /opt/wynd останется как есть.
			{/if}
			Поставите Wynd снова — всё будет на месте.
		</div>
		{#if p.rollbackError}
			<div class="ins-dim" role="alert">{p.rollbackError}</div>
			<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>подробности</TextButton>
		{/if}
		<FootBar>
			<Button
				variant="colored"
				loading={p.rolling}
				loadingLabel="Убираем Wynd…"
				onclick={() => void p.attempt(() => p.rollback())}>{yes}</Button
			>
			<Button variant="ghost" disabled={p.rolling} onclick={() => (p.askRollback = false)}
				>Отмена</Button
			>
		</FootBar>
	</Notice>
{/snippet}

<main class="ins-body ins-pane">
	<Notice
		profile={{
			name: profile.name,
			host: profile.host,
			user: profile.user,
			domain: p.page?.state.domain ?? profile.last_check?.domain,
			status: p.connection === 'connected' ? 'Подключён' : p.status,
			state: p.connection
		}}
	>
		<Button
			variant="ghost"
			loading={p.connection === 'connecting'}
			loadingLabel="Подключаемся…"
			disabled={p.occupied || p.moving || Boolean(profile.reserved_by)}
			onclick={() => void p.attempt(p.connection === 'connected' ? p.toConnect : p.connect)}
			>{p.connection === 'connected' ? 'Отключиться' : 'Подключиться'}</Button
		>
		<Checkbox
			native
			checked={profile.auto_connect}
			disabled={!profile.auto_connect && !profile.use_keys && (!p.saved || !profile.remember)}
			onchange={(on) => void p.attempt(() => p.autoConnect(on))}>Подключаться при запуске</Checkbox
		>
	</Notice>
	{#if p.updating || p.backingUp || p.moving || ['inspect', 'plan', 'install', 'done'].includes(p.screen)}
		<div class="ins-operation-path">{p.updating
				? p.screen === 'uplan'
					? 'Обновление › План'
					: 'Обновление'
				: p.backingUp
					? 'Бэкап'
					: p.moving
						? p.screen === 'mplan'
							? 'Переезд › План'
							: 'Переезд'
						: p.screen === 'plan'
							? 'Установка › План'
							: 'Установка Wynd'}</div>
	{/if}
	{#if p.operationError}<Notice tone="bad" alert>{p.operationError}</Notice>{/if}
	{#if p.moving && p.target}<div class="ins-move-pair">
			<div>
				<strong>Откуда: {profile.name}</strong><small>SSH: {profile.user}@{profile.host}</small>
			</div>
			<span>→</span>
			<div>
				<strong>Куда: {p.target.name}</strong><small>SSH: {p.target.user}@{p.target.host}</small>
			</div>
		</div>{/if}

	{#if p.connection === 'connecting'}<h2>Проверяем сервер</h2>
		<p class="ins-sub">После подключения узнаем состояние Wynd. Можно выбрать другой сервер.</p>
	{:else if profile.reserved_by}<h2>Сервер участвует в переезде</h2>
		<p class="ins-sub">
			Выбран назначением для сервера {profiles.find((q) => q.id === profile.reserved_by)?.name}.
			Управление переездом — в пульте исходного сервера.
		</p>
	{:else if p.screen === 'offline' || p.screen === 'connect'}
		<h2>
			{p.connection === 'password_required'
				? 'Для входа нужен пароль'
				: p.failure
					? 'Не удалось подключиться'
					: 'Подключитесь, чтобы проверить сервер'}
		</h2>
		{#if p.failure}<Notice tone="bad" alert title={p.failure.message} text={p.failure.advice}
				>{#if p.failure.fingerprint}<div class="ins-mono">
						{p.failure.fingerprint}
					</div>{/if}</Notice
			>{/if}
		{#if !profile.use_keys}<Label>Пароль от сервера</Label><Input
				type="p.password"
				autocomplete="off"
				bind:value={p.password}
				placeholder={p.saved ? 'сохранён — можно не вводить' : ''}
			/><Checkbox native bind:checked={p.sessionRemember}
				>Запомнить пароль на этом компьютере</Checkbox
			>
		{/if}
		{#if profile.last_check}{@const c = profile.last_check}<Notice
				title={`Последняя проверка: ${dayTime(c.at)}`}
				><p>
					{c.installed
						? `Wynd ${c.version ?? ''} ${c.running ? 'работал' : 'не работал'}`
						: 'Wynd не был установлен'}{c.free_bytes ? ` · свободно ${gb(c.free_bytes)}` : ''}.
				</p>
				<p class="ins-dim">
					Это сохранённые сведения. Текущее состояние узнаем после подключения.
				</p></Notice
			>{/if}
		{#if p.failure || p.connection === 'password_required'}
			<FootBar
				><Button
					variant="colored"
					loading={p.busy}
					loadingLabel="Подключаемся…"
					disabled={Boolean(profile.reserved_by)}
					onclick={() => void p.attempt(p.connect)}>{p.failure ? 'Повторить' : 'Подключиться'}</Button
				><Button
					variant="ghost"
					class="ins-push"
					disabled={p.occupied || p.moving || Boolean(profile.reserved_by)}
					onclick={onedit}>Изменить доступ</Button
				></FootBar
			>
		{/if}
	{:else if p.screen === 'new'}
		{#if p.rollbackNote}<Notice tone="note">{p.rollbackNote}</Notice>{/if}
		<h2>Wynd на сервере не найден</h2>
		<p class="ins-sub">Проверено сейчас.</p>
		<Notice
			>Можно установить Wynd. Сначала Мастерская проверит, что потребуется изменить, и покажет план.</Notice
		>
		<FootBar
			><Button variant="colored" onclick={() => (p.screen = 'inspect')}>Установить Wynd</Button
			></FootBar
		>
	{:else if p.screen === 'server' && p.page && p.st}
		<!-- Пульт (кадры 7а, 7а2, 7а3, 7е). -->
		{#if p.keyNote}<Notice tone="note">{p.keyNote}</Notice>{/if}
		<h2>
			{p.page.foreign
				? 'Этот Wynd ставила не Мастерская'
				: p.st.running
					? 'Wynd работает'
					: p.st.stopped_at
						? 'Wynd остановлен'
						: 'Wynd не работает'}
		</h2>
		<p class="ins-sub">
			Сервер {p.page.host} · Wynd{p.st.version ? ` ${p.st.version}` : ''} · {p.st.running
				? 'работает'
				: p.st.stopped_at
					? `остановлен вами ${day(p.st.stopped_at)}`
					: 'не работает'} · свободно {gb(p.st.free_bytes)}
		</p>
		{#if p.page.foreign && !p.st.move}
			<!-- Кадр 7з: только переезд. -->
			{@const f = p.page.foreign}
			<Notice title="Этот Wynd ставила не Мастерская">
				<div class="ins-dim">
					Он {f.running ? 'работает' : 'запускается'} как {f.kind === 'service'
						? `служба ${f.name}`
						: f.kind === 'container'
							? `контейнер из ${f.name}`
							: `программа ${f.name}`}, данные — в {f.data_dir}. Обновлять такой Wynd, делать его
					бэкап и убирать Мастерская не умеет. Перевезти на другой сервер — умеет: там она поставит
					Wynd сама и перенесёт данные и адрес сайта.
				</div>
			</Notice>
			<MarkRows class="mt-10">
				<MarkRow mark="→">
					Переехать на другой сервер
					{#snippet action()}
						<TextButton class="link under" onclick={p.toMove}>Перенести</TextButton>
					{/snippet}
				</MarkRow>
			</MarkRows>
		{:else if p.st.move}
			<!-- Кадр 9в2. -->
			<Notice title="Переезд на сервер {p.st.move.to} не закончен">
				<div class="ins-dim">
					Переезд начался {dayTime(p.st.move.started)}. С тех пор сайт на старом сервере {p.page.host} остановлен.
				</div>
				<FootBar>
					<Button variant="colored" onclick={() => p.resumeMove('continue')}
						>Продолжить переезд</Button
					>
					<Button variant="ghost" onclick={() => p.resumeMove('cancel')}>Отменить переезд</Button>
				</FootBar>
			</Notice>
		{:else if p.st.update}
			<Notice title="Обновление до {p.st.update.to} не закончено">
				<div class="ins-dim">
					<!-- Момент обрыва Мастерская не знает — время начала обновления (владелец, 2026-10-25). -->
					Обновление началось {dayTime(p.st.update.started)}. Связь с сервером прервалась{p.st.update
						.stopped
						? ', когда Wynd был остановлен,'
						: ''}{p.st.running ? '' : ' — с тех пор сайт не открывается'}. {p.st.update.copied
						? 'Копия данных перед обновлением на месте.'
						: 'Копию сделать не успели — данные не менялись.'}
				</div>
				<FootBar>
					<Button variant="colored" onclick={() => void p.attempt(() => p.startUpdate())}
						>Продолжить обновление</Button
					>
					<Button
						variant="ghost"
						loading={p.returning}
						loadingLabel="Возвращаем…"
						onclick={() => void p.attempt(() => p.returnPrevious())}
					>
						Вернуть {p.st.update.from}
					</Button>
				</FootBar>
			</Notice>
		{:else}
			<div class="ins-svcbar">
				{#if p.st.running}
					<Button
						variant="ghost"
						loading={p.svcBusy === 'restart'}
						loadingLabel="Перезапускаем…"
						disabled={Boolean(p.svcBusy)}
						onclick={() => void p.attempt(() => p.service('restart'))}>Перезапустить</Button
					>
					<Button variant="ghost" disabled={Boolean(p.svcBusy)} onclick={() => (p.askStop = true)}>
						Остановить
					</Button>
				{:else}
					<Button
						variant="colored"
						loading={p.svcBusy === 'start'}
						loadingLabel="Запускаем…"
						onclick={() => void p.attempt(() => p.service('start'))}
					>
						Запустить
					</Button>
				{/if}
			</div>
			{#if p.askStop}
				<Notice title="Остановить сайт?">
					<div class="ins-dim">
						Пока он остановлен, круги не открываются ни у кого, письма с сервера не уходят. Данные
						остаются на месте — «Запустить» вернёт всё как было.
					</div>
					<FootBar>
						<Button
							variant="colored"
							loading={p.svcBusy === 'stop'}
							loadingLabel="Останавливаем…"
							onclick={() => void p.attempt(() => p.service('stop'))}
						>
							Остановить
						</Button>
						<Button variant="ghost" onclick={() => (p.askStop = false)}>Отмена</Button>
					</FootBar>
				</Notice>
			{/if}
			{#if p.page.latest}
				<Notice class="ins-pult-update" title="Вышла версия {p.page.latest}">
					<div class="ins-dim">
						Перед обновлением сохраним копию данных на сервере.
					</div>
					<FootBar>
						<Button variant={p.st.running ? 'colored' : 'ghost'} onclick={p.toUpdatePlan}
							>Обновить</Button
						>
					</FootBar>
				</Notice>
			{:else}
				<p class="ins-dim">Стоит самая свежая версия.</p>
			{/if}
			<MarkRows class="ins-pult-actions">
				<MarkRow mark="">
					Бэкап на этот компьютер
					{#snippet action()}
						<TextButton class="link under" onclick={() => void p.attempt(() => p.toBackupPlan())}
							>Сделать</TextButton
						>
					{/snippet}
				</MarkRow>
				<MarkRow mark="">
					Переезд на другой сервер
					{#snippet action()}
						<TextButton class="link under" onclick={p.toMove}>Перенести</TextButton>
					{/snippet}
				</MarkRow>
			</MarkRows>
		{/if}
		{#if p.svcError}
			<Notice tone="bad" alert>
				{p.svcError}
				<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>подробности</TextButton>
			</Notice>
		{/if}
		{#if p.askRollback}
			{@render removeBox('Да, убрать', p.st.domain ?? '')}
		{/if}
		{#if p.showLog}
			<CommandLog text={p.logText} />
		{/if}
	{:else if p.screen === 'uplan' && p.page && p.st && p.uplanNoRoom}
		<!-- Кадр 7б2. -->
		<h2>Пока обновить нельзя</h2>
		<Notice title="На сервере мало места для копии">
			<div class="ins-dim">
				Перед обновлением сохраняем копию данных: без неё при неудаче их нечем вернуть. Копии нужно
				{gb(p.page.copy_bytes)}, свободно {gb(p.st.free_bytes)}. Освободите место на сервере и нажмите
				«Проверить снова».
			</div>
		</Notice>
		<FootBar>
			<Button variant="colored" onclick={() => void p.refreshPage()}>Проверить снова</Button>
			<Button variant="ghost" onclick={() => (p.screen = 'server')}>Назад</Button>
		</FootBar>
	{:else if p.screen === 'uplan' && p.page && p.st}
		<!-- Кадры 7б, 7б3, 7б4. -->
		{@const get = p.buildUpdate
			? `Соберём Wynd ${p.page.latest} из исходного кода`
			: `Скачаем Wynd ${p.page.latest}`}
		{@const stopLive = p.st.running && !(p.stopForCopy || !p.page.live_copy)}
		<h2>Обновим до {p.page.latest}</h2>
		<MarkRows>
			<MarkRow mark="1">
				{get}{#if p.st.running}&nbsp;<span class="ins-dim">— сайт пока работает</span>{/if}
			</MarkRow>
			{#if p.st.running && !stopLive}
				<MarkRow mark="2">
					Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span>
				</MarkRow>
			{/if}
			<MarkRow mark={p.st.running && !stopLive ? 3 : 2}>
				Сохраним копию данных на сервере
				<span class="ins-dim"
					>— займёт {gb(p.page.copy_bytes)}, свободно {gb(p.st.free_bytes)}. Фото и видео обновление не
					меняет: в копии они те же файлы и места не занимают</span
				>
			</MarkRow>
			{#if stopLive}
				<MarkRow mark="3">
					Остановим Wynd <span class="ins-dim">— с этой минуты сайт не открывается</span>
				</MarkRow>
			{/if}
			<MarkRow mark={p.st.running ? 4 : 3}>
				{p.st.running || p.startAfter
					? `Запустим ${p.page.latest} и проверим сайт`
					: `Поставим ${p.page.latest}`}
			</MarkRow>
		</MarkRows>
		{#if p.st.running}
			<Checkbox bind:checked={p.stopForCopy} disabled={!p.page.live_copy}>
				Остановить сайт на время копии
				<span class="ins-dim">
					{#if !p.page.live_copy}
						— Wynd {p.st.version} копию без остановки не умеет.
					{/if}
					Пока идёт копия, никто ничего не напишет — при возврате к {p.st.version} ничего не пропадёт. Без
					галочки сайт закрыт только на запуск новой версии, но то, что напишут во время копии, при возврате
					пропадёт.
				</span>
			</Checkbox>
		{:else}
			<Label>После обновления</Label>
			<Radio name="after" value={true} bind:group={p.startAfter}>Запустить новую версию</Radio>
			<Radio name="after" value={false} bind:group={p.startAfter}>
				Оставить сайт остановленным
				{#if p.st.stopped_at}<span class="ins-dim">— его остановили вы {day(p.st.stopped_at)}</span
					>{/if}
			</Radio>
		{/if}
		<p class="ins-sub mt">
			Не тронем: Docker, {p.proxyName || 'Caddy'}, сертификат сайта.
		</p>
		{#if p.page.can_build}
			<p class="ins-sub">
				{p.buildUpdate ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
				<TextButton class="link under" onclick={() => (p.buildUpdate = !p.buildUpdate)}>
					{p.buildUpdate ? 'Скачать готовый' : 'Собрать самому'}
				</TextButton>
			</p>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.startUpdate())}>Обновить</Button>
			<Button variant="ghost" onclick={() => (p.screen = 'server')}>Назад</Button>
			{#if p.buildUpdate}<span class="ins-dim">около 10 минут</span>{/if}
		</FootBar>
	{:else if p.screen === 'update' && p.progress?.failure}
		<!-- Кадр 7д: после остановки — вернуть прежнюю; раньше сайт работает, только повторить. -->
		{@const stopped = ['start', 'check'].includes(p.progress.failure.step)}
		<h2>{p.progress.failure.message}</h2>
		<StepList class="ins-stage-steps" steps={p.progress?.steps ?? []} />
		{#if p.progress.failure.advice}<Notice>{p.progress.failure.advice}</Notice>{/if}
		{#if stopped}
			<p class="ins-sub mt">
				«Вернуть {p.upFrom}» запустит прежнюю версию с данными из копии — сайт заработает как до
				обновления.
			</p>
		{/if}
		{#if p.svcError}<Notice tone="bad" alert>{p.svcError}</Notice>{/if}
		<FootBar>
			{#if stopped}
				<Button
					variant="colored"
					loading={p.returning}
					loadingLabel="Возвращаем…"
					onclick={() => void p.attempt(() => p.returnPrevious())}
				>
					Вернуть {p.upFrom}
				</Button>
				<Button
					variant="ghost"
					disabled={p.returning}
					onclick={() => void p.attempt(() => p.startUpdate())}>Повторить</Button
				>
			{:else}
				<Button variant="colored" onclick={() => void p.attempt(() => p.startUpdate())}
					>Повторить</Button
				>
				<Button variant="ghost" onclick={() => void p.attempt(() => p.toServer())}>К серверу</Button>
			{/if}
			<TextButton class="link under ins-push" onclick={() => (p.showLog = !p.showLog)}
				>{p.showLog
					? 'Скрыть вывод сборки'
					: p.progress.failure.step === 'image'
						? 'Полный вывод сборки'
						: 'подробности'}</TextButton
			>
		</FootBar>
		{#if p.showLog}
			<CommandLog text={p.buildLogText || p.logText} />
		{/if}
	{:else if p.screen === 'update'}
		<h2>Обновляем</h2>
		<StepList class="ins-stage-steps" steps={p.progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>
				{p.showLog ? 'Скрыть вывод сборки' : p.buildUpdate ? 'Полный вывод сборки' : 'показать, что выполняется'}
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
		</FootBar>
		{#if p.showLog}
			<CommandLog text={p.buildLogText || p.logText} />
		{/if}
	{:else if p.screen === 'udone'}
		<!-- Кадр 7г. -->
		<h2>Wynd обновлён</h2>
		<p class="ins-sub">
			{p.page?.state.running
				? `Проверка прошла: работает ${p.upTo}, сайт открывается.`
				: `Wynd ${p.upTo} поставлен, сайт остановлен.`}
		</p>
		<Notice>
			Копия данных до обновления лежит на сервере в папке /opt/wynd/backups/{p.upFrom}{p.page
				? ` — ${gb(p.page.copy_bytes)}`
				: ''}.
		</Notice>
		<FootBar>
			{#if p.page?.state.running}
				<Button
					variant="colored"
					onclick={() => void api.OpenLink(`https://${p.page?.state.domain}`)}
				>
					Открыть {p.page?.state.domain}
				</Button>
			{/if}
			<Button variant="ghost" onclick={() => void p.attempt(() => p.toServer())}>К серверу</Button>
		</FootBar>
	{:else if p.screen === 'bplan' && p.page && p.st}
		<!-- Кадры 8а, 8а2. -->
		<h2>Бэкап на этот компьютер</h2>
		<p class="ins-sub">
			Вся папка Wynd на сервере — из неё Wynd можно поставить заново: записи, фото и видео, учётные
			записи, настройки и сертификат сайта.
		</p>
		<Label>Папка</Label>
		<FieldLine>
			<Input
				mono
				class="ins-grow"
				bind:value={p.bFolder}
				onblur={() => void p.attempt(() => p.planBackup())}
			/>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.chooseBackupFolder())}
				>Выбрать</Button
			>
		</FieldLine>
		{#if p.bPlan?.ok}
			<MarkRow mark="↓">
				Скачаем {gb(p.bPlan.fetch)}
				<span class="ins-dim"
					>{p.bPlan.prev
						? `— в папке уже есть бэкап от ${day(p.bPlan.prev)}, берём только новое. Всего ${gb(p.bPlan.total)}`
						: ''}</span
				>
			</MarkRow>
		{:else if p.bPlan}
			<Notice tone="bad" alert>{p.bPlan.message}</Notice>
		{/if}
		{#if p.st.running}
			<Label>Сайт</Label>
			<Radio name="bsite" value="keep" bind:group={p.bSite} disabled={!p.page.live_copy}>
				Работает, пока идёт бэкап{#if !p.page.live_copy}&nbsp;<span class="ins-dim"
						>— Wynd {p.st.version} так не умеет</span
					>{/if}
			</Radio>
			<Radio name="bsite" value="pause" bind:group={p.bSite}
				>Остановить на время бэкапа и запустить снова</Radio
			>
			<Radio name="bsite" value="stop" bind:group={p.bSite}>
				Остановить и не запускать <span class="ins-dim">— сервер переезжает или закрывается</span>
			</Radio>
		{/if}
		<Checkbox bind:checked={p.bEncrypt} disabled={p.bPlan?.encrypted}>Зашифровать паролем</Checkbox>
		{#if p.bEncrypt}
			<div class="ins-pair">
				<div class="ins-grow">
					<Label>Пароль</Label><Input type="p.password" autocomplete="off" bind:value={p.bPass} />
				</div>
				{#if !p.bPlan?.encrypted}
					<div class="ins-grow">
						<Label>Ещё раз</Label><Input type="p.password" autocomplete="off" bind:value={p.bPass2} />
					</div>
				{/if}
			</div>
			<p class="ins-dim">
				{p.bPlan?.encrypted
					? 'В этой папке бэкап с паролем — нужен тот же пароль, иначе бэкап скачается целиком заново в другую папку.'
					: 'Без пароля бэкап не открыть никак — запишите его.'}
			</p>
		{/if}
		<FootBar>
			<Button
				variant={p.bFolder &&
				p.passOk &&
				p.bPlan?.ok &&
				(p.bSite !== 'keep' || p.page.live_copy || !p.st.running)
					? 'colored'
					: 'off'}
				onclick={() => void p.attempt(() => p.startBackup())}>Сделать бэкап</Button
			>
			<Button variant="ghost" onclick={() => (p.screen = 'server')}>Назад</Button>
		</FootBar>
	{:else if p.screen === 'backup' && p.progress?.failure}
		<!-- Кадр 8г. -->
		<h2>{p.progress.failure.message}</h2>
		{#if p.progress.failure.advice}<Notice>{p.progress.failure.advice}</Notice>{/if}
		{#if p.progress.failure.step === 'fetch' && p.bPlan?.prev}
			<p class="ins-sub mt">Прошлый бэкап в папке не тронут, пока новый не скачан целиком.</p>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.startBackup())}>Повторить</Button>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.toServer())}>К серверу</Button>
			<TextButton class="link under ins-push" onclick={() => (p.showLog = !p.showLog)}
				>подробности</TextButton
			>
		</FootBar>
		{#if p.showLog}
			<CommandLog text={p.logText} />
		{/if}
	{:else if p.screen === 'backup'}
		<!-- Кадр 8б. -->
		<h2>Делаем бэкап</h2>
		<StepList meter class="ins-stage-steps" steps={p.progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>
				показать, что выполняется
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
		</FootBar>
		{#if p.showLog}
			<CommandLog text={p.logText} />
		{/if}
	{:else if p.screen === 'bdone'}
		<!-- Кадр 8в. -->
		<h2>Бэкап готов</h2>
		<p class="ins-sub">В папке {p.bFolder}.</p>
		<Notice>
			{p.bEncrypt
				? 'Бэкап зашифрован. Без пароля его не открыть.'
				: 'В бэкапе — все записи и ключи сервера. Храните его так же бережно, как пароли: кто получит папку, прочтёт все круги.'}
		</Notice>
		<FootBar>
			<Button
				variant="colored"
				onclick={() => void api.ShowInFolder(p.bFolder + '\\wynd-backup.json')}
			>
				Открыть папку
			</Button>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.toServer())}>К серверу</Button>
		</FootBar>
	{:else if p.screen === 'mconnect' && p.st}
		<h2>На какой сервер переезжаем</h2>
		<p class="ins-sub">
			Выберите сохранённый сервер или добавьте новый. Перед переездом проверим его заново.
		</p>
		{#each profiles.filter((q) => q.id !== profile.id) as dest (dest.id)}<Radio
				name={`destination-${profile.id}`}
				value={dest.id}
				bind:group={p.targetID}
				disabled={dest.host === profile.host ||
					Boolean(dest.busy && dest.reserved_by !== profile.id)}
				>{dest.name}<span class="ins-dim">
					— SSH: {dest.user}@{dest.host}{dest.busy && dest.reserved_by !== profile.id
						? ' · занят'
						: ''}</span
				></Radio
			>{/each}
		<TextButton class="link under" onclick={onaddtarget}>Добавить сервер</TextButton>
		{#if p.target && !p.target.use_keys}<Label>Пароль от нового сервера</Label><Input
				type="p.password"
				autocomplete="off"
				bind:value={p.mPass}
			/>{/if}
		{#if p.mFail}<Notice tone="bad" alert title={p.mFail.message} text={p.mFail.advice} />{/if}
		<FootBar
			><Button
				variant="colored"
				loading={p.mBusy}
				loadingLabel="Подключаемся…"
				disabled={!p.targetID}
				onclick={() => void p.attempt(p.moveConnect)}>Подключиться и проверить</Button
			><Button
				variant="ghost"
				class="ins-push"
				disabled={p.mBusy}
				onclick={() => void p.attempt(p.toServer)}>Отмена</Button
			></FootBar
		>
	{:else if p.screen === 'mfinger'}
		<h2>
			{p.mChangedFingerprint ? 'У нового сервера другой отпечаток' : 'Это ваш новый сервер?'}
		</h2>
		<p class="ins-sub">
			{#if p.mChangedFingerprint}Подключение остановлено. Сверьте новый отпечаток в панели хостинга.
				Если вы не переустанавливали сервер, не доверяйте новому ключу.{:else}Подключаемся к {p.target?.host ??
					''} впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга или панелью, если
				там он есть.{/if}
		</p>
		{#if p.mChangedFingerprint}<Label>Сохранённый отпечаток</Label><Notice mono
				>{p.mPreviousFingerprint}</Notice
			><Label>Новый отпечаток</Label>{/if}<Notice mono>{p.mFinger}</Notice>
		<FootBar>
			<Button
				variant="colored"
				loading={p.mBusy}
				loadingLabel="Подключаемся…"
				onclick={() => void p.attempt(() => p.moveConfirm())}
				>{p.mChangedFingerprint ? 'Проверил, доверять новому' : 'Да, подключиться'}</Button
			>
			<Button variant="ghost" disabled={p.mBusy} onclick={() => (p.screen = 'mconnect')}>Отмена</Button>
		</FootBar>
	{:else if p.screen === 'mplan' && !p.mplan}
		<h2>Что сделаем</h2>
		<p class="ins-dim">Осматриваем новый сервер…</p>
	{:else if p.screen === 'mplan' && p.mplan && !p.mplan.ok}
		<h2>Пока переехать нельзя</h2>
		<Notice alert title={p.mplan.message} text={p.mplan.advice} />
		<FootBar>
			<Button variant="ghost" onclick={() => (p.screen = 'mconnect')}>Другой сервер</Button>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.toServer())}>К серверу</Button>
		</FootBar>
	{:else if p.screen === 'mplan' && p.mplan}
		<!-- Кадры 9б и 9б2 (на старом Wynd не со своим Caddy — переезжают одни данные). -->
		{@const dataOnly = p.mplan.data_only}
		{@const oldProxy =
			({ nginx: 'nginx', apache: 'Apache', caddy: 'Caddy' } as Record<string, string>)[
				p.mplan.old_proxy ?? ''
			] ?? ''}
		<h2>Что сделаем</h2>
		<MarkRows>
			<MarkRow mark="1"
				>Поставим Wynd {p.mplan.version}{dataOnly ? ' и Caddy' : ''} на новый сервер
				<span class="ins-dim">— пока без данных и не запуская</span></MarkRow
			>
			<MarkRow mark="2"
				>Остановим сайт на старом <span class="ins-dim">— с этой минуты он не открывается</span
				></MarkRow
			>
			<MarkRow mark="3"
				>Перенесём {dataOnly ? 'данные' : 'папку'} Wynd на новый сервер
				<span class="ins-dim">— {gb(p.mplan.size)}, напрямую со старого, мимо этого компьютера</span
				></MarkRow
			>
			<MarkRow mark="4"
				>Вы смените адрес <span class="ins-dim"
					>— у записи A для {p.mplan.domain} поставите {p.mplan.new_ip}</span
				></MarkRow
			>
			<MarkRow mark="5"
				>{dataOnly
					? 'Запустим Wynd на новом, получим сертификат сайта и проверим сайт'
					: 'Запустим Wynd на новом и проверим сайт'}</MarkRow
			>
		</MarkRows>
		{#if dataOnly && oldProxy}
			<p class="ins-sub mt">
				Не тронем на старом: {oldProxy} и его сертификат.
			</p>
		{/if}
		{#if !p.mplan.foreign}
			<Label>Старый сервер</Label>
			<Radio name="mold" value={false} bind:group={p.mRemoveOld}>
				Оставить Wynd остановленным <span class="ins-dim"
					>— убрать потом, когда убедитесь, что всё на месте</span
				>
			</Radio>
			<Radio name="mold" value={true} bind:group={p.mRemoveOld}>
				Убрать Wynd после переезда <span class="ins-dim"
					>— со старого сервера уйдут его данные{dataOnly
						? ''
						: '; сертификат переедет на новый'}</span
				>
			</Radio>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.startMove())}>Переехать</Button>
			<Button variant="ghost" onclick={() => (p.screen = 'server')}>Назад</Button>
		</FootBar>
	{:else if p.screen === 'move' && p.progress?.failure}
		<h2>{p.progress.failure.message}</h2>
		{#if p.progress.failure.advice}<Notice>{p.progress.failure.advice}</Notice>{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.startMove())}>Повторить</Button>
			<Button
				variant="ghost"
				loading={p.mBusy}
				loadingLabel="Отменяем переезд…"
				onclick={() => void p.attempt(() => p.cancelMove())}>Отменить переезд</Button
			>
			<TextButton class="link under ins-push" onclick={() => (p.showLog = !p.showLog)}
				>подробности</TextButton
			>
		</FootBar>
		{#if p.showLog}<CommandLog text={p.logText} />{/if}
	{:else if p.screen === 'move'}
		<!-- Кадр 9в. -->
		{@const waitingDNS = p.progress?.steps?.some((s) => s.id === 'dns' && s.status === 'running')}
		<h2>Переезжаем</h2>
		<StepList meter class="ins-stage-steps" steps={p.progress?.steps ?? []} />
		{#if waitingDNS && p.mplan}
			<Notice>
				<div class="ins-dim">
					В панели, где покупали адрес {p.mplan.domain}, поменяйте у записи A адрес сервера на этот:
				</div>
				<CopyLine
					text={p.mplan.new_ip}
					copied={p.copiedLine === p.mplan.new_ip}
					oncopy={() => void p.copyLine(p.mplan?.new_ip ?? '')}
				/>
				<div class="ins-dim">Мастерская проверяет сама каждую минуту — окно можно свернуть.</div>
			</Notice>
			<FootBar>
				<Button variant="ghost" onclick={() => void api.MoveCheckNow()}>Проверить сейчас</Button>
				<TextButton class="link under ins-push" onclick={() => void p.attempt(() => p.cancelMove())}
					>Отменить переезд</TextButton
				>
			</FootBar>
		{:else}
			<FootBar>
				<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}
					>показать, что выполняется</TextButton
				>
				<span class="ins-dim ins-push">Окно можно свернуть</span>
			</FootBar>
		{/if}
		{#if p.showLog}<CommandLog text={p.logText} />{/if}
	{:else if p.screen === 'mdone' && p.mplan}
		<!-- Кадр 9г. -->
		<h2>Wynd на новом сервере</h2>
		<p class="ins-sub">
			Проверка прошла: {p.mplan.domain} открывается с сервера {p.mplan.new_ip}.
		</p>
		<Notice>
			{p.mRemoveOld
				? `Wynd со старого сервера ${p.mplan.old_ip} убран.`
				: `Старый сервер ${p.mplan.old_ip}: Wynd остановлен, данные на месте.${
						p.mplan.foreign
							? ''
							: ' Когда убедитесь, что на новом всё есть, уберите его со страницы старого сервера.'
					}`}
		</Notice>
		<FootBar>
			<Button variant="colored" onclick={() => void api.OpenLink(`https://${p.mplan?.domain}`)}>
				Открыть {p.mplan.domain}
			</Button>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.adoptNew())}>К серверу</Button>
		</FootBar>
	{:else if p.screen === 'fingerprint'}
		<h2>
			{p.changedFingerprint ? 'У сервера другой отпечаток' : 'Это ваш сервер?'}
		</h2>
		<p class="ins-sub">
			{#if p.changedFingerprint}Подключение остановлено. Сверьте новый отпечаток в панели хостинга.
				Если вы не переустанавливали сервер, не доверяйте новому ключу.{:else}Подключаемся к {host.trim()}
				впервые. Сервер назвал свой отпечаток — сверьте его с письмом хостинга или панелью, если там он
				есть.{/if}
		</p>
		{#if p.changedFingerprint}<Label>Сохранённый отпечаток</Label><Notice mono
				>{p.previousFingerprint}</Notice
			><Label>Новый отпечаток</Label>{/if}<Notice mono>{p.fingerprint}</Notice>
		<p class="ins-sub mt">
			Если отпечатка негде взять — это нормально для нового сервера. Мы запомним его, и при
			следующем подключении подмену заметим.
		</p>
		<FootBar>
			<Button
				variant="colored"
				loading={p.busy}
				loadingLabel="Подключаемся…"
				onclick={() => void p.attempt(() => p.confirmHost())}
			>
				{p.changedFingerprint ? 'Проверил, доверять новому' : 'Да, подключиться'}
			</Button>
			<Button variant="ghost" disabled={p.busy} onclick={() => void p.attempt(() => p.toConnect())}
				>Отмена</Button
			>
		</FootBar>
	{:else if p.screen === 'plan' && p.plan && !p.plan.ok}
		{#if p.rollbackNote}
			<Notice tone="note">{p.rollbackNote}</Notice>
		{/if}
		<h2>Пока ставить нельзя</h2>
		<Notice alert title={p.plan.message} text={p.plan.advice} />
		<FootBar>
			<Button variant="ghost" onclick={() => (p.screen = 'inspect')}>Назад</Button>
		</FootBar>
	{:else if p.screen === 'plan' && p.plan}
		<h2>Что сделаем</h2>
		<p class="ins-sub">До кнопки «Установить» на сервере ничего не меняется.</p>
		{#if p.rollbackNote}
			<Notice>{p.rollbackNote}</Notice>
		{/if}
		<div class="ins-sec">Поставим</div>
		{#each p.plan.install ?? [] as item (item)}
			<MarkRow mark="+" tone="ok">{item}</MarkRow>
		{/each}
		{#if p.plan.change?.length}
			<div class="ins-sec">Изменим</div>
			{#each p.plan.change as item (item)}
				<MarkRow mark="~" tone="note">{item}</MarkRow>
			{/each}
		{/if}
		{#if p.plan.keep?.length}
			<div class="ins-sec">Не тронем</div>
			{#each p.plan.keep as item (item)}
				<MarkRow mark="·">{item}</MarkRow>
			{/each}
		{/if}
		{#if p.plan.unpublished}
			<p class="ins-sub">Эту версию ещё нельзя скачать готовой — сервер соберёт Wynd сам.</p>
		{:else if p.plan.canBuild}
			<p class="ins-sub">
				{p.plan.build ? 'Сервер соберёт Wynd сам.' : 'Сервер скачает Wynd готовым.'}
				<TextButton
					class="link under"
					onclick={() => void p.attempt(() => p.chooseBuild(!p.plan?.build))}
				>
					{p.plan.build ? 'Скачать готовый' : 'Собрать самому'}
				</TextButton>
			</p>
		{/if}
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.install())}>Установить</Button>
			<Button variant="ghost" onclick={() => (p.screen = 'inspect')}>Назад</Button>
			<span class="ins-dim">{p.plan.duration}</span>
		</FootBar>
	{:else if p.screen === 'install' && p.progress?.failure}
		<h2>{p.progress.failure.message}</h2>
		<p class="ins-sub">{p.progress.failure.advice}</p>
		{#if p.progress.steps?.some((s) => s.status === 'done')}
			<Notice>Сделанное раньше осталось на месте — повтор начнётся с этого шага.</Notice>
		{/if}
		{#if p.askRollback}
			{@render removeBox('Да, откатить', '')}
		{:else}
			<FootBar>
				<Button variant="colored" onclick={() => void p.attempt(() => p.install())}>Повторить</Button>
				<Button variant="ghost" onclick={() => (p.askRollback = true)}>Откатить установку</Button>
				<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>подробности</TextButton>
			</FootBar>
		{/if}
		{#if p.showLog}
			<CommandLog text={p.logText} />
		{/if}
	{:else if p.screen === 'install'}
		<h2>Ставим</h2>
		<StepList meter class="ins-stage-steps" steps={p.progress?.steps ?? []} />
		<FootBar>
			<TextButton class="link under" onclick={() => (p.showLog = !p.showLog)}>
				показать, что выполняется
			</TextButton>
			<span class="ins-dim ins-push">Окно можно свернуть</span>
		</FootBar>
		{#if p.showLog}
			<CommandLog text={p.logText} />
		{/if}
	{:else if p.screen === 'done' && p.progress}
		<h2>Wynd работает</h2>
		<p class="ins-sub">Проверка прошла: сайт открывается, сертификат действует.</p>
		{#if p.askRollback}
			{@render removeBox('Да, убрать', p.plan?.domain ?? '')}
			{#if p.showLog && p.rollbackLog.length}
				<CommandLog text={p.logText} />
			{/if}
		{:else if p.progress.link}
			<Label>Ссылка первого запуска — для вас, одна</Label>
			<Notice mono>{p.progress.link}</Notice>
			<p class="ins-sub mt">
				По ней вы станете администратором сервера и создадите первый круг. Никому её не передавайте.
			</p>
			<FootBar>
				<Button variant="colored" onclick={() => void api.OpenLink(p.progress?.link ?? '')}>
					Открыть в браузере
				</Button>
				<Button variant="ghost" onclick={() => void p.copyLink()}>
					{p.copied ? 'Скопировано' : 'Скопировать'}
				</Button>
				<Button variant="ghost" onclick={() => void p.attempt(p.toServer)}>К серверу</Button>
				<TextButton class="link under ins-push" onclick={() => (p.askRollback = true)}>
					Убрать Wynd
				</TextButton>
			</FootBar>
		{:else}
			<Notice>
				Этот сервер уже настроен: администратор у него есть, ссылки первого запуска нет. Откройте
				сайт и войдите как обычно.
			</Notice>
			<FootBar>
				<Button variant="colored" onclick={() => void api.OpenLink(p.progress?.site ?? '')}>
					Открыть {p.plan?.domain}
				</Button>
				<Button variant="ghost" onclick={() => void p.attempt(p.toServer)}>К серверу</Button>
				<TextButton class="link under ins-push" onclick={() => (p.askRollback = true)}>
					Убрать Wynd
				</TextButton>
			</FootBar>
		{/if}
	{:else if !p.inspection}
		<h2>Что на сервере</h2>
		<p class="ins-sub">Только смотрим — ничего не меняем.</p>
		<p class="ins-dim">Осматриваем…</p>
	{:else if !p.inspection.ok}
		<h2>Осмотр не удался</h2>
		<Notice tone="bad" alert title={p.inspection.message} text={p.inspection.advice} />
		<FootBar>
			<Button variant="colored" onclick={() => void p.attempt(() => p.toConnect())}
				>Подключиться ещё раз</Button
			>
		</FootBar>
	{:else if p.blocks.length}
		<!-- Итог удаления не теряется, когда новый осмотр нашёл помеху (кадр 5д). -->
		{#if p.rollbackNote}
			<Notice tone="note">{p.rollbackNote}</Notice>
		{/if}
		<h2>Пока ставить нельзя</h2>
		<p class="ins-sub">
			{p.blocks.length === 1 ? 'Мешает одно:' : 'Мешает вот что:'}
		</p>
		{#each p.blocks as block (block.id)}
			<Notice title={block.text} text={block.advice}>
				{#if block.copy}
					<CopyLine
						text={block.copy}
						copied={p.copiedLine === block.copy}
						oncopy={() => void p.copyLine(block.copy ?? '')}
					/>
				{/if}
				{#if block.after}
					<div class="ins-dim">{block.after}</div>
				{/if}
			</Notice>
		{/each}
		<div class="ins-more">
			<TextButton class="link under" onclick={() => (p.showRaw = true)}>
				подробнее, что проверили
			</TextButton>
		</div>
		<FootBar>
			{#if p.blocks.every((b) => b.final)}
				<Button variant="colored" disabled={p.busy} onclick={() => void p.attempt(() => p.toConnect())}>
					Другой сервер
				</Button>
				<Button
					variant="ghost"
					loading={p.busy}
					loadingLabel="Осматриваем…"
					onclick={() => void p.attempt(() => p.inspect())}
				>
					Проверить снова
				</Button>
			{:else}
				<Button
					variant="colored"
					loading={p.busy}
					loadingLabel="Осматриваем…"
					onclick={() => void p.attempt(() => p.inspect())}
				>
					Проверить снова
				</Button>
				<Button variant="ghost" disabled={p.busy} onclick={() => void p.attempt(() => p.toConnect())}>
					Другой сервер
				</Button>
			{/if}
		</FootBar>
	{:else}
		<h2>Что на сервере</h2>
		<p class="ins-sub">Только смотрим — ничего не меняем.</p>
		{#if p.rollbackNote}
			<Notice>{p.rollbackNote}</Notice>
		{/if}
		<MarkRows>
			{#each p.findings as f (f.id)}
				<MarkRow mark={mark(f.level)} tone={f.level}>
					{f.text}{#if f.plan}&nbsp;— <span class="ins-dim"
							>{f.plan}{f.id === 'wynd' ? '.' : ''}</span
						>{/if}
					{#if f.id === 'wynd' && !p.askRollback}
						<TextButton class="link under" onclick={() => (p.askRollback = true)}>
							Убрать Wynd с сервера
						</TextButton>
					{/if}
				</MarkRow>
			{/each}
		</MarkRows>
		{#if p.askRollback}
			{@render removeBox('Да, убрать', p.domainOk ? p.domain : '')}
			{#if p.showLog && p.rollbackLog.length}
				<CommandLog text={p.logText} />
			{/if}
		{/if}
		<Label>Адрес, по которому будут заходить</Label>
		<form
			class="ins-p.domain"
			onsubmit={(e) => {
				e.preventDefault();
				void p.attempt(() => p.checkDomain());
			}}
		>
			<Input
				mono
				class="ins-grow"
				autocomplete="off"
				spellcheck={false}
				placeholder="family.example.ru"
				bind:value={p.domain}
				oninput={() => (p.domainCheck = undefined)}
			/>
			{#if p.domainOk}
				<span class="ins-mark ok wide">✓ {p.domainCheck?.text}</span>
			{:else}
				<Button
					variant="ghost"
					loading={p.checkingDomain}
					loadingLabel="Проверяем…"
					onclick={() => void p.attempt(() => p.checkDomain())}
				>
					{p.domainCheck ? 'Проверить снова' : 'Проверить'}
				</Button>
			{/if}
		</form>
		{#if p.domainCheck && !p.domainOk}
			<Notice alert title={p.domainCheck.text} text={p.domainCheck.advice} />
		{/if}
		{#if !p.st?.installed}
			<!-- Куда поставить Wynd: по умолчанию /opt/wynd, как у ручной установки. -->
			<Label>Каталог на сервере</Label>
			<Input mono autocomplete="off" spellcheck={false} bind:value={p.installRoot} />
		{/if}
		<FootBar>
			<Button
				variant={p.domainOk ? 'colored' : 'off'}
				loading={p.planning}
				loadingLabel="Составляем план…"
				onclick={() => void p.attempt(() => p.toPlan())}
			>
				Дальше — план
			</Button>
			<Button variant="ghost" onclick={() => void p.attempt(() => p.toConnect())}>Другой сервер</Button>
			{#if !p.domainOk}
				<span class="ins-dim">Сначала укажите адрес сайта</span>
			{/if}
			<TextButton class="link under" onclick={() => (p.showRaw = true)}>
				подробнее, что проверили
			</TextButton>
		</FootBar>
	{/if}
	{#if !p.occupied && (p.screen === 'server' || p.screen === 'new' || p.screen === 'offline' || p.screen === 'connect')}
	<FootBar class="ins-profile-tools"
		><TextButton
			class="link under"
			disabled={p.occupied || p.moving || Boolean(profile.reserved_by)}
			onclick={onedit}>Изменить доступ</TextButton
		><TextButton
			class="link under"
			disabled={p.occupied || p.moving || Boolean(profile.reserved_by)}
			onclick={onforget}>Убрать из списка</TextButton
		>{#if p.screen === 'server' && p.page && !p.page.foreign && !p.page.state.move && !p.askRollback}<TextButton
				class="link under"
				disabled={p.occupied}
				onclick={() => (p.askRollback = true)}>Убрать Wynd с сервера</TextButton
			>{/if}
	</FootBar>
	{/if}
</main>
{#if p.showRaw && p.inspection}
	<RawSheet
		parts={p.rawParts}
		copied={p.copiedRaw}
		oncopy={() => void p.copyRaw()}
		onclose={() => (p.showRaw = false)}
	/>
{/if}

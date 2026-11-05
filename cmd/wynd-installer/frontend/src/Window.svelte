<script lang="ts">
	import { untrack } from 'svelte';
	import TitleBar from '$ui/workshop/TitleBar.svelte';
	import WorkshopTabs from '$ui/workshop/WorkshopTabs.svelte';
	import StepNav from '$ui/workshop/StepNav.svelte';
	import SelfBar from '$ui/workshop/SelfBar.svelte';
	import Notice from '$ui/workshop/Notice.svelte';
	import FootBar from '$ui/workshop/FootBar.svelte';
	import MarkRows from '$ui/workshop/MarkRows.svelte';
	import MarkRow from '$ui/workshop/MarkRow.svelte';
	import Checkbox from '$ui/workshop/Checkbox.svelte';
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Input from '$ui/forms/Input.svelte';
	import App from './App.svelte';
	import Archives from './Archives.svelte';
	import {
		backend,
		type SelfUpdate,
		type ServerProfile,
		type ProfileRequest,
		type ProfileViewState
	} from './api';
	interface Runtime {
		WindowMinimise(): void;
		WindowToggleMaximise(): void;
		WindowIsMaximised(): Promise<boolean>;
		Quit(): void;
	}
	const rt = (window as unknown as { runtime?: Runtime }).runtime;
	let maximised = $state(false);
	let version = $state('');
	let tab = $state('servers');
	let profiles = $state<ServerProfile[]>([]);
	let states = $state<Record<string, ProfileViewState>>({});
	let selected = $state('');
	let loaded = $state(false);
	let error = $state('');
	let editing = $state<string | undefined>();
	let returnToMove = $state('');
	let requests = $state<Record<string, ProfileRequest | undefined>>({});
	let reconnect = $state<Record<string, number>>({});
	let forget = $state('');
	let formBusy = $state(false);
	let name = $state('');
	let host = $state('');
	let user = $state('root');
	let auth = $state('key');
	let password = $state('');
	let remember = $state(false);
	let auto = $state(false);
	const anyBusy = $derived(
		Object.values(states).some((s) => s.busy) || profiles.some((p) => p.busy)
	);
	const formAutoDisabled = $derived(auth === 'password' && !remember);
	const rail = $derived(
		profiles.map((p) => ({
			id: p.id,
			name: p.name,
			host: p.host,
			status: states[p.id]?.status ?? (p.reserved_by ? 'Назначение переезда' : 'Нет подключения'),
			busy: states[p.id]?.busy || p.busy
		}))
	);
	async function syncMaximised() {
		maximised = (await rt?.WindowIsMaximised().catch(() => false)) ?? false;
	}
	async function refreshProfiles() {
		try {
			profiles = await backend().Profiles();
			if (!loaded) selected = profiles[0]?.id ?? '';
			loaded = true;
		} catch (e) {
			error = String(e);
		}
	}
	void refreshProfiles();
	void syncMaximised();
	function add(targetFor = '') {
		error = '';
		returnToMove = targetFor;
		editing = '';
		name = '';
		host = '';
		user = 'root';
		auth = 'key';
		password = '';
		remember = false;
		auto = false;
		tab = 'servers';
	}
	function edit(id: string) {
		const p = profiles.find((p) => p.id === id);
		if (!p) return;
		editing = id;
		returnToMove = '';
		name = p.name;
		host = p.host;
		user = p.user;
		auth = p.use_keys ? 'key' : 'password';
		password = '';
		remember = p.remember;
		auto = p.auto_connect;
	}
	function cancelForm() {
		password = '';
		editing = undefined;
		if (returnToMove) selected = returnToMove;
		returnToMove = '';
	}
	async function save() {
		if (formBusy) return;
		formBusy = true;
		error = '';
		try {
			const p = await backend().SaveProfile({
				id: editing ?? '',
				name,
				host,
				user,
				use_keys: auth === 'key',
				remember,
				auto_connect: auto && !formAutoDisabled
			});
			const updatedProfiles = await backend().Profiles();
			const targetFor = returnToMove;
			requests[p.id] = {
				id: Date.now(),
				password,
				auto: auto && !formAutoDisabled
			};
			password = '';
			editing = undefined;
			returnToMove = '';
			profiles = updatedProfiles;
			loaded = true;
			selected = targetFor || p.id;
		} catch (e) {
			error = String(e);
		} finally {
			formBusy = false;
		}
	}
	async function remove() {
		if (!forget) return;
		try {
			const id = forget;
			await backend().RemoveProfile(id);
			forget = '';
			await refreshProfiles();
			if (selected === id) selected = profiles[0]?.id ?? '';
		} catch (e) {
			error = String(e);
		}
	}
	function select(id: string) {
		selected = id;
		editing = undefined;
		password = '';
		forget = '';
	}
	function report(id: string, s: ProfileViewState) {
		untrack(() => {
			states[id] = s;
		});
	}
	function requestConsumed(id: string) {
		requests[id] = undefined;
	}
	async function moved(id: string) {
		await refreshProfiles();
		reconnect[id] = (reconnect[id] ?? 0) + 1;
	}
	// Новая Мастерская (10а–10в): спрашиваем при запуске; «✕» прячет только
	// полосу — тихая строка внизу остаётся, пока не обновились (10а2).
	let selfUp = $state<SelfUpdate | undefined>();
	let selfHidden = $state(false);
	let selfOpen = $state(false);
	let selfBusy = $state(false);
	let selfBad = $state(false);
	let selfError = $state('');
	void backend()
		.CheckSelfUpdate()
		.then((u) => (selfUp = u.available ? u : undefined))
		.catch(() => undefined);
	void backend()
		.Version()
		.then((v) => (version = v))
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
</script>

<svelte:window onresize={() => void syncMaximised()} />
<div class="ins-win ins-servers" class:maximised>
	<TitleBar
		title={`Wynd Мастерская${version ? ` [${version}]` : ''}`}
		{maximised}
		onminimise={() => rt?.WindowMinimise()}
		ontoggle={() => rt?.WindowToggleMaximise()}
		onclose={() => rt?.Quit()}
	/>
	<WorkshopTabs
		tabs={[
			{ id: 'servers', label: 'Серверы' },
			{ id: 'archives', label: 'Архивы' }
		]}
		bind:value={tab}
	/>
	{#if selfOpen && selfUp}
		<!-- Кадры 10б, 10в. -->
		<div class="ins">
			<main class="ins-body">
				{#if selfBad}
					<h2>Эту версию Мастерская не поставит</h2>
					<Notice title="Мастерская {selfUp.version} не подписана ключом Wynd">
						<div class="ins-dim">
							Файл мог испортиться при скачивании — или его подменили. Ставить такую версию опасно:
							у Мастерской есть доступ к вашему серверу. Текущая Мастерская {version} работает как прежде.
						</div>
					</Notice>
					<FootBar>
						<Button
							variant="colored"
							loading={selfBusy}
							loadingLabel="Обновляем…"
							onclick={() => void applySelf()}>Попробовать ещё раз</Button
						>
						<Button variant="ghost" onclick={() => (selfOpen = false)}>Закрыть</Button>
						<TextButton class="link under ins-push" onclick={() => void backend().ReleasesPage()}
							>страница выпусков</TextButton
						>
					</FootBar>
				{:else}
					<h2>Мастерская {selfUp.version}</h2>
					{#if selfUp.notes?.length}
						<p class="ins-sub">Что нового:</p>
						<MarkRows>
							{#each selfUp.notes as n (n)}<MarkRow mark="·">{n}</MarkRow>{/each}
						</MarkRows>
					{/if}
					<MarkRows>
						<MarkRow mark="✓" tone="ok">
							Подписана ключом Wynd <span class="ins-dim"
								>— выложил тот, кто делает Wynd, файл не подменён</span
							>
						</MarkRow>
						<MarkRow mark="✓" tone="ok">
							Собрана из открытого кода тега workshop-v{selfUp.version} в GitHub Actions
							<TextButton
								class="link under"
								onclick={() => void backend().OpenLink(selfUp?.attestation ?? '')}
								>проверить</TextButton
							>
						</MarkRow>
					</MarkRows>
					{#if selfError}<Notice tone="bad" alert>{selfError}</Notice>{/if}
					<FootBar>
						<Button
							variant={anyBusy ? 'off' : 'colored'}
							loading={selfBusy}
							loadingLabel="Обновляем…"
							onclick={() => void (anyBusy ? undefined : applySelf())}
							>Обновить и перезапустить</Button
						>
						<Button variant="ghost" onclick={() => (selfOpen = false)}>Не сейчас</Button>
						{#if anyBusy}<span class="ins-dim">Сначала дождитесь окончания</span>{/if}
					</FootBar>
				{/if}
			</main>
		</div>
	{/if}
	<div class="ins-content" class:ins-hidden={selfOpen}>
		{#if selfUp && !selfHidden}<SelfBar
				version={selfUp.version ?? ''}
				onupdate={() => (selfOpen = true)}
				onhide={() => (selfHidden = true)}
			/>{/if}
		<div class="ins" class:ins-hidden={tab !== 'servers'}>
			<StepNav
				servers={rail}
				selected={editing !== undefined ? '' : selected}
				onselect={select}
				onadd={loaded ? () => add() : undefined}
			/>
			<div class="ins-workspace">
				{#if error}<Notice tone="bad" alert>{error}</Notice>{/if}
				{#if forget}
					<main class="ins-body ins-pane">
						<h2>Убрать сервер из списка?</h2>
						<p class="ins-sub">
							Удалим только сохранённый доступ к серверу {profiles.find((p) => p.id === forget)
								?.name}. Wynd и данные на сервере останутся.
						</p>
						<FootBar
							><Button variant="colored" onclick={() => void remove()}>Убрать из списка</Button
							><Button variant="ghost" class="ins-push" onclick={() => (forget = '')}>Отмена</Button
							></FootBar
						>
					</main>
				{:else if editing !== undefined}
					<main class="ins-body ins-pane">
						<div class="ins-operation-path">Серверы › {editing ? 'Доступ к серверу' : 'Добавить сервер'}</div>
						<h2>{editing ? 'Доступ к серверу' : 'Добавить сервер'}</h2>
						<p class="ins-sub">Укажите доступ по SSH. Мастерская проверит, установлен ли Wynd.</p>
						<form
							class="ins-profile-form"
							onsubmit={(e) => {
								e.preventDefault();
								void save();
							}}
						>
							<Label>Имя сервера</Label><Input
								bind:value={name}
								placeholder="Например, семейный сервер"
								disabled={formBusy}
							/>
							<Label>Адрес сервера</Label><Input
								bind:value={host}
								placeholder="185.12.34.56"
								spellcheck={false}
								autocomplete="off"
								disabled={formBusy}
							/>
							<div class="ins-form-pair">
								<div>
									<Label>Пользователь</Label><Input bind:value={user} disabled={formBusy} />
								</div>
								<div>
									<Label>Способ входа</Label><Input
										bind:value={auth}
										options={[
											{ value: 'key', label: 'Ключ с этого компьютера' },
											{ value: 'password', label: 'Пароль' }
										]}
										disabled={formBusy}
									/>
								</div>
							</div>
							<div
								class="ins-password-fields"
								class:ins-inactive={auth === 'key'}
								inert={auth === 'key'}
								aria-hidden={auth === 'key'}
							>
								<Label>Пароль от сервера</Label><Input
									type="password"
									autocomplete="off"
									bind:value={password}
									disabled={formBusy}
								/>
								<Checkbox native bind:checked={remember} disabled={formBusy}
									>Запомнить пароль на этом компьютере</Checkbox
								>
							</div>
							<p
								class="ins-credential-note"
								class:ins-inactive={!formAutoDisabled}
								aria-hidden={!formAutoDisabled}
							>
								Для автоподключения сохраните пароль или выберите ключ.
							</p>
							<FootBar
								><Button
									variant="colored"
									loading={formBusy}
									loadingLabel="Сохраняем…"
									disabled={!name.trim() || !host.trim() || !user.trim()}
									onclick={() => void save()}
									>{editing ? 'Сохранить и подключиться' : 'Подключиться'}</Button
								><Checkbox
									native
									checked={auto && !formAutoDisabled}
									disabled={formAutoDisabled || formBusy}
									onchange={(v) => (auto = v)}>Подключаться при запуске</Checkbox
								><Button variant="ghost" class="ins-push" disabled={formBusy} onclick={cancelForm}
									>Отмена</Button
								></FootBar
							>
						</form>
					</main>
				{:else if loaded && !selected}
					<main class="ins-body ins-pane">
						<div class="ins-welcome">
							<h2>Ваши серверы</h2>
							<p class="ins-sub">
								Добавьте сервер, чтобы установить Wynd или обслуживать готовую установку.
							</p>
						</div>
						<FootBar
							><Button variant="colored" onclick={() => add()}>Добавить сервер</Button></FootBar
						>
					</main>
				{/if}
				{#each profiles as profile (profile.id)}
					<div
						class="ins-profile-view"
						class:ins-hidden={selected !== profile.id || editing !== undefined || Boolean(forget)}
					>
						<App
							{profile}
							{profiles}
							request={requests[profile.id]}
							reconnect={reconnect[profile.id] ?? 0}
							onconsumed={() => requestConsumed(profile.id)}
							onstate={(s) => report(profile.id, s)}
							onprofileschange={refreshProfiles}
							onedit={() => edit(profile.id)}
							onforget={() => (forget = profile.id)}
							onaddtarget={() => add(profile.id)}
							onselect={select}
							onmoved={moved}
						/>
					</div>
				{/each}
			</div>
		</div>
		<div class="ins-archive-view" class:ins-hidden={tab !== 'archives'}>
			<Archives />
		</div>
		{#if selfUp && selfHidden}<div class="ins-self-note">
				<TextButton class="link under" onclick={() => (selfOpen = true)}
					>Вышла Мастерская {selfUp.version}</TextButton
				>
			</div>{/if}
	</div>
</div>

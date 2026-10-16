<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { goUp } from '$lib/navigation/up';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import Loading from '$ui/Loading.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { fetchInstance } from '$lib/auth/auth';
	import { WORD, plural } from '$lib/format/plural';
	import { loadSessions, removeSession } from '$lib/session/session.svelte';
	import { stopSync } from '$lib/sync/sync';
	import {
		clearPendingLeave,
		fetchLeavePreview,
		leaveConfirmText,
		leaveErrorHint,
		leaveServer,
		loadPendingLeave,
		ownedCircleSubtitle,
		type LeavePreview,
		type PendingLeave
	} from '$lib/auth/leaveServer';

	let pending = $state<PendingLeave | undefined>();
	let preview = $state<LeavePreview | undefined>();
	let erasureAllowed = $state(false);
	let eraseNames = $state(false);
	let loadError = $state('');
	let confirmOpen = $state(false);
	let confirmName = $state('');
	let leaving = $state(false);
	let leaveError = $state('');

	const serverLabel = $derived(pending ? pending.serverName || pending.host : '');
	const owns = $derived((preview?.owned.length ?? 0) > 0);
	const typedRight = $derived(confirmName.trim() === serverLabel);

	onMount(() => {
		pending = loadPendingLeave();
		if (!pending) {
			goto('/settings/servers');
			return;
		}
		void load(pending.origin);
	});

	async function load(origin: string) {
		loadError = '';
		try {
			const [next, instance] = await Promise.all([
				fetchLeavePreview(origin),
				fetchInstance(origin).catch(() => undefined)
			]);
			preview = next;
			erasureAllowed = Boolean(instance?.allow_name_erasure);
		} catch {
			loadError = 'Сервер не отвечает — уйти с него можно, когда связь появится.';
		}
	}

	function back() {
		clearPendingLeave();
		void goUp('/settings/servers');
	}

	function openConfirm() {
		confirmName = '';
		leaveError = '';
		confirmOpen = true;
	}

	async function onLeave() {
		if (!pending || !typedRight) return;
		leaving = true;
		leaveError = '';
		try {
			await leaveServer(pending.origin, erasureAllowed && eraseNames);
		} catch (err) {
			leaveError = leaveErrorHint(err);
			leaving = false;
			return;
		}
		stopSync(pending.origin);
		await removeSession(pending.origin);
		clearPendingLeave();
		const left = await loadSessions();
		leaving = false;
		confirmOpen = false;
		// Серверов не осталось — первый запуск (7.16).
		await goto(left.length > 0 ? '/settings/servers' : '/', { replaceState: true });
	}
</script>

<FormLayout shell app title="Уйти с сервера насовсем" onback={back}>
	{#if pending}
		<ServerRow
			name={serverLabel}
			subtitle={pending.host}
			card
			gdpr={erasureAllowed}
			top={10}
		/>
		<SettingsRow
			title={pending.email}
			subtitle={preview ? plural(preview.circles, WORD.circle) : undefined}
			chevron={false}
		/>
		{#if loadError}
			<Hint>{loadError}</Hint>
		{:else if !preview}
			<Loading />
		{:else if owns}
			<SectionLabel class="mt-8">Сначала — ваши круги</SectionLabel>
			<Hint class="mt-0 ink">
				Без владельца круг остаться не может.
			</Hint>
			{#each preview.owned as c, i (c.id)}
				<SettingsRow
					class={i === 0 ? 'mt-8' : undefined}
					title={c.name}
					subtitle={ownedCircleSubtitle(c)}
					divided={preview.owned.length === 1 ? 'both' : i === 0 ? 'top' : i === preview.owned.length - 1 ? 'bottom' : undefined}
					onclick={() =>
						goto(
							c.members > 1
								? `/circles/${c.id}/settings/members?transfer=1`
								: `/circles/${c.id}/settings`
						)}
				/>
			{/each}
			<!-- Правило видно, как на 6.39: кнопка на месте, но не нажимается. -->
			<DangerZone class="mt-22" disabled items={[`Уйти с «${serverLabel}» насовсем`]} />
		{:else}
			<SectionLabel class="mt-8">Исчезнет</SectionLabel>
			<Hint class="mt-0 ink">
				Вход на всех ваших устройствах. Вернуться с этой почтой можно — как новый человек.
			</Hint>
			<SectionLabel class="mt-18">Останется в кругах</SectionLabel>
			<Hint class="mt-0 ink">
				Всё, что вы написали и выложили, — под вашими именами.
			</Hint>
			{#if erasureAllowed}
				<SettingsRow
					class="mt-14"
					title="Стереть моё имя и фото во всех кругах"
					subtitle="записи останутся без имени"
					divided="both"
				>
					{#snippet control()}
						<Switch bind:checked={eraseNames} label="Стереть моё имя и фото во всех кругах" />
					{/snippet}
				</SettingsRow>
			{/if}
			<DangerZone
				class="mt-18"
				items={[`Уйти с «${serverLabel}» насовсем`]}
				onitem={openConfirm}
			/>
		{/if}
	{/if}
</FormLayout>

{#if confirmOpen && pending && preview}
	<ConfirmDialog
		title="Уйти с «{serverLabel}» насовсем?"
		confirmLabel="Уйти"
		confirmDisabled={!typedRight}
		loading={leaving}
		onconfirm={() => void onLeave()}
		oncancel={() => (confirmOpen = false)}
	>
		<Hint class="mb-14">{leaveConfirmText(preview.circles, erasureAllowed && eraseNames)}</Hint>
		<Input active placeholder={serverLabel} bind:value={confirmName} />
		{#if leaveError}
			<Hint class="mt-8">{leaveError}</Hint>
		{/if}
	</ConfirmDialog>
{/if}

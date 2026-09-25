<script lang="ts">
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		inviteRegistrySubtitle,
		inviteRegistryTitle,
		isLiveInvite
	} from '$lib/circles/invite-registry';
	import {
		createServerInvite,
		fetchAccess,
		fetchInvites,
		revokeInvite,
		saveAccess,
		serverCaption,
		type AccessSettings,
		type AdminInvite
	} from '$lib/admin/admin';

	const modes: { key: AccessSettings['registration_mode']; label: string }[] = [
		{ key: 'open', label: 'Открытый' },
		{ key: 'invite', label: 'По приглашению' },
		{ key: 'closed', label: 'Закрытый' }
	];

	const TTL_OPTIONS = [
		{ label: '1 час', sec: 3600 },
		{ label: '72 часа', sec: 259200 },
		{ label: 'Неделя', sec: 604800 }
	] as const;

	let mode = $state<AccessSettings['registration_mode']>('invite');
	let server = $state('');
	let inviteUrl = $state('');
	let qrSvg = $state('');
	let liveInvites = $state<AdminInvite[]>([]);
	let kind = $state<'single' | 'multi'>('multi');
	let ttlSec = $state(259200);
	let error = $state('');
	let loading = $state(true);
	let copied = $state(false);
	let copiedInviteId = $state('');
	let currentInviteId = $state('');
	let creating = false;

	function inviteUrlFor(token: string): string {
		const base = typeof window !== 'undefined' ? window.location.origin : '';
		return `${base}/join/${token}`;
	}

	async function renderQr(url: string) {
		qrSvg = url ? await QRCode.toString(url, { type: 'svg', margin: 0, width: 142 }) : '';
	}

	async function loadLiveInvites() {
		const invites = await fetchInvites();
		liveInvites = invites.filter(isLiveInvite);
	}

	async function persistMode(next: AccessSettings['registration_mode']) {
		mode = next;
		try {
			await saveAccess({ registration_mode: next });
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function makeInvite() {
		if (creating) return;
		creating = true;
		copied = false;
		const previousId = currentInviteId;
		currentInviteId = '';
		try {
			if (previousId) {
				try {
					await revokeInvite(previousId);
				} catch {
					/* already used or revoked */
				}
			}
			const inv = await createServerInvite({
				kind,
				max_uses: kind === 'single' ? 1 : 5,
				ttl_sec: ttlSec
			});
			inviteUrl = inviteUrlFor(inv.token);
			await renderQr(inviteUrl);
			await loadLiveInvites();
			currentInviteId = liveInvites.find((row) => row.token === inv.token)?.id ?? '';
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			creating = false;
		}
	}

	async function copyLink() {
		if (!inviteUrl) return;
		await navigator.clipboard.writeText(inviteUrl);
		copied = true;
	}

	async function copyInvite(inv: AdminInvite) {
		await navigator.clipboard.writeText(inviteUrlFor(inv.token));
		copiedInviteId = inv.id;
	}

	async function revokeLiveInvite(inv: AdminInvite) {
		try {
			await revokeInvite(inv.id);
			await loadLiveInvites();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const [access, caption] = await Promise.all([fetchAccess(), serverCaption()]);
			mode = access.registration_mode;
			server = caption;
			await loadLiveInvites();
			await makeInvite();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Доступ" {server}>
	<AdminSection title="Доступ">
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else}
			<div class="cols">
				<div>
					<SectionLabel class="mt-0 mx-0 mb-8">Кого пускать</SectionLabel>
					<ChipGroup class="m-0">
						{#each modes as item (item.key)}
							<Chip selected={mode === item.key} onclick={() => void persistMode(item.key)}>
								{item.label}
							</Chip>
						{/each}
					</ChipGroup>
					<div class="note mt-10 lh-16">
						Завестись можно только по ссылке: в круг её выдаёт любой участник, на сервер — вы.
						Открытый пускает всякого, кто знает адрес; закрытый не пускает никого, и старые ссылки
						перестают работать.
					</div>
					<SectionLabel class="mt-24 mx-0 mb-8">Позвать на сервер</SectionLabel>
					<div class="flex-mid gap-12">
						<FieldDisplay
							admin
							mono
							value={inviteUrl || '…'}
							class="grow clip nowrap m-0"
						/>
						<TextButton variant="adminBox" class="bold" onclick={() => void copyLink()}>
							{copied ? 'Скопировано' : 'Скопировать'}
						</TextButton>
					</div>
					<ChipGroup class="mt-12 mx-0">
						<Chip
							selected={kind === 'single'}
							onclick={() => {
								kind = 'single';
								void makeInvite();
							}}
						>
							Одноразовая
						</Chip>
						<Chip
							selected={kind === 'multi'}
							onclick={() => {
								kind = 'multi';
								void makeInvite();
							}}
						>
							На 5 человек
						</Chip>
					</ChipGroup>
					<ChipGroup class="mt-8 mx-0">
						{#each TTL_OPTIONS as opt (opt.sec)}
							<Chip
								selected={ttlSec === opt.sec}
								onclick={() => {
									ttlSec = opt.sec;
									void makeInvite();
								}}
							>
								{opt.label}
							</Chip>
						{/each}
					</ChipGroup>
					<div class="fine mt-10 lh-16">
						Такая ссылка не ведёт ни в один круг: человек заведёт свой или дождётся, когда позовут.
					</div>
					{#if liveInvites.length > 0}
						<SectionLabel class="mt-24 mx-0 mb-8">Живые</SectionLabel>
						{#each liveInvites as inv (inv.id)}
							<SettingsRow
								title={inviteRegistryTitle(inv)}
								subtitle={inviteRegistrySubtitle(inv)}
								chevron={false}
								class="pt-2"
							>
								{#snippet control()}
									<span class="flex gap-12 no-shrink">
										<TextButton variant="admin" onclick={() => void copyInvite(inv)}>
											{copiedInviteId === inv.id ? 'скопировано' : 'скопировать'}
										</TextButton>
										<TextButton variant="admin" onclick={() => void revokeLiveInvite(inv)}>
											отозвать
										</TextButton>
									</span>
								{/snippet}
							</SettingsRow>
						{/each}
					{/if}
				</div>
				<div style="flex:0 0 auto;width:210px">
					{#if qrSvg}
						<div class="qr sm mt-26" aria-hidden="true">
							{@html qrSvg}
						</div>
					{/if}
					<div class="sz-11 faint ctr mt-10">
						та же ссылка кодом
					</div>
				</div>
			</div>
			<div
				class="fine mt-22 lh-16"
				style="border-top:1px solid var(--line);padding-top:14px;max-width:620px"
			>
				Записи и медиа лежат на этом диске незашифрованными: у вас есть база и файлы, а значит, вы
				можете прочитать что угодно. Панель этого не показывает и не будет, но и гарантией это не
				притворяется — то же самое написано людям на экране, где они заводят здесь круг.
			</div>
			{#if error}
				<Hint class="mt-12">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

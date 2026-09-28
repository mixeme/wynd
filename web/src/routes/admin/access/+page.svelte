<script lang="ts">
	import { copyText } from '$lib/clipboard';
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Input from '$ui/forms/Input.svelte';
	import {
		choicesFromInvite,
		CUSTOM_DAYS_MAX,
		CUSTOM_USES_MAX,
		inviteRequest,
		TTL_PRESETS,
		USES_PRESETS,
		type TtlChoice,
		type UsesChoice
	} from '$lib/circles/invite-options';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
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


	let mode = $state<AccessSettings['registration_mode']>('invite');
	let server = $state('');
	let inviteUrl = $state('');
	let qrSvg = $state('');
	let liveInvites = $state<AdminInvite[]>([]);
	let uses = $state<UsesChoice>(5);
	let customUses = $state(20);
	let ttl = $state<TtlChoice>(259200);
	let customDays = $state(14);
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
			const inv = await createServerInvite(inviteRequest(uses, customUses, ttl, customDays));
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

	function pickUses(next: UsesChoice) {
		uses = next;
		void makeInvite();
	}

	function pickTtl(next: TtlChoice) {
		ttl = next;
		void makeInvite();
	}

	async function showInvite(inv: AdminInvite) {
		({ uses, customUses, ttl, customDays } = choicesFromInvite(inv));
		currentInviteId = inv.id;
		inviteUrl = inviteUrlFor(inv.token);
		await renderQr(inviteUrl);
	}

	async function copyLink() {
		if (!inviteUrl) return;
		await copyText(inviteUrl);
		copied = true;
	}

	async function copyInvite(inv: AdminInvite) {
		await copyText(inviteUrlFor(inv.token));
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
			// Заход на экран не выпускает ссылку: раньше каждое открытие
			// «Доступа» создавало новую многоразовую и копило живые. Есть
			// живая — показываем свежую; нет — выпускаем одну.
			const latest = liveInvites[0];
			if (latest) {
				await showInvite(latest);
			} else {
				await makeInvite();
			}
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
			<Loading compact />
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
					<!-- Опции те же, что у участника на 2.7; «Без ограничений» и «Без срока»
					     — только здесь: вечная дверь на сервер — решение админа. -->
					<ChipGroup class="mt-12 mx-0">
						<Chip selected={uses === 'single'} onclick={() => pickUses('single')}>Одноразовая</Chip>
						{#each USES_PRESETS as n (n)}
							<Chip selected={uses === n} onclick={() => pickUses(n)}>На {n} человек</Chip>
						{/each}
						<Chip selected={uses === 'unlimited'} onclick={() => pickUses('unlimited')}
							>Без ограничений</Chip
						>
						<Chip selected={uses === 'custom'} onclick={() => pickUses('custom')}>Своё…</Chip>
					</ChipGroup>
					{#if uses === 'custom'}
						<div class="flex-mid gap-8 mt-8">
							<Input
								admin
								class="w72"
								type="number"
								min="1"
								max={String(CUSTOM_USES_MAX)}
								bind:value={customUses}
								onchange={() => void makeInvite()}
							/>
							<span class="note">человек, до {CUSTOM_USES_MAX}</span>
						</div>
					{/if}
					<ChipGroup class="mt-8 mx-0">
						{#each TTL_PRESETS as opt (opt.sec)}
							<Chip selected={ttl === opt.sec} onclick={() => pickTtl(opt.sec)}>{opt.label}</Chip>
						{/each}
						<Chip selected={ttl === 'forever'} onclick={() => pickTtl('forever')}>Без срока</Chip>
						<Chip selected={ttl === 'custom'} onclick={() => pickTtl('custom')}>Своё…</Chip>
					</ChipGroup>
					{#if ttl === 'custom'}
						<div class="flex-mid gap-8 mt-8">
							<Input
								admin
								class="w72"
								type="number"
								min="1"
								max={String(CUSTOM_DAYS_MAX)}
								bind:value={customDays}
								onchange={() => void makeInvite()}
							/>
							<span class="note">дней, до {CUSTOM_DAYS_MAX}</span>
						</div>
					{/if}
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

<script lang="ts">
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		createServerInvite,
		fetchAccess,
		saveAccess,
		serverCaption,
		type AccessSettings
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

	let name = $state('');
	let mode = $state<AccessSettings['registration_mode']>('invite');
	let server = $state('');
	let inviteUrl = $state('');
	let qrSvg = $state('');
	let kind = $state<'single' | 'multi'>('multi');
	let ttlSec = $state(259200);
	let error = $state('');
	let loading = $state(true);
	let copied = $state(false);

	async function renderQr(url: string) {
		qrSvg = url ? await QRCode.toString(url, { type: 'svg', margin: 0, width: 142 }) : '';
	}

	async function persistName() {
		const trimmed = name.trim();
		if (!trimmed) return;
		try {
			await saveAccess({ name: trimmed });
			server = await serverCaption();
		} catch (err) {
			error = authErrorHint(err);
		}
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
		copied = false;
		try {
			const inv = await createServerInvite({
				kind,
				max_uses: kind === 'single' ? 1 : 5,
				ttl_sec: ttlSec
			});
			const base = typeof window !== 'undefined' ? window.location.origin : '';
			inviteUrl = `${base}/join/${inv.token}`;
			await renderQr(inviteUrl);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function copyLink() {
		if (!inviteUrl) return;
		await navigator.clipboard.writeText(inviteUrl);
		copied = true;
	}

	onMount(async () => {
		try {
			const [access, caption] = await Promise.all([fetchAccess(), serverCaption()]);
			name = access.name;
			mode = access.registration_mode;
			server = caption;
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
		{:else if error && !name}
			<Hint>{error}</Hint>
		{:else}
			<div class="cols">
				<div>
					<SectionLabel style="margin:0 0 8px">Имя сервера</SectionLabel>
					<Input
						admin
						style="margin-left:0;width:100%"
						bind:value={name}
						onchange={() => void persistName()}
					/>
					<div style="font-size:11.5px;color:var(--faint);margin-top:8px;line-height:1.6">
						Так сервер назван в приложении. Адрес люди видят второй строкой и почти никогда не
						набирают.
					</div>
					<SectionLabel style="margin:24px 0 8px">Кого пускать</SectionLabel>
					<ChipGroup style="margin:0">
						{#each modes as item (item.key)}
							<Chip selected={mode === item.key} onclick={() => void persistMode(item.key)}>
								{item.label}
							</Chip>
						{/each}
					</ChipGroup>
					<div style="font-size:12.5px;color:var(--muted);margin-top:10px;line-height:1.6">
						Учётка заводится только по ссылке: в круг её выдаёт любой участник, на сервер — вы.
						Открытый пускает всякого, кто знает адрес; закрытый не пускает никого, и старые ссылки
						перестают работать.
					</div>
					<SectionLabel style="margin:24px 0 8px">Позвать на сервер</SectionLabel>
					<div style="display:flex;align-items:center;gap:12px">
						<FieldDisplay
							admin
							mono
							value={inviteUrl || '…'}
							style="flex:1;overflow:hidden;white-space:nowrap;margin:0"
						/>
						<TextButton variant="admin" onclick={() => void copyLink()}>
							{copied ? 'Скопировано' : 'Скопировать'}
						</TextButton>
					</div>
					<ChipGroup style="margin:12px 0 0">
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
					<ChipGroup style="margin:8px 0 0">
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
					<div style="font-size:11.5px;color:var(--faint);margin-top:10px;line-height:1.6">
						Такая ссылка не ведёт ни в один круг: человек заведёт свой или дождётся, когда позовут.
					</div>
				</div>
				<div style="flex:0 0 auto;width:210px">
					{#if qrSvg}
						<div class="qr" style="margin:26px auto 0;width:150px;height:150px;padding:11px" aria-hidden="true">
							{@html qrSvg}
						</div>
					{/if}
					<div style="font-size:11.5px;color:var(--faint);text-align:center;margin-top:10px">
						та же ссылка кодом
					</div>
				</div>
			</div>
			<div
				style="font-size:11.5px;color:var(--faint);margin-top:22px;line-height:1.6;border-top:1px solid var(--line);padding-top:14px;max-width:620px"
			>
				Записи и медиа лежат на этом диске незашифрованными: у вас есть база и файлы, а значит, вы
				можете прочитать что угодно. Панель этого не показывает и не будет, но и гарантией это не
				притворяется — то же самое написано людям на экране, где они заводят здесь круг.
			</div>
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

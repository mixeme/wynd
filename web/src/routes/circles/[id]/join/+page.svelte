<script lang="ts">
	import { getContext, onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import PeopleStrip from '$ui/forms/PeopleStrip.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		clearInviteJoinToken,
		fetchInvitePeek,
		joinViaInvite,
		loadInviteJoinToken,
		memberAvatarColor,
		memberSubtitle,
		type InvitePeek
	} from '$lib/auth/invites';
	import { fetchJoinPreview, joinPendingCircle } from '$lib/circles/settings';
	import { circleInitial, setCircleIdentity } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { fetchMembers, updateIdentity } from '$lib/circles/settings';
	import { uploadBlob } from '$lib/journal/posts';
	import type { CroppedImage } from '$lib/media/crop';
	import type { MemberInfo } from '$lib/circles/settings';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const showMembers = $derived($page.url.searchParams.get('members') === '1');

	let peek = $state<InvitePeek | undefined>();
	let members = $state<MemberInfo[]>([]);
	let name = $state('');
	let firstPost = $state('');
	let loading = $state(false);
	let error = $state('');
	let inviteToken = $state<string | undefined>();
	let pendingJoin = $state(false);
	let cropFile = $state<File | undefined>();
	let pendingAvatar = $state<CroppedImage | undefined>();
	let avatarPreview = $state('');
	let fileInput: HTMLInputElement | undefined = $state();

	const displayMembers = $derived.by(() => {
		if (peek?.members) {
			return peek.members.slice(0, 5).map((m, i) => ({
				name: m.name,
				initial: circleInitial(m.name),
				color: memberAvatarColor(i)
			}));
		}
		return members
			.filter((m) => m.status === 'active')
			.slice(0, 5)
			.map((m, i) => ({
				name: m.name,
				initial: circleInitial(m.name),
				color: memberAvatarColor(i)
			}));
	});

	const memberTotal = $derived(peek?.member_count ?? members.filter((m) => m.status === 'active').length);
	const moreCount = $derived(Math.max(0, memberTotal - displayMembers.length));

	onMount(async () => {
		inviteToken = loadInviteJoinToken(circle.circleId);
		try {
			if (inviteToken) {
				peek = await fetchInvitePeek(circle.origin, inviteToken);
			} else {
				try {
					const preview = await fetchJoinPreview(circle.origin, circle.circleId);
					pendingJoin = true;
					peek = {
						server_name: '',
						host: '',
						circle_name: preview.circle_name,
						color: preview.color,
						member_count: preview.member_count,
						members: preview.members
					};
				} catch {
					members = await fetchMembers(circle.origin, circle.circleId);
				}
			}
		} catch (err) {
			error = authErrorHint(err);
		}
	});

	function openMembers() {
		goto(`/circles/${circle.circleId}/join?members=1`);
	}

	function closeMembers() {
		goto(`/circles/${circle.circleId}/join`);
	}

	function openPhotoPicker() {
		fileInput?.click();
	}

	function onPhotoSelected(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		if (file.type && !file.type.startsWith('image/')) {
			error = 'Нужно фото';
			return;
		}
		cropFile = file;
	}

	function onCropDone(crop: CroppedImage) {
		cropFile = undefined;
		if (avatarPreview) URL.revokeObjectURL(avatarPreview);
		pendingAvatar = crop;
		avatarPreview = URL.createObjectURL(new Blob([crop.data], { type: crop.type }));
	}

	function onCropCancel() {
		cropFile = undefined;
	}

	async function uploadPendingAvatar(): Promise<boolean> {
		if (!pendingAvatar) return true;
		const crop = pendingAvatar;
		pendingAvatar = undefined;
		try {
			const blobId = await uploadBlob(circle.origin, crop);
			await updateIdentity(circle.origin, circle.circleId, { avatar_blob_id: blobId });
			return true;
		} catch {
			return false;
		}
	}

	async function enterCircle() {
		error = '';
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Введите имя';
			return;
		}
		const hadAvatar = Boolean(pendingAvatar);
		loading = true;
		try {
			if (inviteToken) {
				await joinViaInvite(circle.origin, inviteToken, {
					name: trimmed,
					body: firstPost.trim() || undefined
				});
				clearInviteJoinToken(circle.circleId);
			} else if (pendingJoin) {
				await joinPendingCircle(circle.origin, circle.circleId, {
					name: trimmed,
					body: firstPost.trim() || undefined
				});
			}
			await setCircleIdentity(circle.origin, circle.circleId, trimmed);
			const avatarOk = await uploadPendingAvatar();
			rememberCircleOrigin(circle.circleId, circle.origin);
			circle.identityName = trimmed;
			circle.identityInitial = circleInitial(trimmed);
			await circle.refresh();
			const avatarQuery = hadAvatar && !avatarOk ? '?joinAvatar=fail' : '';
			goto(`/circles/${circle.circleId}${avatarQuery}`);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	async function skipToFeed() {
		goto(`/circles/${circle.circleId}`);
	}

	onDestroy(() => {
		if (avatarPreview) URL.revokeObjectURL(avatarPreview);
	});
</script>

{#if showMembers && peek?.members}
	<FormLayout
		app
		color={circle.color}
		title="Кто уже здесь"
		subtitle={circle.name}
		right={String(peek.member_count)}
		onback={closeMembers}
	>
		{#each peek.members as member, i (member.name)}
			<MemberRow
				initial={circleInitial(member.name)}
				name={member.name}
				subtitle={memberSubtitle(member)}
				color={memberAvatarColor(i)}
			/>
		{/each}
	</FormLayout>
{:else}
<FormLayout app color={circle.color} circleTitle={circle.name}>
	{#if memberTotal > 0}
		<Label style="margin-top:16px">Кто уже здесь · {memberTotal}</Label>
		<PeopleStrip people={displayMembers} />
		{#if moreCount > 0 && (inviteToken || pendingJoin)}
			<div class="hint ctr" style="margin-top:12px">
				<TextButton onclick={openMembers}>ещё {moreCount}</TextButton>
			</div>
		{:else if moreCount > 0}
			<div class="hint ctr" style="margin-top:12px">ещё {moreCount}</div>
		{/if}
	{/if}

	{#if inviteToken || pendingJoin}
		<div class="h1s" style="margin-top:22px;line-height:1.25">
			Как вас зовут<br />в этом круге?
		</div>
		<AddPhotoButton previewUrl={avatarPreview || undefined} onclick={openPhotoPicker} />
		<div class="hint ctr" style="margin-top:8px">
			<TextButton onclick={openPhotoPicker}>добавить фото</TextButton>
		</div>
		<input bind:this={fileInput} type="file" accept="image/*" hidden onchange={onPhotoSelected} />
		<Label style="margin-top:18px">Имя</Label>
		<Input active type="text" autocomplete="name" bind:value={name} />
		<Label style="display:flex">
			<span>Скажи что-нибудь кругу</span>
			<span style="margin-left:auto;text-transform:none;letter-spacing:0;font-weight:400"
				>необязательно</span
			>
		</Label>
		<TextArea
			variant="area"
			active
			rows={3}
			bind:value={firstPost}
			placeholder="Первая запись в журнале круга"
		/>
		<Button variant="colored" {loading} onclick={enterCircle}>Войти в круг</Button>
	{:else}
		<div class="h1s" style="margin-top:22px;line-height:1.25">
			Добро пожаловать<br />в {circle.name}
		</div>
		<Hint style="margin-top:12px">
			Здесь вас зовут «{circle.identityName}». Первую запись можно сделать в ленте.
		</Hint>
		<Button variant="colored" onclick={skipToFeed}>Войти в круг</Button>
	{/if}

	{#if error}
		<Hint style="margin-top:12px">{error}</Hint>
	{/if}
</FormLayout>
{/if}

{#if cropFile}
	<AvatarCrop
		file={cropFile}
		color={circle.color}
		ondone={onCropDone}
		oncancel={onCropCancel}
	/>
{/if}

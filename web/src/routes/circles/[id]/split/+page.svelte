<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Avatar from '$ui/data/Avatar.svelte';
	import Row from '$ui/data/Row.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Loading from '$ui/Loading.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { circleInitial } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { fetchSplitPreview, splitCircle, type SplitPreview } from '$lib/circles/settings';
	import { plural, WORD } from '$lib/format/plural';
	import { formatCutoffDate } from '$lib/format/time';
	import { localDayOf } from '$lib/journal/present';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { CIRCLE_COLORS } from '$lib/theme/colors';
	import { getSession } from '$lib/idb/db';

	// «Отделиться» (план 46, A9; кадры 6.42 и 6.43): новый круг на этом же
	// сервере из того, что вы видели, и те, кого позовёте. В этом круге
	// ничего не меняется.
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let preview = $state<SplitPreview | undefined>();
	let loading = $state(true);
	let error = $state('');
	let step = $state<'what' | 'who'>('what');
	let name = $state('');
	let invite = $state<Record<string, boolean>>({});
	let showAll = $state(false);
	let saving = $state(false);
	let avatarUrls = $state<Record<string, string>>({});
	// Карточка сервера (6.42) — имя сервера, как у нового круга.
	let serverName = $state('');

	// Сколько людей видно сразу (6.43), остальные — за «ещё N».
	const FIRST = 5;
	const people = $derived(preview?.people ?? []);
	const shown = $derived(showAll ? people : people.slice(0, FIRST));
	const chosen = $derived(people.filter((p) => invite[p.account_id]).length);
	const palette = Object.values(CIRCLE_COLORS);

	onMount(async () => {
		void getSession(circle.origin).then((s) => (serverName = s?.name ?? ''));
		try {
			preview = await fetchSplitPreview(circle.origin, circle.circleId);
			const byBlob = new Map<string, string>();
			for (const p of preview.people) if (p.avatar_blob_id) byBlob.set(p.avatar_blob_id, p.account_id);
			if (byBlob.size) {
				await resolveMediaUrls(circle.origin, [...byBlob.keys()], (blobId, url) => {
					const who = byBlob.get(blobId);
					if (who) avatarUrls = { ...avatarUrls, [who]: url };
				});
			}
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});

	function back() {
		if (step === 'who') {
			step = 'what';
			return;
		}
		goto(`/circles/${circle.circleId}/settings`);
	}

	async function split() {
		error = '';
		saving = true;
		try {
			const res = await splitCircle(circle.origin, circle.circleId, {
				name: name.trim(),
				invite: people.filter((p) => invite[p.account_id]).map((p) => p.account_id)
			});
			rememberCircleOrigin(res.circle_id, circle.origin);
			goto(`/circles/${res.circle_id}`);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			saving = false;
		}
	}
</script>

<FormLayout app color={circle.color} title="Отделиться" onback={back}>
	{#if loading}
		<Loading />
	{:else if !preview}
		<Hint class="gutter">{error}</Hint>
	{:else if step === 'what'}
		<Label>Сервер</Label>
		<ServerRow
			class="pad-2-16-10"
			name={serverName || displayHost(circle.origin)}
			subtitle={serverName ? displayHost(circle.origin) : ''}
			card
		/>
		<Label>Название</Label>
		<Input type="text" placeholder="Как назовёте новый круг" bind:value={name} />
		<Label>Что перейдёт в новый круг</Label>
		<Row>
			{#snippet main()}
				<div class="bold">{plural(preview?.posts ?? 0, WORD.post)} с комментариями</div>
				<div class="sub">
					всё, что вы видели в круге «{circle.name}»{preview?.since
						? `, — с ${formatCutoffDate(localDayOf(preview.since))}`
						: ''}
				</div>
			{/snippet}
		</Row>
		<Hint>Прошлое в новом круге увидят только те, кто видел его здесь.</Hint>
		<Button variant="colored" disabled={!name.trim()} onclick={() => (step = 'who')}>
			Дальше: кого позвать
		</Button>
	{:else}
		<Label>Кого позвать · {chosen} из {people.length}</Label>
		{#each shown as p, i (p.account_id)}
			<Row class={i === 0 ? 'pt-2' : undefined}>
				{#snippet leading()}<Avatar
						initial={circleInitial(p.name)}
						color={palette[i % palette.length].cssVar}
						src={avatarUrls[p.account_id]}
					/>{/snippet}
				{#snippet main()}<div class="bold">{p.name}</div>{/snippet}
				{#snippet trailing()}<Switch
						label={`Позвать ${p.name}`}
						bind:checked={() => !!invite[p.account_id], (v) => (invite = { ...invite, [p.account_id]: v })}
					/>{/snippet}
			</Row>
		{/each}
		{#if !showAll && people.length > FIRST}
			<Hint class="mt-8">
				<TextButton onclick={() => (showAll = true)}>ещё {people.length - FIRST}</TextButton>
			</Hint>
		{/if}
		<Hint>
			Каждому придёт письмо и строка в списке кругов. Войдут, если сами захотят. В круге «{circle.name}»
			ничего не изменится, и там никто об этом не узнает.
		</Hint>
		<Button variant="colored" loading={saving} onclick={split}>Отделиться</Button>
		{#if error}
			<Hint class="mt-12">{error}</Hint>
		{/if}
	{/if}
</FormLayout>

<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { goUp } from '$lib/navigation/up';
	import Avatar from '$ui/data/Avatar.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import PeopleStrip from '$ui/forms/PeopleStrip.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { displayHost } from '$lib/auth/origin';
	import { memberAvatarColor } from '$lib/auth/invites';
	import { circleInitial } from '$lib/circles/meta';
	import { seeArrival } from '$lib/circles/move';
	import { WORD, plural } from '$lib/format/plural';
	import { getSession } from '$lib/idb/db';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	// «Круг переехал» / «Круг оживили из архива» (1.10, план 53): вместо
	// вступления 1.4 для того, чья почта приехала с кругом. Имя не спрашивают —
	// оно приехало вместе с записями.
	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let serverName = $state('');
	let serverHost = $state('');
	let loading = $state(false);
	let error = $state('');

	const arrival = $derived(circle.arrival);
	const people = $derived(
		(arrival?.returned ?? []).map((name, i) => ({
			name,
			initial: circleInitial(name),
			color: memberAvatarColor(i)
		}))
	);
	// Тот, кто входит, — крупно, как на «Кто вы в этом круге» (владелец,
	// 2026-10-18: «данные того, кто входит, более заметно и красиво»).
	const kept = $derived(
		arrival?.your_posts
			? `${plural(arrival.your_posts, WORD.post)} — имя, фото и записи на месте`
			: 'имя и фото на месте'
	);

	onMount(async () => {
		if (!arrival) {
			goto(`/circles/${circle.circleId}`, { replaceState: true });
			return;
		}
		const session = await getSession(circle.origin);
		serverName = session?.name ?? '';
		serverHost = displayHost(session?.origin ?? circle.origin);
	});

	async function enter() {
		loading = true;
		error = '';
		try {
			await seeArrival(circle.origin, circle.circleId);
			circle.arrival = undefined;
			await goto(`/circles/${circle.circleId}`, { replaceState: true });
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}
</script>

<FormLayout app color={circle.color} circleTitle={circle.name} onback={() => goUp('/circles')}>
	{#if arrival}
		<!-- Сервер — плашкой и первым (владелец, 2026-10-18). -->
		<Label class="mt-16">Сервер</Label>
		<ServerRow name={serverName || serverHost} subtitle={serverHost} card top={4} />
		{#if arrival.returned_count > 0}
			<Label class="mt-22">Уже вернулись · {arrival.returned_count} из {arrival.total}</Label>
			<PeopleStrip {people} />
		{/if}
		<ScreenTitle class="mt-22 lh-125">
			{arrival.kind === 'moved' ? 'Круг переехал' : 'Круг оживили из архива'}
		</ScreenTitle>
		<Avatar
			class="mt-18"
			initial={circle.identityInitial}
			color={circle.colorHex}
			src={circle.avatarUrl || undefined}
			size="lg"
		/>
		<ScreenTitle centered class="mt-10">{circle.identityName}</ScreenTitle>
		<Hint centered class="mt-4">{kept}</Hint>
		<Button variant="colored" {loading} onclick={() => void enter()}>Войти в круг</Button>
		{#if error}
			<Hint class="mt-12">{error}</Hint>
		{/if}
	{/if}
</FormLayout>

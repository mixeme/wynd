<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import { loadStreetCircles, type StreetCircle } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { deletePin, getPin, putPin } from '$lib/idb/db';
	import { registerRefetch } from '$lib/sync/sync';
	import { initSession, loadSessions } from '$lib/session/session.svelte';

	let circles = $state<StreetCircle[]>([]);
	let loading = $state(true);
	let suppressClick = $state(false);
	let longPressTimer: ReturnType<typeof setTimeout> | undefined;

	async function refresh() {
		circles = await loadStreetCircles();
		loading = false;
	}

	onMount(() => {
		let unsubs: Array<() => void> = [];
		void initSession().then(() => loadSessions()).then((sessions) => {
			if (!sessions.length) {
				goto('/');
				return;
			}
			void refresh();
			unsubs = sessions.map((session) =>
				registerRefetch({ origin: session.origin, refetch: refresh })
			);
		});
		return () => unsubs.forEach((u) => u());
	});

	const pinned = $derived(circles.filter((c) => c.pinned));
	const rest = $derived(circles.filter((c) => !c.pinned));
	const empty = $derived(!loading && circles.length === 0);

	function openCircle(circle: StreetCircle) {
		if (suppressClick) {
			suppressClick = false;
			return;
		}
		rememberCircleOrigin(circle.id, circle.origin);
		goto(`/circles/${circle.id}`);
	}

	function startLongPress(circle: StreetCircle) {
		clearTimeout(longPressTimer);
		longPressTimer = setTimeout(async () => {
			suppressClick = true;
			const pin = await getPin(circle.origin, circle.id);
			if (pin) await deletePin(circle.origin, circle.id);
			else await putPin(circle.origin, circle.id);
			await refresh();
		}, 500);
	}

	function cancelLongPress() {
		clearTimeout(longPressTimer);
	}

	function openNew() {
		goto('/circles/new');
	}

	function openSearch() {
		goto('/search');
	}

	function openSettings() {
		goto('/settings');
	}
</script>

<ShellLayout app onsearch={empty ? undefined : openSearch} onsettings={openSettings}>
	{#snippet fab()}
		<IconButton
			name="plus"
			label="Новый круг"
			style="width:26px;height:26px;stroke-width:1.5"
			onclick={openNew}
		/>
	{/snippet}

	{#if loading}
		<Hint style="margin-top:24px">Загрузка…</Hint>
	{:else if empty}
		<Hint style="margin-top:40px" centered>
			Пока ни одного круга.<br />
			<TextButton onclick={openNew}>Создать первый</TextButton>
		</Hint>
	{:else}
		{#if pinned.length}
			<SectionLabel>Закреплённые</SectionLabel>
			{#each pinned as circle (circle.origin + circle.id)}
				<CircleRow
					initial={circle.initial}
					name={circle.name}
					preview={circle.preview}
					time={circle.time}
					badge={circle.unread || undefined}
					color={circle.color}
					onclick={() => openCircle(circle)}
					onmousedown={() => startLongPress(circle)}
					onmouseup={cancelLongPress}
					onmouseleave={cancelLongPress}
					ontouchstart={() => startLongPress(circle)}
					ontouchend={cancelLongPress}
					ontouchcancel={cancelLongPress}
				/>
			{/each}
			{#if rest.length}
				<div class="sep"></div>
			{/if}
		{/if}
		{#each rest as circle (circle.origin + circle.id)}
			<CircleRow
				initial={circle.initial}
				name={circle.name}
				preview={circle.preview}
				time={circle.time}
				badge={circle.unread || undefined}
				color={circle.color}
				onclick={() => openCircle(circle)}
				onmousedown={() => startLongPress(circle)}
				onmouseup={cancelLongPress}
				onmouseleave={cancelLongPress}
				ontouchstart={() => startLongPress(circle)}
				ontouchend={cancelLongPress}
				ontouchcancel={cancelLongPress}
			/>
		{/each}
	{/if}
</ShellLayout>

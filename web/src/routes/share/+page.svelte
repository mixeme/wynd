<script lang="ts">
	import { onMount } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Loading from '$ui/Loading.svelte';
	import { fetchCircles, loadStreet, type StreetCircle } from '$lib/circles/circles';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import { finishIncomingShare, peekIncomingShare, type IncomingShare } from '$lib/android/shares';

	let share = $state<IncomingShare | null>(null);
	let circles = $state<StreetCircle[]>([]);
	let loading = $state(true);
	let signedIn = $state(false);
	let busy = $state(false);
	let error = $state('');
	beforeNavigate((navigation) => {
		// Это страница, не оверлей: не отменять и не откатывать popstate (C89).
		if (navigation.type === 'popstate' && share && !busy) {
			busy = true;
			void finishIncomingShare(share.token).catch(() => {
				/* Набор останется для повторной отмены. */
			});
		}
	});
	onMount(() => {
		let active = true;
		void (async () => {
			try {
				await initSession();
				const received = await peekIncomingShare();
				if (active) share = received;
				const sessions = await loadSessions();
				const street = sessions.length ? await loadStreet() : null;
				let unavailable = false;
				const writable = new Set<string>();
				await Promise.all(
					[...new Set(street?.circles.map((circle) => circle.origin) ?? [])].map(async (origin) => {
						try {
							for (const circle of await fetchCircles(origin)) {
								if (circle.status === 'active' && !circle.frozen_at && !circle.space_full)
									writable.add(`${origin}:${circle.id}`);
							}
						} catch {
							unavailable = true;
						}
					})
				);
				if (!active) return;
				share = received;
				signedIn = sessions.length > 0;
				circles =
					street?.circles.filter((circle) => writable.has(`${circle.origin}:${circle.id}`)) ?? [];
				if (unavailable)
					error = 'Некоторые серверы не отвечают. Полученное остаётся на этом устройстве';
			} catch {
				if (active) error = 'Не удалось открыть полученное. Попробуйте ещё раз';
			} finally {
				if (active) loading = false;
			}
		})();
		return () => {
			active = false;
		};
	});
	async function cancel() {
		if (busy) return;
		busy = true;
		try {
			if (share) await finishIncomingShare(share.token);
			await goto('/circles', { replaceState: true });
		} catch {
			error = 'Не удалось отменить получение. Попробуйте ещё раз';
			busy = false;
		}
	}
	function choose(circle: StreetCircle) {
		if (!share || busy) return;
		busy = true;
		rememberCircleOrigin(circle.id, circle.origin);
		void goto(
			`/circles/${encodeURIComponent(circle.id)}/compose?incomingShare=${share.token}`
		).catch(() => {
			busy = false;
			error = 'Не удалось открыть запись. Попробуйте ещё раз';
		});
	}
</script>

<FormLayout app title="Новая запись" onback={() => void cancel()}>
	{#if loading}
		<Loading />
	{:else if !share}
		<Hint>Нет полученных материалов для новой записи</Hint>
		<Hint><TextButton onclick={() => void goto('/circles')}>К кругам</TextButton></Hint>
	{:else if !signedIn}
		<Hint>Полученное сохранено на этом устройстве. Войдите, чтобы выбрать круг</Hint>
		<Button onclick={() => void goto('/')}>Войти</Button>
	{:else}
		<Hint
			>Выберите круг для новой записи. {share.files.length
				? `Файлов: ${share.files.length}.`
				: 'Получен текст.'} Перед публикацией запись можно изменить</Hint
		>
		{#each circles as circle (`${circle.origin}:${circle.id}`)}
			<CircleRow
				initial={circle.initial}
				name={circle.name}
				preview={circle.instanceName}
				time=""
				color={circle.color}
				onclick={() => choose(circle)}
			/>
		{/each}
		{#if !circles.length}<Hint>Пока нет кругов для новой записи</Hint>{/if}
	{/if}
	{#if error}<Hint>{error}</Hint>{/if}
	{#if share}<Hint centered
			><TextButton disabled={busy} onclick={() => void cancel()}>Отмена</TextButton></Hint
		>{/if}
</FormLayout>

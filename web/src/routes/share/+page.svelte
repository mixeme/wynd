<script lang="ts">
	import { onMount } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Button from '$ui/forms/Button.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Loading from '$ui/Loading.svelte';
	import {
		fetchCircles,
		loadCirclesCached,
		loadStreet,
		loadStreetCached,
		type CircleListItem,
		type StreetCircle
	} from '$lib/circles/circles';
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
				if (!active) return;
				signedIn = sessions.length > 0;
				const origins = [...new Set(sessions.map((session) => session.origin))];
				// Куда можно добавить — по списку кругов сервера; в снимке он уже есть,
				// поэтому кнопки не врут даже до ответа сети (C100-8).
				const writableKeys = (origin: string, list: CircleListItem[]): string[] =>
					list
						.filter(
							(circle) => circle.status === 'active' && !circle.frozen_at && !circle.space_full
						)
						.map((circle) => `${origin}:${circle.id}`);
				// Снимки улицы и списков — первый кадр: экран не ждёт сети.
				if (sessions.length) {
					const [street, lists] = await Promise.all([
						loadStreetCached(),
						Promise.all(
							origins.map((origin) =>
								loadCirclesCached(origin)
									.then((list) => writableKeys(origin, list))
									.catch(() => [] as string[])
							)
						)
					]);
					if (!active) return;
					const cachedWritable = new Set(lists.flat());
					circles =
						street?.circles.filter((circle) =>
							cachedWritable.has(`${circle.origin}:${circle.id}`)
						) ?? [];
					loading = false;
				}
				let unavailable = false;
				const writable = new Set<string>();
				await Promise.all(
					origins.map(async (origin) => {
						try {
							writableKeys(origin, await fetchCircles(origin)).forEach((key) => writable.add(key));
						} catch {
							unavailable = true;
						}
					})
				);
				const street = sessions.length ? await loadStreet() : null;
				if (!active) return;
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

<FormLayout app title="Новая запись" barMinHeight={48} onback={() => void cancel()}>
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

<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { createCircle } from '$lib/circles/circles';
	import { setCircleColor, setCircleIdentity } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { setIdentityAvatar } from '$lib/circles/settings';
	import { localDayOf } from '$lib/journal/present';
	import IdentityForm from '$ui/forms/IdentityForm.svelte';
	import {
		NEW_CIRCLE_CTX,
		newCircleEditWindowSec,
		type NewCircleContext
	} from '$lib/circles/new-circle';
	import { createPost } from '$lib/journal/posts';
	import type { CroppedImage } from '$lib/media/crop';

	// Создатель круга выбирает имя и фото тем же экраном, что и вступающий
	// (1.3): у него в круге такая же идентичность, и спрашивать её надо так же.
	// «Кто уже здесь» нет — в новом круге никого. Круг заводится здесь, уже с
	// выбранным именем: иначе первой строкой журнала было бы переименование.

	const form = getContext<NewCircleContext>(NEW_CIRCLE_CTX);

	let name = $state('');
	let firstPost = $state('');
	let loading = $state(false);
	let error = $state('');
	let pendingAvatar = $state<CroppedImage | undefined>();
	let gender = $state<'' | 'm' | 'f'>('');

	const session = $derived(form.sessions.find((s) => s.origin === form.selectedOrigin));

	onMount(() => {
		// Прямой заход или перезагрузка: формы нет — назад к её началу.
		if (!form.name.trim()) goto('/circles/new', { replaceState: true });
	});

	async function create() {
		error = '';
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Введите имя';
			return;
		}
		if (!session) {
			goto('/circles/new');
			return;
		}
		const origin = session.origin;
		loading = true;
		try {
			const created = await createCircle(origin, {
				name: form.name.trim(),
				owner_name: trimmed,
				owner_gender: gender,
				edit_window_sec: newCircleEditWindowSec(form),
				color: form.color
			});
			await setCircleColor(origin, created.id, form.color);
			await setCircleIdentity(origin, created.id, trimmed);
			rememberCircleOrigin(created.id, origin);
			// Фото и первая запись — не повод терять уже созданный круг: не
			// вышло — круг открывается всё равно, фото ставится в профиле.
			let avatarFailed = false;
			if (pendingAvatar) avatarFailed = !(await setIdentityAvatar(origin, created.id, pendingAvatar));
			const body = firstPost.trim();
			if (body) {
				try {
					await createPost(origin, created.id, { body, entry_date: localDayOf(new Date().toISOString()), media: [] });
				} catch {
					/* запись можно написать в ленте */
				}
			}
			form.name = '';
			const avatarQuery = avatarFailed ? '?joinAvatar=fail' : '';
			if (form.diaryMode) goto(`/circles/${created.id}${avatarQuery}`, { replaceState: true });
			else goto(`/circles/${created.id}/settings/invite?from=create`, { replaceState: true });
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

</script>

<FormLayout app color={form.color} circleTitle={form.name} onback={() => goUp('/circles/new')}>
	<IdentityForm
		color={form.color}
		bind:name
		bind:firstPost
		bind:avatar={pendingAvatar}
		bind:gender
		postLabel={form.diaryMode ? 'Первая запись' : undefined}
		postPlaceholder={form.diaryMode ? 'С чего начнётся дневник' : undefined}
		onerror={(message) => (error = message)}
	/>
	<Button variant="colored" {loading} onclick={() => void create()}>
		{form.diaryMode ? 'Завести дневник' : 'Создать и позвать'}
	</Button>
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</FormLayout>


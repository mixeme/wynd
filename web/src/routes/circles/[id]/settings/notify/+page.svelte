<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchNotifyPrefs, saveNotifyPrefs } from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let posts = $state(true);
	let comments = $state(true);
	let reactions = $state(false);
	let error = $state('');
	let ready = $state(false);

	async function persist() {
		if (!ready) return;
		try {
			const prefs = await saveNotifyPrefs(circle.origin, circle.circleId, {
				posts,
				comments,
				reactions
			});
			posts = prefs.posts;
			comments = prefs.comments;
			reactions = prefs.reactions;
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	$effect(() => {
		void posts;
		void comments;
		void reactions;
		void ready;
		void persist();
	});

	onMount(async () => {
		try {
			const prefs = await fetchNotifyPrefs(circle.origin, circle.circleId);
			posts = prefs.posts;
			comments = prefs.comments;
			reactions = prefs.reactions;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			ready = true;
		}
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Уведомления круга"
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	<Label>Присылать</Label>
	<div class="row2" style="padding-top:2px">
		<div class="g">Новые записи</div>
		<Switch bind:checked={posts} />
	</div>
	<div class="row2">
		<div class="g">Комментарии</div>
		<Switch bind:checked={comments} />
	</div>
	<div class="row2">
		<div class="g">Реакции</div>
		<Switch bind:checked={reactions} />
	</div>
	<div class="row2">
		<div class="g">
			<div>Упоминания</div>
			<div class="sub">всегда</div>
		</div>
		<Switch checked={true} disabled />
	</div>
	<Hint style="margin-top:14px"
		>Пуш не несёт текста — только круг и тип события. Содержание подтягивается после
		синхронизации.</Hint
	>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>

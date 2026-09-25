<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchNotifyPrefs, saveNotifyPrefs } from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	type NotifyPrefs = { posts: boolean; comments: boolean; reactions: boolean };

	let posts = $state(true);
	let comments = $state(true);
	let reactions = $state(false);
	let error = $state('');
	let ready = $state(false);
	let baseline = $state<NotifyPrefs | null>(null);

	async function persist() {
		if (!ready || !baseline) return;
		if (
			posts === baseline.posts &&
			comments === baseline.comments &&
			reactions === baseline.reactions
		) {
			return;
		}
		try {
			const prefs = await saveNotifyPrefs(circle.origin, circle.circleId, {
				posts,
				comments,
				reactions
			});
			posts = prefs.posts;
			comments = prefs.comments;
			reactions = prefs.reactions;
			baseline = { posts, comments, reactions };
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	$effect(() => {
		void posts;
		void comments;
		void reactions;
		void persist();
	});

	onMount(async () => {
		try {
			const prefs = await fetchNotifyPrefs(circle.origin, circle.circleId);
			posts = prefs.posts;
			comments = prefs.comments;
			reactions = prefs.reactions;
			baseline = { posts, comments, reactions };
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
	<SettingsRow title="Новые записи" style="padding-top:2px">
		{#snippet control()}
			<Switch bind:checked={posts} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Комментарии">
		{#snippet control()}
			<Switch bind:checked={comments} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Реакции">
		{#snippet control()}
			<Switch bind:checked={reactions} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Упоминания" subtitle="всегда">
		{#snippet control()}
			<Switch checked={true} disabled />
		{/snippet}
	</SettingsRow>
	<Hint style="margin-top:14px"
		>Пуш не несёт текста — только круг и тип события. Содержание подтягивается после
		синхронизации.</Hint
	>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>

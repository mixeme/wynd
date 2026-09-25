<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { deleteCircle, fetchCircleSettings } from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let expectedName = $state('');
	let confirm = $state('');
	let error = $state('');
	let loading = $state(false);

	onMount(async () => {
		try {
			const s = await fetchCircleSettings(circle.origin, circle.circleId);
			expectedName = s.name;
		} catch (err) {
			error = authErrorHint(err);
		}
	});

	async function onDelete() {
		if (confirm !== expectedName) {
			error = 'Введите название круга точно';
			return;
		}
		loading = true;
		error = '';
		try {
			await deleteCircle(circle.origin, circle.circleId, confirm);
			goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
			loading = false;
		}
	}
</script>

<FormLayout
	app
	color={circle.color}
	title="Удаление круга"
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	<Hint style="margin:16px"
		>Чтобы удалить круг и все записи, введите его название: «{expectedName}».</Hint
	>
	<Label>Название круга</Label>
	<Input active bind:value={confirm} />
	<Button variant="colored" style="margin-top:16px" {loading} onclick={() => void onDelete()}>
		Удалить круг и все записи
	</Button>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{/if}
</FormLayout>

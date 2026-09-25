<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { NEW_CIRCLE_CTX, type NewCircleContext } from '$lib/circles/new-circle';

	const form = getContext<NewCircleContext>(NEW_CIRCLE_CTX);

	function pickOrigin(origin: string) {
		form.selectedOrigin = origin;
		goto(`/circles/new?origin=${encodeURIComponent(origin)}`);
	}
</script>

<FormLayout
	color={form.color}
	app
	title="Сервер"
	onback={() => goto('/circles/new')}
>
	{#each form.sessions as session (session.origin)}
		<ServerRow
			name={session.name}
			subtitle={form.serverSubtitle(session)}
			variant={session.origin === form.selectedOrigin ? 'ok' : 'info'}
			style={session.origin === form.sessions[0]?.origin ? 'margin-top:8px' : undefined}
			onclick={() => pickOrigin(session.origin)}
		/>
	{/each}
	<Hint style="margin-top:18px">
		Здесь серверы, на которых вы уже есть. Добавить другой можно в настройках, до этой формы.
	</Hint>
</FormLayout>

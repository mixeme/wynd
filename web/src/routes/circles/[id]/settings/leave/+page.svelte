<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchCircleSettings, leaveCircle } from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let leaveMode = $state<'read' | 'gone'>('read');
	let isOwner = $state(false);
	let loading = $state(false);
	let pageLoading = $state(true);
	let error = $state('');

	const readHint = $derived(
		leaveMode === 'read'
			? 'Круг останется в списке. Писать нельзя. Видно то, что застали — от входа до сегодня.'
			: 'Круг исчезнет из списка. Записи останутся под вашим именем.'
	);

	async function onLeave() {
		loading = true;
		error = '';
		try {
			await leaveCircle(circle.origin, circle.circleId, leaveMode === 'read');
			goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
			loading = false;
		}
	}

	onMount(async () => {
		try {
			const settings = await fetchCircleSettings(circle.origin, circle.circleId);
			isOwner = settings.is_owner ?? false;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			pageLoading = false;
		}
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Покинуть круг"
	onback={() => goUp(`/circles/${circle.circleId}/settings`)}
>
	{#if pageLoading}
		<Loading />
	{:else if isOwner}
		<Hint class="gutter">Сначала передайте владение — пока вы владелец, уйти нельзя.</Hint>
	{:else}
		<Hint class="gutter"
			>Записи останутся в «{circle.name}» под именем «{circle.identityName}». В хронике будет
			«покинул круг» — так же, как если бы вас исключили.</Hint
		>
		<Label class="mt-22">Как уйти</Label>
		<ChipGroup>
			<Chip selected={leaveMode === 'read'} onclick={() => (leaveMode = 'read')}>
				Читать, не писать
			</Chip>
			<Chip selected={leaveMode === 'gone'} onclick={() => (leaveMode = 'gone')}>Совсем</Chip>
		</ChipGroup>
		<Hint>{readHint}</Hint>
		<Button class="mt-16" variant="colored" {loading} onclick={() => void onLeave()}>
			Покинуть круг
		</Button>
		{#if error}
			<Hint class="gutter">{error}</Hint>
		{/if}
	{/if}
</FormLayout>

{#if !pageLoading && isOwner}
	<ConfirmDialog
		title="Сначала передайте владение"
		confirmLabel="Передать"
		onconfirm={() => goto(`/circles/${circle.circleId}/settings/members?transfer=1`)}
		oncancel={() => goto(`/circles/${circle.circleId}/settings`)}
	>
		<Hint>Подвешенных кругов не бывает. Пока вы владелец «{circle.name}», уйти нельзя.</Hint>
	</ConfirmDialog>
{/if}

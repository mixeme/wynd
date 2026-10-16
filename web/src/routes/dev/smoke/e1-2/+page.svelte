<!-- Smoke-test: PlainLayout from docs/screens.html #e1-2 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import InviteCard from '$ui/forms/InviteCard.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
</script>

<PlainLayout>
	<ScreenTitle class="mt-24">Вас пригласили</ScreenTitle>
	<InviteCard initial="С" name="Семья" preview="22 участника · пригласила Аня" />
	<Label>Сервер</Label>
	<ServerRow name="Дом Ани" subtitle="home.example.org" variant="ok" card gdpr top={0} />
	<Label>Почта</Label>
	<FieldDisplay value="you@example.com" active />
	<Hint>Пришлём код для входа. Пароля нет.</Hint>
	<Button onclick={() => {}}>Получить код</Button>
	<Hint centered class="mt-26">
		Вход создаётся на этом сервере.<br />Круги с других серверов добавляются позже.
	</Hint>
</PlainLayout>

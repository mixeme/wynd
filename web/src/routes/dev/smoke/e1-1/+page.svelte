<!-- Smoke-test: PlainLayout from docs/screens.html #e1-1 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import InviteCard from '$ui/forms/InviteCard.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
</script>

<PlainLayout>
	<ScreenTitle style="margin-top:24px">Вас пригласили</ScreenTitle>
	<InviteCard initial="С" name="Семья" preview="22 участника · пригласила Аня" />
	<Label>Сервер</Label>
	<FieldDisplay>
		<div style="font-weight:600">Дом Ани</div>
		<div style="font-size:12.5px;color:var(--muted)">home.example.org</div>
	</FieldDisplay>
	<Label>Почта</Label>
	<FieldDisplay value="you@example.com" active gray />
	<Hint>
		Пришлём код для входа. Пароля нет: почта понадобится, только чтобы вернуться на
		другом устройстве.
	</Hint>
	<Button onclick={() => {}}>Получить код</Button>
	<Hint centered style="margin-top:26px">
		Учётная запись создаётся на этом сервере.<br />Круги с других серверов добавляются позже.
	</Hint>
</PlainLayout>

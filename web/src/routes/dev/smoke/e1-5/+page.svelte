<!-- Smoke-test: static markup from docs/screens.html #e1-5 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Logo from '$ui/Logo.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
</script>

<div class="ph shell">
	<div class="sb">
		<span>9:41</span><span class="sig"><u></u><u></u><u></u><u></u></span>
	</div>
	<div class="logo-wrap">
		<Logo />
	</div>
	<div class="h1s ctr" style="margin-top:30px">Вернуться</div>
	<div class="hint ctr">
		Wynd не помнит устройств — только почту, которой вы называетесь на этом сервере.
	</div>
	<Label style="margin-top:26px">Сервер</Label>
	<FieldDisplay>
		<div style="font-weight:600">Дом Ани</div>
		<div style="font-size:12.5px;color:var(--muted)">home.example.org</div>
	</FieldDisplay>
	<Label>Почта</Label>
	<FieldDisplay value="anya@example.com" active />
	<Button onclick={() => {}}>Получить код</Button>
	<div class="hint ctr" style="margin-top:30px">
		Впервые? <TextButton onclick={() => {}}>Прийти без приглашения</TextButton><br />или откройте присланную
		ссылку.
	</div>
</div>

<style>
	.logo-wrap {
		margin: 38px 0 0;
		display: flex;
		justify-content: center;
		color: var(--ink);
	}
</style>

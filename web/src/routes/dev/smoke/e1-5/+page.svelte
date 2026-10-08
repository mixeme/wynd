<!-- Smoke-test: static markup from docs/screens.html #e1-5 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
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
	<div class="h1s ctr" style="margin-top:30px">Войти</div>
	<ServerRow name="Дом Ани" subtitle="home.example.org" variant="ok" card />
	<Label style="margin-top:16px">Почта</Label>
	<FieldDisplay value="anya@example.com" active />
	<Hint>Пришлём код для входа. Пароля нет.</Hint>
	<Button onclick={() => {}}>Получить код</Button>
	<div class="hint ctr" style="margin-top:30px">
		<TextButton onclick={() => {}}>Регистрация без приглашения</TextButton><br />
		<TextButton onclick={() => {}}>У меня есть приглашение</TextButton>
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

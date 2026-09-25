<!-- Smoke-test: forms components from docs/screens.html #e1-2 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import CodeBox from '$ui/forms/CodeBox.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame shell>
	<StatusBar />
	<div style="padding:6px 16px 0"><Icon name="back" /></div>
	<ScreenTitle>Код из письма</ScreenTitle>
	<div style="margin:0 16px">Отправлен на <strong>anya@example.com</strong></div>
	<div style="margin:7px 16px 0"><span class="link under">изменить адрес</span></div>
	<CodeBox digits={['4', '1', '8']} active={3} />
	<Hint>Код действует 15 минут. Три попытки.</Hint>
	<div class="rowin">
		<Button variant="off" style="flex:1" onclick={() => {}}>Отправить ещё раз</Button>
		<span style="color:var(--faint);font-size:12.5px">через 0:47</span>
	</div>
	<div
		style="position:absolute;left:0;right:0;bottom:0;height:250px;background:#E7E1D6;border-top:1px solid var(--line);display:grid;place-items:center;font-size:11.5px;color:#A29A90"
	>
		цифровая клавиатура
	</div>
</PhoneFrame>

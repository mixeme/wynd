<!-- Smoke-test: overlay components from docs/screens.html #e7-4 -->
<script lang="ts">
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import PushBanner from '$ui/overlays/PushBanner.svelte';
</script>

<PhoneFrame>
	<div class="pic p6" style="position:absolute;inset:0;border:0;border-radius:0"></div>
	<div
		style="position:absolute;inset:0;background:linear-gradient(180deg,rgba(23,21,19,.55),rgba(23,21,19,.3) 40%,rgba(23,21,19,.6))"
	></div>
	<div class="sb" style="color:#fff;position:relative">
		<span>9:41</span>
		<span class="sig"><u style="background:#fff"></u><u style="background:#fff"></u><u style="background:#fff"></u><u style="background:#fff"></u></span>
	</div>
	<div style="position:relative;text-align:center;color:#fff;margin-top:56px">
		<div style="font-size:56px;font-weight:600;line-height:1">9:41</div>
		<div style="font-size:13.5px;opacity:.85;margin-top:4px">среда, 19 августа</div>
	</div>
	<PushBanner
		initial="С"
		color="var(--terracotta)"
		title="Семья"
		subtitle="3 новые записи"
		time="сейчас"
		style="top:250px"
	/>
	<PushBanner
		initial="Р"
		color="var(--coffee)"
		title="Ремонт"
		subtitle="Славик упомянул вас"
		time="12 мин"
		style="top:326px"
	/>
	<PushBanner
		initial="У"
		color="var(--indigo)"
		title="Универ"
		subtitle="1 новая запись"
		time="вчера"
		faded
		style="top:402px"
	/>
</PhoneFrame>

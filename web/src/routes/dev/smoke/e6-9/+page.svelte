<!-- Smoke-test: forms components from docs/screens.html #e6-9 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import DangerNote from '$ui/forms/DangerNote.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Сроки архивации" tabs={false} />
	<Label>Отсечка</Label>
	<div class="row2" style="border-top:1px solid var(--line);border-bottom:1px solid var(--line)">
		<div class="g">
			<div style="font-weight:600">1 июня 2025</div>
			<div class="sub">126 записей, 6,2 ГБ</div>
		</div>
		<span class="val">изменить</span>
	</div>
	<Hint>
		Отсечка двигается, пока никто не скачал архив. После первого скачивания она замирает:
		другой диапазон — это новый цикл, а не правка этого.
	</Hint>
	<Label style="margin-top:18px">Скачать до</Label>
	<FieldDisplay value="20 сентября 2026" active />
	<Hint>
		Срок двигается в любую сторону и в любой момент: архив снят на отсечку, а не на срок, и
		от сдвига не портится.
	</Hint>
	<Label style="margin-top:18px">Напомнить письмом</Label>
	<ChipGroup>
		<Chip>За сутки</Chip>
		<Chip selected>За 3 дня</Chip>
		<Chip>За неделю</Chip>
	</ChipGroup>
	<Hint>
		Писем два: на старте и напоминание. Сдвинете срок — напоминание пересчитается от новой
		даты, интервал останется прежним.
	</Hint>
	<DangerNote style="margin-top:20px">
		20 сентября всё до 1 июня 2025 удалится с сервера — независимо от того, все ли успели
		скачать. Предупредить людей — на вас.
	</DangerNote>
	<Button variant="colored" style="margin-top:14px" onclick={() => {}}>Запустить архивацию</Button>
</PhoneFrame>

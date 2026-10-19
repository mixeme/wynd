<!-- Smoke-test: forms components from docs/screens.html #e6-21 -->
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
	<FieldDisplay value="1 июня 2025" active />
	<Hint>
		Пока никто не скачал архив, отсечку можно сдвинуть.
	</Hint>
	<Label style="margin-top:18px">Скачать до</Label>
	<FieldDisplay value="20 сентября 2026" active />
	<Hint>
		Срок можно сдвинуть в любой момент.
	</Hint>
	<Label style="margin-top:18px">Напомнить письмом</Label>
	<ChipGroup>
		<Chip>За сутки</Chip>
		<Chip selected>За 3 дня</Chip>
		<Chip>За неделю</Chip>
	</ChipGroup>
	<Hint>
		Письмо придёт всем сразу и ещё раз перед сроком.
	</Hint>
	<DangerNote style="margin-top:20px">
		20 сентября всё до 1 июня 2025 удалится с сервера — независимо от того, все ли успели
		скачать. Предупредить людей — на вас.
	</DangerNote>
	<Button variant="colored" style="margin-top:14px" onclick={() => {}}>Запустить архивацию</Button>
</PhoneFrame>

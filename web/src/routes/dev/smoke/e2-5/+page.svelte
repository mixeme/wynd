<!-- Smoke-test: data components from docs/screens.html #e2-5 -->
<script lang="ts">
	import BackBar from '$ui/chrome/BackBar.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame shell>
	<StatusBar />
	<BackBar compact search="яблони" />
	<ChipGroup style="margin-top:12px">
		<Chip>Этот круг</Chip>
		<Chip selected>Все круги</Chip>
	</ChipGroup>
	<ChipGroup style="margin-top:8px">
		<Chip>Период</Chip>
		<Chip>С фото</Chip>
		<Chip>С местом</Chip>
	</ChipGroup>
	<SearchGroupHeader color="var(--terracotta)" name="Семья" count={4} />
	<SearchResultRow author="Аня" time="сегодня, 14:02" thumb thumbVariant="p1">
		{#snippet preview()}
			Были на даче, все живы. <span class="hl">Яблони</span> в этом году…
		{/snippet}
	</SearchResultRow>
	<SearchResultRow author="Мама" time="12 мая">
		{#snippet preview()}
			Обрезали <span class="hl">яблони</span>, было страшно
		{/snippet}
	</SearchResultRow>
	<SearchGroupHeader color="var(--ochre)" name="Дача" count={2} />
	<SearchResultRow author="Мама" time="5 авг" thumb thumbVariant="p2">
		{#snippet preview()}
			<span class="hl">Яблони</span> зацвели
		{/snippet}
	</SearchResultRow>
	<SearchResultRow author="я" time="3 апр">
		{#snippet preview()}
			Купил две <span class="hl">яблони</span>, посадил у забора
		{/snippet}
	</SearchResultRow>
	<Hint centered style="margin-top:22px">
		Ищется только то, что вы застали:<br />отрезки видимости работают и в поиске.
	</Hint>
</PhoneFrame>

<!-- Smoke-test: AdminWideLayout from docs/screens.html #e9-5 -->
<script lang="ts">
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import InlineInput from '$ui/admin/InlineInput.svelte';
	import QuotaRequestRow from '$ui/admin/QuotaRequestRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import StackBar from '$ui/admin/StackBar.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
</script>

<AdminWideLayout active="Хранилище" server="Дом Ани · home.example.org" height="650px">
	<AdminSection title="Хранилище">
		<StackBar
			threshold="90%"
			segments={[
				{ width: '21%', color: 'var(--terracotta)' },
				{ width: '12.8%', color: 'var(--olive)' },
				{ width: '6.5%', color: 'var(--indigo)' },
				{ width: '4.8%', color: 'var(--ochre)' },
				{ width: '1%', color: 'var(--slate)' }
			]}
		/>
		<div style="font-size:12.5px;color:var(--muted);margin:10px 0 22px">
			18,4 ГБ из 40 ГБ · свободно 21,6 ГБ
		</div>
		<DataTable>
			<thead>
				<tr>
					<th style="width:38%">Круг</th>
					<th>Записей</th>
					<th>Медиа</th>
					<th>Квота</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td class="n"
						><span class="dot" style="background:var(--terracotta)"></span>Семья</td
					>
					<td>340</td>
					<td>8,4 ГБ</td>
					<td
						><span class="qbar"><u style="width:84%;background:var(--terracotta)"></u></span>10
						ГБ</td
					>
				</tr>
				<tr>
					<td class="n"
						><span class="dot" style="background:var(--olive)"></span>Поход на Алтай</td
					>
					<td>128</td>
					<td>5,1 ГБ</td>
					<td style="color:var(--faint)">без квоты</td>
				</tr>
				<tr>
					<td class="n"
						><span class="dot" style="background:var(--indigo)"></span>Универ</td
					>
					<td>96</td>
					<td>2,6 ГБ</td>
					<td style="color:var(--faint)">без квоты</td>
				</tr>
				<tr>
					<td class="n"><span class="dot" style="background:var(--ochre)"></span>Дача</td>
					<td>210</td>
					<td>1,9 ГБ</td>
					<td
						><span class="qbar"><u style="width:38%;background:var(--ochre)"></u></span>5
						ГБ</td
					>
				</tr>
				<tr>
					<td class="n"
						><span class="dot" style="background:var(--slate)"></span>Соседи</td
					>
					<td>34</td>
					<td>0,4 ГБ</td>
					<td style="color:var(--faint)">без квоты</td>
				</tr>
			</tbody>
		</DataTable>
		<div style="display:flex;align-items:center;gap:14px;margin-top:26px;flex-wrap:wrap">
			<span style="font-size:13.5px;font-weight:600;width:168px;flex:0 0 auto">Потолок инстанса</span>
			<InlineInput><span style="color:var(--faint)">40 ГБ</span></InlineInput>
			<span style="font-size:12.5px;color:var(--muted)">или</span>
			<InlineInput>80% диска</InlineInput>
		</div>
		<div style="font-size:11.5px;color:var(--faint);margin-top:8px;line-height:1.5">
			Действует одно: гигабайты или доля диска. Сохранение одного очищает другое. При упоре текст
			пишется, медиа — нет.
		</div>
		<SectionLabel style="margin:26px 0 8px">Просят больше</SectionLabel>
		<QuotaRequestRow
			circleColor="var(--terracotta)"
			circleName="Семья"
			requested="20 ГБ"
			previous="10"
			requester="владелец"
			date="9 августа"
			freeSpace="21,6 ГБ"
			onapprove={() => {}}
			onreject={() => {}}
		/>
		<div style="font-size:11.5px;color:var(--faint);margin-top:14px;line-height:1.6">
			Отказ ничего не ломает: владелец круга сам выберет отсечку и срок, скажет об этом людям и
			освободит место. Ни отсечку, ни срок панель не двигает — этих кнопок здесь нет.
		</div>
	</AdminSection>
</AdminWideLayout>

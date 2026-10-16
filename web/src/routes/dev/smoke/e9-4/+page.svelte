<!-- Smoke-test: AdminWideLayout from docs/screens.html #e9-4 -->
<script lang="ts">
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import SwitchRow from '$ui/admin/SwitchRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';

	let allow = $state(false);
</script>

<AdminWideLayout active="Люди" server="Дом Ани · home.example.org" height="520px">
	<AdminSection title="Люди">
		<DataTable>
			<thead>
				<tr><th>Почта</th><th>Круги</th><th>На сервере</th><th></th></tr>
			</thead>
			<tbody>
				<tr><td class="n">anya@example.com</td><td>5</td><td class="muted">с 12 марта 2024</td><td></td></tr>
				<tr><td class="n">kot@example.com</td><td>3</td><td class="muted">с 12 марта 2024</td><td></td></tr>
				<tr><td class="n">m.petrov@example.com</td><td>1</td><td class="muted">с 5 августа</td><td></td></tr>
			</tbody>
		</DataTable>
		<SectionLabel class="mt-30 mx-0 mb-10">Уход с сервера</SectionLabel>
		<SwitchRow end bind:checked={allow} title="Разрешить стирать имена и фото">
			Кто уходит с сервера насовсем, сможет стереть свои имена, фото и пол во всех кругах.
			Записи останутся без подписи. Это право на удаление по GDPR: включите, если сервер должен
			его соблюдать.
		</SwitchRow>
		<div class="fine mt-8">Тех, кто ушёл раньше, это не касается.</div>
	</AdminSection>
</AdminWideLayout>

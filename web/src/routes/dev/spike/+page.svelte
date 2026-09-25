<script lang="ts">
	import { goto } from '$app/navigation';
</script>

<main class="wrap">
	<h1>UI spike — e3-1</h1>
	<p>
		Этап 0 завершён: победила <strong>ветка Б</strong> (Bits UI + свой CSS). Экран e3-1 —
		<a href="/dev/spike/b">/dev/spike/b</a>. Ветка А (shadcn-svelte) удалена после сравнения.
	</p>

	<table>
		<thead>
			<tr>
				<th>Замер</th>
				<th>А (shadcn)</th>
				<th>Б (Bits UI + CSS)</th>
			</tr>
		</thead>
		<tbody>
			<tr>
				<td>Компонентов: готовых / перекрашенных / своих</td>
				<td>2 / 5 / 5</td>
				<td>1 / 0 / 8</td>
			</tr>
			<tr>
				<td>12.5 / 11.5 / 13.5 px и gap 10 px без борьбы</td>
				<td>нет</td>
				<td>да</td>
			</tr>
			<tr>
				<td>Строк разметки на экран</td>
				<td>~310</td>
				<td>~115</td>
			</tr>
			<tr>
				<td>Прирост бандла JS+CSS gzip</td>
				<td>~52 KB</td>
				<td>~24 KB</td>
			</tr>
			<tr>
				<td>Своего кода на CommentBar</td>
				<td>~45 строк</td>
				<td>0 (разметка .comp)</td>
			</tr>
			<tr>
				<td>Читается ли разметка</td>
				<td>class soup</td>
				<td>.post .cbar .comp</td>
			</tr>
		</tbody>
	</table>

	<button type="button" onclick={() => goto('/dev/spike/b')}>Открыть e3-1 (ветка Б)</button>
</main>

<style>
	.wrap {
		max-width: 720px;
		font-size: 14px;
		line-height: 1.5;
	}

	h1 {
		font-size: 17px;
		margin: 0 0 12px;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		margin: 20px 0;
		font-size: 13px;
	}

	th,
	td {
		text-align: left;
		padding: 8px 10px 8px 0;
		border-bottom: 1px solid var(--line);
		vertical-align: top;
	}

	th {
		font-size: 11px;
		letter-spacing: 0.09em;
		text-transform: uppercase;
		color: var(--faint);
	}

	button {
		background: var(--ink);
		color: var(--paper);
		border: none;
		border-radius: 12px;
		padding: 12px 18px;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
</style>

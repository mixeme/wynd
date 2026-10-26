<script lang="ts">
	// Что Мастерская выполнила на сервере и что он ответил (план 54):
	// «показать, что выполняется», «подробности».
	let {
		text,
		class: className = '',
		style = ''
	}: {
		text: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<pre class="ins-raw {className}" {style}>{text}</pre>

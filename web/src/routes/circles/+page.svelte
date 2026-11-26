<script lang="ts">
	import StreetLayout from '$lib/layouts/StreetLayout.svelte';
	import { splitLayout } from '$lib/session/layout.svelte';
</script>

<!-- План 75: на широком окне список стоит слева, в каркасе /circles/+layout;
     правая часть без выбора пустая, её рисует каркас. На телефоне — как было. -->
{#if !splitLayout()}
	<StreetLayout />
{/if}

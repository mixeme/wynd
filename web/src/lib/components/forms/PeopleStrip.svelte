<script lang="ts">
	export type PeopleStripItem = {
		name: string;
		initial: string;
		color?: string;
	};

	let {
		people
	}: {
		people: PeopleStripItem[];
	} = $props();
</script>

<div class="people">
	{#each people as person (person.name)}
		<div class="pp">
			<div class="av2" style:background={person.color}>{person.initial}</div>
			{person.name}
		</div>
	{/each}
</div>

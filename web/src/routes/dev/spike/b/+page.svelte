<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import { Tabs } from 'bits-ui';

	const tabs = ['Хронология', 'Сетка', 'Карта'];
	let active = $state('Хронология');
</script>

<div class="ph">
	<div class="sb">
		<span>9:41</span>
		<span class="sig"><u></u><u></u><u></u><u></u></span>
	</div>

	<div class="cbar">
		<div class="top">
			<Icon name="back" />
			<span class="t">Семья</span>
			<span class="idn">Мышь<span class="av">М</span></span>
		</div>
		<Tabs.Root bind:value={active}>
			<Tabs.List class="tabs">
				{#each tabs as tab (tab)}
					<Tabs.Trigger value={tab}>
						{#snippet child({ props })}
							<span {...props} class:on={active === tab}>{tab}</span>
						{/snippet}
					</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
	</div>

	<div class="ev"><span>Мышка теперь Мышь</span></div>

	<div class="post">
		<div class="pa">
			<div class="av" style="background:#58673A">А</div>
			<div>
				<div class="n">Аня</div>
				<div class="tm">сегодня, 14:02</div>
			</div>
		</div>
		<div class="pt">Были на даче, все живы. Яблони в этом году как с ума сошли.</div>
		<div class="pic sq p1">
			<span class="tagr"><Icon name="loc" size="xs" />Дача</span>
			<span class="cnt">12</span>
		</div>
		<div class="rx">
			<span class="one">
				<Icon name="heart" size="xs" style="color:var(--c)" />
				Кот, Петя
			</span>
			<span class="add">+</span>
		</div>
		<div class="cm">
			<div>Петя: а компот будет?</div>
			<div class="mo">ещё 11 комментариев</div>
		</div>
	</div>

	<div class="post">
		<div class="pa">
			<div class="av" style="background:#3C4D83">М</div>
			<div>
				<div class="n">Мама</div>
				<div class="tm">вчера, 21:40</div>
			</div>
		</div>
		<div class="pt">Дед нашёл на чердаке коробку с плёнками. Будем проявлять.</div>
	</div>

	<div class="comp">
		<div class="f">
			Написать в журнал…
			<Icon name="photo" size="sm" style="margin-left:auto" />
		</div>
		<div class="send"><Icon name="send" style="color:#fff;width:19px;height:19px" /></div>
	</div>
</div>

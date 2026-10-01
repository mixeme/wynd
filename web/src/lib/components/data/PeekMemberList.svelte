<script lang="ts">
	import MemberRow from '$ui/data/MemberRow.svelte';
	import { circleInitial } from '$lib/circles/meta';
	import {
		memberAvatarColor,
		memberSubtitle,
		type InvitePeekMember
	} from '$lib/auth/invites';

	// «Кто уже здесь» до входа (1.3; план 47, 2.13): участники круга по
	// приглашению или заявке. Был скопирован в «Вступление» и «Вас
	// пригласили»; цвета аватаров — по порядку, фото до входа не видно.
	let { members }: { members: InvitePeekMember[] } = $props();
</script>

{#each members as member, i (i)}
	<MemberRow
		initial={circleInitial(member.name)}
		name={member.name}
		subtitle={memberSubtitle(member)}
		color={memberAvatarColor(i)}
	/>
{/each}

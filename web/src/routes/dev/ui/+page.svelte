<!-- Dev-каталог UI-библиотеки Wynd — этап 8 -->
<script lang="ts">
	import AdminNav from '$ui/admin/AdminNav.svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import CodeBlock from '$ui/admin/CodeBlock.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import InlineInput from '$ui/admin/InlineInput.svelte';
	import AdminField from '$ui/admin/AdminField.svelte';
	import Panel from '$ui/admin/Panel.svelte';
	import QuotaRequestRow from '$ui/admin/QuotaRequestRow.svelte';
	import StackBar from '$ui/admin/StackBar.svelte';
	import StatusIcon from '$ui/admin/StatusIcon.svelte';
	import AdminBar from '$ui/chrome/AdminBar.svelte';
	import AppBar from '$ui/chrome/AppBar.svelte';
	import BackBar from '$ui/chrome/BackBar.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import ArchiveBanner from '$ui/data/ArchiveBanner.svelte';
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import CircleRow from '$ui/data/CircleRow.svelte';
	import DayCard from '$ui/data/DayCard.svelte';
	import DayGrid from '$ui/data/DayGrid.svelte';
	import DayHeader from '$ui/data/DayHeader.svelte';
	import EntryDateMark from '$ui/data/EntryDateMark.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import FeedDayPromptCard from '$ui/data/FeedDayPromptCard.svelte';
	import FoldHeader from '$ui/data/FoldHeader.svelte';
	import GroupFoldCard from '$ui/data/GroupFoldCard.svelte';
	import MapBadge from '$ui/data/MapBadge.svelte';
	import MapPostSheet from '$ui/data/MapPostSheet.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import MentionText from '$ui/data/MentionText.svelte';
	import EditWindowPicker from '$ui/forms/EditWindowPicker.svelte';
	import IdentityForm from '$ui/forms/IdentityForm.svelte';
	import type { EditWindowKey } from '$lib/circles/settings';
	import MediaTile from '$ui/data/MediaTile.svelte';
	import MonthLabel from '$ui/data/MonthLabel.svelte';
	import PhotoGrid from '$ui/data/PhotoGrid.svelte';
	import PhotoPlaceholder from '$ui/data/PhotoPlaceholder.svelte';
	import PayStreetBanner from '$ui/data/PayStreetBanner.svelte';
	import PostCard from '$ui/data/PostCard.svelte';
	import CommentPreview from '$ui/data/CommentPreview.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';
	import ReactionBar from '$ui/data/ReactionBar.svelte';
	import ReactionListRow from '$ui/data/ReactionListRow.svelte';
	import SearchGroupHeader from '$ui/data/SearchGroupHeader.svelte';
	import SearchResultRow from '$ui/data/SearchResultRow.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import AddPhotoButton from '$ui/forms/AddPhotoButton.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import CodeBox from '$ui/forms/CodeBox.svelte';
	import ColorSwatches from '$ui/forms/ColorSwatches.svelte';
	import DangerNote from '$ui/forms/DangerNote.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import InviteCard from '$ui/forms/InviteCard.svelte';
	import RequisitesCard from '$ui/forms/RequisitesCard.svelte';
	import MentionPicker from '$ui/forms/MentionPicker.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import PeopleStrip from '$ui/forms/PeopleStrip.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import SearchField from '$ui/forms/SearchField.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import VolumeChart from '$ui/forms/VolumeChart.svelte';
	import Icon, { type IconName } from '$ui/Icon.svelte';

	let cropFile = $state<File | undefined>();

	async function openCropDemo() {
		const res = await fetch('/icon-192.png');
		const blob = await res.blob();
		cropFile = new File([blob], 'demo.png', { type: blob.type });
	}

	const iconNames: IconName[] = [
		'search',
		'gear',
		'back',
		'heart',
		'plus',
		'cloud',
		'loc',
		'send'
	];
	import Logo from '$ui/Logo.svelte';
	import Mark from '$ui/Mark.svelte';
	import Loading from '$ui/Loading.svelte';
	import AvatarCrop from '$ui/overlays/AvatarCrop.svelte';
	import CommentBar from '$ui/overlays/CommentBar.svelte';
	import Fab from '$ui/overlays/Fab.svelte';
	import Lightbox from '$ui/overlays/Lightbox.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import PushBanner from '$ui/overlays/PushBanner.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import type { CircleColor } from '$lib/theme/colors';
	import {
		ACCEPTANCE,
		COMPONENT_MAP,
		SMOKE_ROUTES,
		countComponents
	} from './catalog';
	import type { VolumeBucket } from '$lib/circles/settings';
	import { formatEntryDate } from '$lib/format/time';
	import '$lib/styles/map.css';

	let swOn = $state(false);
	let demoEditWindow = $state<EditWindowKey>('custom');
	let demoCustomHours = $state(36);
	let color = $state<CircleColor>('ochre');
	const total = countComponents();

	const demoVolume: VolumeBucket[] = Array.from({ length: 14 }, (_, i) => {
		const month = 11 + i;
		const y = 2024 + Math.floor((month - 1) / 12);
		const m = ((month - 1) % 12) + 1;
		return {
			period: `${y}-${String(m).padStart(2, '0')}`,
			bytes: [34, 52, 68, 80, 58, 44, 62, 76, 28, 22, 34, 18, 26, 30][i] ?? 30,
			cumulative_bytes: 0
		};
	});
	let demoCode = $state('');
	let demoCutoffIndex = $state(7);
	let demoCutoffX = $state(205);
	const demoCutoffLabel = $derived(
		demoVolume[demoCutoffIndex]
			? formatEntryDate(`${demoVolume[demoCutoffIndex].period}-01`)
			: ''
	);

	function onDemoCutoff(index: number) {
		demoCutoffIndex = index;
	}
</script>

<main class="catalog">
	<header class="head">
		<h1>Wynd UI — dev-каталог</h1>
		<p>
			Импорт: <code>import Button from '$ui/forms/Button.svelte'</code>.
			{total} компонентов и 6 layout-шаблонов. Данные-примеры из
			<code>docs/visual/screens.html</code>; полные экраны — в
			<a href="#smoke">smoke-тестах</a>.
		</p>
		<nav class="toc">
			<a href="#map">Таблица</a>
			<a href="#layouts">Layouts</a>
			<a href="#brand">Бренд</a>
			<a href="#chrome">Chrome</a>
			<a href="#forms">Forms</a>
			<a href="#data">Data</a>
			<a href="#overlays">Overlays</a>
			<a href="#admin">Admin</a>
			<a href="#acceptance">Приёмка</a>
			<a href="#smoke">Smoke</a>
		</nav>
	</header>

	<section id="map" class="sec">
		<h2>Компонент → эталонный экран</h2>
		<table class="map">
			<thead>
				<tr>
					<th>Группа</th>
					<th>Компонент</th>
					<th>Экран</th>
					<th>Smoke</th>
				</tr>
			</thead>
			<tbody>
				{#each COMPONENT_MAP as group (group.id)}
					{#each group.entries as entry, i (entry.component)}
						<tr>
							{#if i === 0}
								<td rowspan={group.entries.length} class="grp">{group.title}</td>
							{/if}
							<td><code>{entry.component}</code></td>
							<td><code>#{entry.screen}</code></td>
							<td>
								{#if entry.smoke}
									<a href={entry.smoke}>{entry.smoke}</a>
								{:else}
									<span class="muted">—</span>
								{/if}
							</td>
						</tr>
					{/each}
				{/each}
			</tbody>
		</table>
	</section>

	<section id="layouts" class="sec">
		<h2>Layouts — 6 каркасов</h2>
		<div class="phones">
			<figure>
				<figcaption>PlainLayout · #e1-1</figcaption>
				<PlainLayout height="320px">
					<ScreenTitle style="margin-top:24px">Вас пригласили</ScreenTitle>
					<InviteCard initial="С" name="Семья" preview="22 участника · пригласила Аня" />
					<Button onclick={() => {}}>Получить код</Button>
				</PlainLayout>
			</figure>
			<figure>
				<figcaption>ShellLayout · #e2-1</figcaption>
				<ShellLayout height="320px" onsearch={() => {}} onsettings={() => {}}>
					{#snippet fab()}
						<Icon name="plus" style="width:26px;height:26px;stroke-width:1.5" />
					{/snippet}
					<SectionLabel>Закреплённые</SectionLabel>
					<CircleRow
						initial="С"
						name="Семья"
						preview="Аня: были на даче"
						time="14:02"
						badge={3}
						color="var(--terracotta)"
					/>
				</ShellLayout>
			</figure>
			<figure>
				<figcaption>CircleLayout · #e3-1</figcaption>
				<CircleLayout title="Семья" identity="Мышь" avatar="М" height="360px">
					<PostCard>
						{#snippet author()}
							<Avatar initial="А" color="#58673A" />
							<div>
								<div class="n">Аня</div>
								<div class="tm">сегодня, 14:02</div>
							</div>
						{/snippet}
						{#snippet text()}Были на даче, все живы.{/snippet}
					</PostCard>
				</CircleLayout>
			</figure>
			<figure>
				<figcaption>FormLayout · #e1-3 (вступление)</figcaption>
				<FormLayout app circleTitle="Семья" {color} height="220px">
					<div class="h1s" style="margin-top:22px;line-height:1.25">
						Как вас зовут<br />в этом круге?
					</div>
				</FormLayout>
			</figure>
			<figure>
				<figcaption>FormLayout · #e2-4</figcaption>
				<FormLayout title="Новый круг" {color} height="400px">
					<Label>Сервер</Label>
					<ServerRow
						name="Дом Ани"
						subtitle="home.example.org · ещё 5 ваших кругов"
						variant="select"
						style="padding:2px 16px 10px"
					/>
					<Label>Название</Label>
					<FieldDisplay value="Дача" active />
					<Label>Цвет</Label>
					<ColorSwatches bind:value={color} />
					<Button variant="colored" onclick={() => {}}>Создать</Button>
				</FormLayout>
			</figure>
			<figure>
				<figcaption>OverlayLayout · #e4-12</figcaption>
				<div class="overlay-wrap">
					<CircleLayout title="Семья" identity="Мышь" avatar="М" commentBar={false} height="360px">
						<PostCard>
							{#snippet author()}
								<Avatar initial="А" color="#58673A" />
								<div>
									<div class="n">Аня</div>
									<div class="tm">сегодня</div>
								</div>
							{/snippet}
							{#snippet text()}Были на даче.{/snippet}
						</PostCard>
						<OverlayLayout>
							<SectionLabel>Реакция · 2</SectionLabel>
							<ReactionListRow initial="К" name="Кот" color="#62452F" icon="heart" />
							<ReactionListRow initial="П" name="Петя" color="#357077" icon="heart" />
						</OverlayLayout>
					</CircleLayout>
				</div>
			</figure>
			<figure>
				<figcaption>AdminWideLayout · #e9-5</figcaption>
				<AdminWideLayout active="Хранилище" server="Дом Ани" height="320px">
					<AdminSection title="Хранилище">
						<StackBar
							threshold="90%"
							segments={[{ width: '42%', color: 'var(--terracotta)' }]}
						/>
					</AdminSection>
				</AdminWideLayout>
			</figure>
		</div>
	</section>

	<section id="brand" class="sec">
		<h2>Бренд</h2>
		<div class="row">
			<div class="card">
				<h3>Mark · #e2-1</h3>
				<div class="demo-inline" style="padding:20px">
					<Mark />
				</div>
			</div>
			<div class="card" id="loading-demo">
				<h3>Loading · экран, пока грузится</h3>
				<div class="demo-inline">
					<Loading style="min-height:160px" />
				</div>
			</div>
			<div class="card">
				<h3>Logo · #e1-5</h3>
				<div class="demo-inline" style="padding:20px">
					<Logo />
				</div>
			</div>
			<div class="card wide">
				<h3>Icon · #e2-1</h3>
				<div class="icons">
					{#each iconNames as name (name)}
						<span title={name}><Icon {name} /></span>
					{/each}
				</div>
			</div>
		</div>
	</section>

	<section id="chrome" class="sec">
		<h2>Chrome</h2>
		<div class="phones">
			<figure>
				<figcaption>AppBar · #e2-1</figcaption>
				<PhoneFrame shell height="120px">
					<StatusBar />
					<AppBar onsearch={() => {}} onsettings={() => {}} />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>CircleBar · #e3-1</figcaption>
				<PhoneFrame color="terracotta" height="140px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>BackBar · #e2-4</figcaption>
				<PhoneFrame color="ochre" height="100px">
					<StatusBar />
					<BackBar title="Новый круг" />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>AdminBar · #e9-5</figcaption>
				<PhoneFrame shell wide height="80px">
					<AdminBar active="Хранилище" server="Дом Ани · home.example.org" />
				</PhoneFrame>
			</figure>
		</div>
	</section>

	<section id="forms" class="sec">
		<h2>Forms</h2>
		<div class="phones">
			<figure>
				<figcaption>#e1-1 — приглашение</figcaption>
				<PhoneFrame height="420px">
					<StatusBar />
					<ScreenTitle style="margin-top:24px">Вас пригласили</ScreenTitle>
					<InviteCard initial="С" name="Семья" preview="22 участника" />
					<Label>Почта</Label>
					<FieldDisplay value="you@example.com" active />
					<Hint>Пришлём код для входа.</Hint>
					<Button onclick={() => {}}>Получить код</Button>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e1-2 — CodeBox</figcaption>
				<PhoneFrame shell height="280px">
					<StatusBar />
					<ScreenTitle>Код из письма</ScreenTitle>
					<CodeBox digits={['4', '1', '8']} active={3} />
					<Hint>Код действует 15 минут.</Hint>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e2-4 — Chip, ColorSwatches</figcaption>
				<PhoneFrame color="ochre" height="300px">
					<StatusBar />
					<BackBar title="Новый круг" />
					<Label>Цвет</Label>
					<ColorSwatches bind:value={color} />
					<Label>Окно правок</Label>
					<ChipGroup>
						<Chip>Летопись</Chip>
						<Chip selected>Час</Chip>
						<Chip>Сутки</Chip>
					</ChipGroup>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-2 — Meter, DangerZone</figcaption>
				<PhoneFrame color="terracotta" height="380px">
					<StatusBar />
					<CircleBar title="Настройки" tabs={false} />
					<Meter value={8.4} max={10} />
					<Hint style="margin-top:8px">8,4 ГБ из 10 ГБ</Hint>
					<DangerZone items={['Покинуть круг', 'Удалить круг']} />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-10 — VolumeChart</figcaption>
				<PhoneFrame color="terracotta" height="360px">
					<StatusBar />
					<CircleBar title="Архив" tabs={false} />
					<Label style="margin-top:8px">Сколько освободит отсечка</Label>
					<VolumeChart
						volume={demoVolume}
						cutoffLabel={demoCutoffLabel}
						bind:cutoffX={demoCutoffX}
						oncutoff={onDemoCutoff}
					/>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-12 — DangerNote</figcaption>
				<PhoneFrame color="terracotta" height="260px">
					<StatusBar />
					<CircleBar title="Сроки" tabs={false} />
					<DangerNote>20 сентября всё до 1 июня удалится с сервера.</DangerNote>
				</PhoneFrame>
			</figure>
		</div>
		<div class="row" style="margin-top:20px">
			<div class="card">
				<h3>TextArea · #e4-2</h3>
				<TextArea value="Что нового?" active />
			</div>
			<div class="card">
				<h3>Switch · #e6-9</h3>
				<Switch bind:checked={swOn} />
			</div>
			<div class="card">
				<h3>SearchField · #e2-3</h3>
				<SearchField placeholder="Поиск по кругам" />
			</div>
			<div class="card">
				<h3>RequisitesCard · #e10-2</h3>
				<RequisitesCard text={'Сбербанк\n+7 900 000-00-00\nИван И.'} />
			</div>
			<div class="card">
				<h3>CodeBox · ввод</h3>
				<CodeBox bind:value={demoCode} />
			</div>
			<div class="card">
				<h3>PeopleStrip · #e1-3</h3>
				<PeopleStrip
					people={[
						{ name: 'Аня', initial: 'А', color: '#58673A' },
						{ name: 'Петя', initial: 'П', color: '#357077' }
					]}
				/>
			</div>
			<div class="card">
				<h3>MentionPicker · #e4-2</h3>
				<MentionPicker>
					<MemberRow initial="А" name="Аня" color="#58673A" onclick={() => {}} />
					<MemberRow initial="П" name="Петя" color="#357077" onclick={() => {}} />
				</MentionPicker>
			</div>
			<div class="card">
				<h3>AddPhotoButton · #e4-2, #e1-3</h3>
				<AddPhotoButton onclick={() => {}} />
				<AddPhotoButton
					onclick={() => {}}
					previewUrl="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='92' height='92'%3E%3Crect fill='%2358673A' width='92' height='92'/%3E%3C/svg%3E"
				/>
			</div>
			<div class="card">
				<h3>EditWindowPicker · #e2-4, #e6-2</h3>
				<EditWindowPicker
					value={demoEditWindow}
					bind:customHours={demoCustomHours}
					onpick={(key) => (demoEditWindow = key)}
				/>
			</div>
			<div class="card">
				<h3>IdentityForm · #e1-3</h3>
				<IdentityForm color="ochre" onerror={() => {}} />
			</div>
			<div class="card">
				<h3>MentionText · #e3-1, #e4-5</h3>
				<MentionText body={'Были на даче с @Аня.\n\nЗавтра — снова.'} />
			</div>
			<div class="card">
				<h3>Button variants</h3>
				<div style="display:flex;flex-direction:column;gap:8px">
					<Button onclick={() => {}}>default</Button>
					<Button variant="colored" onclick={() => {}}>colored</Button>
					<Button variant="ghost" onclick={() => {}}>ghost</Button>
					<Button variant="off" onclick={() => {}}>off</Button>
				</div>
			</div>
		</div>
	</section>

	<section id="data" class="sec">
		<h2>Data</h2>
		<div class="phones">
			<figure>
				<figcaption>#e3-1 — PostCard, ReactionBar, CommentPreview</figcaption>
				<PhoneFrame color="terracotta" height="400px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<EventDivider text="Мышка теперь Мышь" />
					<PostCard>
						{#snippet author()}
							<Avatar initial="А" color="#58673A" />
							<div>
								<div class="n">Аня</div>
								<div class="tm">сегодня, 14:02</div>
							</div>
						{/snippet}
						{#snippet text()}Были на даче, все живы.{/snippet}
						{#snippet reactions()}
							<ReactionBar
								groups={[{ icon: 'heart', names: 'Кот, Петя' }]}
								keys={['heart', 'laugh', 'surprise', 'anger']}
								showAdd
								onopenList={() => {}}
								onadd={() => {}}
								onpick={() => {}}
							/>
						{/snippet}
					</PostCard>
					<CommentPreview onclick={() => {}}>
						<div>Петя: а компот будет? <span class="tm">сегодня, 14:22</span></div>
						<div class="mo">ещё 11 комментариев</div>
					</CommentPreview>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e4-5 — CommentRow</figcaption>
				<PhoneFrame color="terracotta" height="320px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<div class="thread">
						<CommentRow
							initial="П"
							name="Петя"
							color="#357077"
							onedit={() => {}}
							ondelete={() => {}}
						>
							{#snippet time()}14:18{/snippet}
							{#snippet children()}А компот будет?{/snippet}
						</CommentRow>
					</div>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e4-7 — CommentRow · правка</figcaption>
				<PhoneFrame color="terracotta" height="360px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<div class="thread">
						<CommentRow initial="М" name="Мышь" color="#3C4D83">
							{#snippet time()}14:22{/snippet}
							{#snippet children()}
								<TextArea variant="field" class="ced" value="А компот будет?" />
								<div class="rowin">
									<Button variant="colored" style="flex:1;margin:0" onclick={() => {}}>Сохранить</Button>
									<Button variant="ghost" style="flex:1;margin:0" onclick={() => {}}>Отмена</Button>
								</div>
							{/snippet}
						</CommentRow>
					</div>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e4-7 — CommentRow · очередь `.q`</figcaption>
				<PhoneFrame color="terracotta" height="280px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<div class="thread">
						<CommentRow queued initial="М" name="Мышь" color="#3C4D83">
							{#snippet time()}
								<span style="display:flex;align-items:center;gap:5px">
									<Icon name="clock" size="xs" />в очереди
								</span>
							{/snippet}
							{#snippet children()}Жду ответ про компот.{/snippet}
						</CommentRow>
					</div>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e4-10 — ReactionBar picker</figcaption>
				<PhoneFrame color="terracotta" height="280px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<PostCard>
						{#snippet author()}
							<Avatar initial="А" color="#58673A" />
							<div>
								<div class="n">Аня</div>
								<div class="tm">сегодня, 14:02</div>
							</div>
						{/snippet}
						{#snippet text()}Были на даче, все живы.{/snippet}
						{#snippet reactions()}
							<ReactionBar
								groups={[{ icon: 'heart', names: 'Кот, Петя' }]}
								keys={['heart', 'laugh', 'surprise', 'anger']}
								pickerOpen
								onopenList={() => {}}
								onadd={() => {}}
								onpick={() => {}}
							/>
						{/snippet}
					</PostCard>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e3-2 — EventDivider variant="unread"</figcaption>
				<PhoneFrame color="terracotta" height="300px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" tabs={false} />
					<PostCard>
						{#snippet author()}
							<Avatar initial="К" color="#62452F" />
							<div>
								<div class="n">Кот</div>
								<div class="tm">сегодня, 11:20</div>
							</div>
						{/snippet}
						{#snippet text()}Компот будет. Три банки.{/snippet}
					</PostCard>
					<EventDivider text="выше — новое" variant="unread" />
					<PostCard>
						{#snippet author()}
							<Avatar initial="М" color="#3C4D83" />
							<div>
								<div class="n">Мама</div>
								<div class="tm">вчера, 21:40</div>
							</div>
						{/snippet}
						{#snippet text()}Дед нашёл на чердаке коробку с плёнками.{/snippet}
					</PostCard>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e2-9 — SearchResultRow</figcaption>
				<PhoneFrame shell height="280px">
					<StatusBar />
					<BackBar compact search="яблони" />
					<SearchGroupHeader color="var(--terracotta)" name="Семья" count={2} />
					<SearchResultRow author="Аня" time="сегодня" thumb thumbVariant="p1">
						{#snippet preview()}
							Были на даче. <span class="hl">Яблони</span>…
						{/snippet}
					</SearchResultRow>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e5-1 — DayCard, DayGrid</figcaption>
				<PhoneFrame color="terracotta" height="340px">
					<StatusBar />
					<CircleBar title="Семья" identity="Мышь" avatar="М" active="Дни" />
					<MonthLabel>Август 2026</MonthLabel>
					<DayGrid>
						<DayCard title="Дача" subtitle="12 авг · 4 записи" cover="p1" photoCount={12} />
						<DayCard title="11 августа" subtitle="2 записи" />
					</DayGrid>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-3 — MemberRow</figcaption>
				<PhoneFrame color="terracotta" height="240px">
					<StatusBar />
					<CircleBar title="Участники" tabs={false} />
					<MemberRow initial="К" name="Кот" subtitle="владелец" color="#62452F" />
					<MemberRow initial="А" name="Аня" subtitle="может менять настройки" color="#58673A" />
					<SettingsRow title="ещё 19" link />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-9 — SettingsRow + Switch</figcaption>
				<PhoneFrame color="terracotta" height="360px">
					<StatusBar />
					<CircleBar title="Уведомления круга" tabs={false} />
					<Label>Присылать</Label>
					<SettingsRow title="Новые записи" style="padding-top:2px">
						{#snippet control()}
							<Switch bind:checked={swOn} />
						{/snippet}
					</SettingsRow>
					<SettingsRow title="Комментарии к моим записям">
						{#snippet control()}
							<Switch bind:checked={swOn} />
						{/snippet}
					</SettingsRow>
					<SettingsRow title="Упоминания" subtitle="всегда">
						{#snippet control()}
							<Switch checked={true} disabled />
						{/snippet}
					</SettingsRow>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e1-6 — ServerRow</figcaption>
				<PhoneFrame height="200px">
					<StatusBar />
					<ServerRow name="Дом Ани" subtitle="home.example.org" variant="select" card />
					<ServerRow name="У Славы" subtitle="slava.example.org" variant="ok" />
					<ServerRow name="Закрытый" subtitle="closed.example.org" variant="warn" />
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e5-4 — EntryDateMark</figcaption>
				<PhoneFrame color="terracotta" height="160px">
					<StatusBar />
					<CircleBar title="Семья" tabs={false} />
					<div class="row2" style="padding:10px 16px">
						<Avatar initial="А" color="#58673A" />
						<div class="g">
							<div class="n">Аня</div>
						</div>
						<EntryDateMark label="12 июля" />
					</div>
				</PhoneFrame>
			</figure>
		</div>
		<div class="row" style="margin-top:20px">
			<div class="card">
				<h3>DayHeader · #e5-2</h3>
				<PhoneFrame color="terracotta" height="200px">
					<DayHeader
						cover="p1"
						title="Дача, яблони"
						subtitle="12 августа · 4 записи"
						oncover={() => {}}
					/>
				</PhoneFrame>
			</div>
			<div class="card">
				<h3>ArchiveBanner · #e6-15</h3>
				<PhoneFrame color="terracotta" height="200px">
					<ArchiveBanner title="Архив готов">
						Скачайте до 20 сентября — потом удалится с сервера.
					</ArchiveBanner>
				</PhoneFrame>
			</div>
			<div class="card">
				<h3>FeedDayPromptCard · лента</h3>
				<PhoneFrame color="terracotta" height="280px">
					<FeedDayPromptCard
						title="Первая запись за 12 августа"
						primaryLabel="Назвать день"
						secondaryLabel="Потом"
						onprimary={() => {}}
						onsecondary={() => {}}
					>
						Дать этому дню название и обложку? День общий — увидят все участники.
					</FeedDayPromptCard>
				</PhoneFrame>
			</div>
			<div class="card">
				<h3>PayStreetBanner · #e10-5</h3>
				<PhoneFrame height="320px">
					<PayStreetBanner
						variant="donate"
						text="Поддержите сервер — реквизиты в разделе оплаты."
						dismissible
						onclick={() => {}}
						ondismiss={() => {}}
					/>
					<PayStreetBanner
						variant="reminder"
						expiresAtLabel="12 ноября 2026"
						reminderDaysLeft={5}
						onclick={() => {}}
					/>
					<PayStreetBanner
						variant="pending"
						pendingAtLabel="10 сентября 2026"
						expiresAtLabel="12 ноября 2026"
					/>
				</PhoneFrame>
			</div>
			<div class="card">
				<h3>FoldHeader · #e4-5</h3>
				<FoldHeader label="Комментарии" count={11} />
			</div>
			<div class="card">
				<h3>GroupFoldCard · #e2-14</h3>
				<GroupFoldCard
					label="Семья"
					count={3}
					expanded={true}
					foldStyle="padding-top:12px"
					onclick={() => {}}
					actionLabel="Переименовать"
					onaction={() => {}}
					actionLabel2="Удалить группу"
					onaction2={() => {}}
				/>
			</div>
			<div class="card">
				<h3>AttachmentRow · #e4-13, #e4-15</h3>
				<AttachmentRow filename="scan.pdf" size="1,2 МБ" />
				<AttachmentRow
					audio
					filename="Бригада — Утро"
					preview={{ progress: 0.38, time: '0:42 · 1:51' }}
				/>
			</div>
			<div class="card">
				<h3>PhotoGrid · #e4-2</h3>
				<PhotoGrid>
					<PhotoPlaceholder variant="p1" photoCount={3} />
					<PhotoPlaceholder variant="p2" />
					<PhotoPlaceholder empty />
				</PhotoGrid>
			</div>
			<div class="card">
				<h3>MapBadge / MapPostSheet · карта</h3>
				<PhoneFrame color="terracotta" height="200px">
					<div class="map-wrap" style="min-height:200px">
						<MapBadge badge="3 записи с местом из 12" />
						<MapPostSheet
							author="Аня"
							time="12 августа, 18:40"
							body="Яблони у забора — уже краснеют."
							onclick={() => {}}
						/>
					</div>
				</PhoneFrame>
			</div>
			<div class="card">
				<h3>MediaTile · #e3-1</h3>
				<MediaTile variant="feed" count={3} onclick={() => {}} />
				<PhotoGrid style="margin-top:8px">
					<MediaTile variant="grid" count={2} onclick={() => {}} />
					<MediaTile variant="album" coverLabel="обложка" onclick={() => {}} />
					<MediaTile variant="album" coverLabel="обложка" selected onclick={() => {}} />
				</PhotoGrid>
				<div class="rowin" style="margin-top:12px;align-items:flex-start;gap:8px">
					<MediaTile
						variant="headerMini"
						src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='88' height='88'%3E%3Crect fill='%23c4b5a0' width='88' height='88'/%3E%3C/svg%3E"
						aria-label="Миниатюра в шапке дня"
						onclick={() => {}}
					/>
					<MediaTile variant="compose" isCover onclick={() => {}} />
					<MediaTile variant="compose" fileName="scan.pdf" onclick={() => {}} />
				</div>
			</div>
		</div>
	</section>

	<section id="overlays" class="sec">
		<h2>Overlays</h2>
		<div class="phones">
			<figure>
				<figcaption>#e2-1 — Fab</figcaption>
				<ShellLayout height="280px" onsearch={() => {}} onsettings={() => {}}>
					{#snippet fab()}
						<Icon name="plus" style="width:26px;height:26px;stroke-width:1.5" />
					{/snippet}
					<SectionLabel>Закреплённые</SectionLabel>
					<CircleRow
						initial="С"
						name="Семья"
						preview="Аня: были на даче"
						time="14:02"
						color="var(--terracotta)"
					/>
				</ShellLayout>
			</figure>
			<figure>
				<figcaption>#e2-11 — Fab menu</figcaption>
				<ShellLayout
					height="320px"
					onsearch={() => {}}
					onsettings={() => {}}
					fabMenuOpen={true}
					fabMenuItems={[
						{ label: 'Новый круг', onclick: () => {} },
						{ label: 'Новая группа', onclick: () => {} }
					]}
				>
					{#snippet fab()}
						<Icon name="plus" style="width:26px;height:26px;stroke-width:1.5" />
					{/snippet}
					<SectionLabel>Круги</SectionLabel>
				</ShellLayout>
			</figure>
			<figure>
				<figcaption>#e3-1 — CommentBar</figcaption>
				<CircleLayout
					title="Семья"
					identity="Мышь"
					avatar="М"
					height="280px"
					onCommentSend={() => {}}
					onCommentCompose={() => {}}
					onCommentPhotos={() => {}}
				>
					<PostCard>
						{#snippet author()}
							<Avatar initial="А" color="#58673A" />
							<div>
								<div class="n">Аня</div>
								<div class="tm">сегодня, 14:02</div>
							</div>
						{/snippet}
						{#snippet text()}Были на даче, все живы.{/snippet}
					</PostCard>
				</CircleLayout>
			</figure>
			<figure>
				<figcaption>#e4-14 — Lightbox</figcaption>
				<PhoneFrame height="360px">
					<Lightbox
						counter="3 из 12"
						caption="Аня · сегодня · Дача"
						onclose={() => {}}
						ondownload={() => {}}
					>
						{#snippet media()}
							<div class="pic p2" style="width:100%;height:240px;border-radius:0;border:0"></div>
						{/snippet}
						{#snippet dots()}
							<u></u><u></u><u class="on"></u><u></u>
						{/snippet}
					</Lightbox>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e7-4 — PushBanner</figcaption>
				<PhoneFrame height="200px">
					<PushBanner
						initial="С"
						color="var(--terracotta)"
						title="Семья"
						subtitle="3 новые записи"
						time="сейчас"
						style="top:60px"
					/>
				</PhoneFrame>
			</figure>
			<figure>
				<figcaption>#e6-2 — AvatarCrop</figcaption>
				<PhoneFrame color="terracotta" height="200px">
					<Button variant="colored" style="margin:24px 16px" onclick={() => void openCropDemo()}>
						Открыть кадрирование
					</Button>
				</PhoneFrame>
			</figure>
		</div>
	</section>

	{#if cropFile}
		<AvatarCrop file={cropFile} color="terracotta" ondone={() => (cropFile = undefined)} oncancel={() => (cropFile = undefined)} />
	{/if}

	<section id="admin" class="sec">
		<h2>Admin</h2>
		<div class="row">
			<div class="card">
				<h3>AdminNav · #e9-5</h3>
				<AdminNav active="Проверка" />
			</div>
			<div class="card">
				<h3>StatusIcon · #e9-8</h3>
				<div class="status-icons">
					<StatusIcon status="ok" />
					<StatusIcon status="warn" />
					<StatusIcon status="bad" />
				</div>
			</div>
			<div class="card wide">
				<h3>CheckRow · #e9-8</h3>
				<CheckRow
					status="ok"
					name="HTTPS снаружи"
					description="200 за 180 мс"
				/>
				<CheckRow
					status="warn"
					name="Бэкап"
					description="каталог ещё ни разу не копировали"
				/>
			</div>
		</div>
		<div class="phones" style="margin-top:20px">
			<figure>
				<figcaption>#e9-5 — DataTable, QuotaRequestRow</figcaption>
				<AdminWideLayout active="Хранилище" server="Дом Ани" height="400px">
					<AdminSection title="Хранилище">
						<DataTable>
							<thead>
								<tr>
									<th>Круг</th>
									<th>Записей</th>
									<th>Квота</th>
								</tr>
							</thead>
							<tbody>
								<tr>
									<td class="n"
										><span class="dot" style="background:var(--terracotta)"></span>Семья</td
									>
									<td>340</td>
									<td>10 ГБ</td>
								</tr>
							</tbody>
						</DataTable>
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
						<div style="margin-top:14px">
							<InlineInput>40 ГБ</InlineInput>
						</div>
						<AdminField label="Длинная сторона" unit="px" class="mt-14">
							<InlineInput>2048</InlineInput>
						</AdminField>
						<Panel class="mt-14 sz-13">Оригиналы остаются на телефоне.</Panel>
					</AdminSection>
				</AdminWideLayout>
			</figure>
			<figure>
				<figcaption>#e9-9 — CodeBlock</figcaption>
				<PhoneFrame shell wide height="200px">
					<CodeBlock>
						proxy_set_header X-Forwarded-For $remote_addr;<br />
						client_max_body_size 32m;
					</CodeBlock>
				</PhoneFrame>
			</figure>
		</div>
	</section>

	<section id="acceptance" class="sec">
		<h2>Критерии приёмки</h2>
		<ul class="checklist">
			{#each ACCEPTANCE as item (item.rule)}
				<li class:ok={item.ok}>
					<span class="mark">{item.ok ? '☑' : '☐'}</span>
					{item.rule}
				</li>
			{/each}
		</ul>
	</section>

	<section id="smoke" class="sec">
		<h2>Smoke-тесты</h2>
		<p class="muted">Полные экраны для визуальной сверки с <code>docs/visual/screens.html</code>.</p>
		<div class="smoke-grid">
			{#each SMOKE_ROUTES as route (route.id)}
				<a class="smoke" href={route.href}>
					<code>#{route.id}</code>
					{route.label}
				</a>
			{/each}
		</div>
	</section>
</main>

<style>
	:global(.app.dev) {
		display: block;
		padding: 32px 24px 80px;
	}

	.catalog {
		max-width: 1200px;
		margin: 0 auto;
		font-size: 14px;
		line-height: 1.5;
	}

	.head h1 {
		font-size: 17px;
		margin: 0 0 8px;
	}

	.head p {
		margin: 0 0 16px;
		color: var(--muted);
		font-size: 13px;
	}

	.toc {
		display: flex;
		flex-wrap: wrap;
		gap: 6px 14px;
		font-size: 12.5px;
		margin-bottom: 32px;
	}

	.toc a {
		color: var(--ink);
	}

	.sec {
		margin-bottom: 48px;
	}

	.sec h2 {
		font-size: 13.5px;
		font-weight: 600;
		margin: 0 0 16px;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--line);
	}

	.map {
		width: 100%;
		border-collapse: collapse;
		font-size: 12.5px;
	}

	.map th,
	.map td {
		text-align: left;
		padding: 7px 12px 7px 0;
		border-bottom: 1px solid var(--line);
		vertical-align: top;
	}

	.map th {
		font-size: 11px;
		letter-spacing: 0.09em;
		text-transform: uppercase;
		color: var(--faint);
	}

	.map .grp {
		font-weight: 600;
		color: var(--muted);
		vertical-align: top;
		padding-right: 20px;
	}

	.phones {
		display: flex;
		flex-wrap: wrap;
		gap: 24px;
	}

	.phones figure {
		margin: 0;
	}

	.phones figcaption {
		font-size: 11.5px;
		color: var(--faint);
		margin-bottom: 8px;
	}

	.overlay-wrap :global(.ph) {
		position: relative;
	}

	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
	}

	.card {
		flex: 1 1 200px;
		border: 1px solid var(--line);
		border-radius: 12px;
		padding: 14px 16px;
		background: var(--card);
	}

	.card.wide {
		flex: 2 1 320px;
	}

	.card h3 {
		font-size: 11.5px;
		color: var(--faint);
		margin: 0 0 12px;
		font-weight: 400;
	}

	.demo-inline {
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.icons {
		display: flex;
		flex-wrap: wrap;
		gap: 14px;
	}

	.status-icons {
		display: flex;
		gap: 12px;
	}

	.checklist {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.checklist li {
		display: flex;
		gap: 10px;
		padding: 8px 0;
		border-bottom: 1px solid var(--line);
		font-size: 13px;
	}

	.checklist .mark {
		flex-shrink: 0;
	}

	.checklist li.ok {
		color: var(--ink);
	}

	.smoke-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 8px;
	}

	.smoke {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 10px 12px;
		border: 1px solid var(--line);
		border-radius: 10px;
		text-decoration: none;
		color: var(--ink);
		font-size: 12.5px;
		background: var(--card);
	}

	.smoke:hover {
		border-color: var(--ink);
	}

	.smoke code {
		font-size: 11px;
		color: var(--faint);
	}

	.muted {
		color: var(--faint);
	}

	code {
		font-size: 12px;
	}
</style>

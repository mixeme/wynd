export type CatalogEntry = {
	component: string;
	screen: string;
	smoke?: string;
};

export type CatalogGroup = {
	id: string;
	title: string;
	entries: CatalogEntry[];
};

/** Wynd UI component map — компонент → эталонный экран в docs/screens.html */
export const COMPONENT_MAP: CatalogGroup[] = [
	{
		id: 'brand',
		title: 'Бренд',
		entries: [
			{ component: 'Mark', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'Logo', screen: 'e1-5', smoke: '/dev/smoke/e1-5' },
			{ component: 'Icon', screen: 'e2-1, e3-1', smoke: '/dev/smoke/e2-1' }
		]
	},
	{
		id: 'chrome',
		title: 'Chrome',
		entries: [
			{ component: 'PhoneFrame', screen: 'все' },
			{ component: 'StatusBar', screen: 'все' },
			{ component: 'AppBar', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'CircleBar', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'BackBar', screen: 'e2-3, e2-5', smoke: '/dev/smoke/e2-3' },
			{ component: 'AdminBar', screen: 'e9-1', smoke: '/dev/smoke/e9-1' }
		]
	},
	{
		id: 'forms',
		title: 'Forms',
		entries: [
			{ component: 'ScreenTitle', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'Label', screen: 'e1-1, e2-3', smoke: '/dev/smoke/e1-1' },
			{ component: 'Input', screen: 'e1-1, e2-3', smoke: '/dev/smoke/e1-1' },
			{ component: 'FieldDisplay', screen: 'e1-1, e2-3', smoke: '/dev/smoke/e1-1' },
			{ component: 'TextArea', screen: 'e4-1', smoke: '/dev/smoke/e4-1' },
			{ component: 'Hint', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'Button', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'InviteCard', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'CodeBox', screen: 'e1-2', smoke: '/dev/smoke/e1-2' },
			{ component: 'Chip / ChipGroup', screen: 'e2-3', smoke: '/dev/smoke/e2-3' },
			{ component: 'ColorSwatches', screen: 'e2-3', smoke: '/dev/smoke/e2-3' },
			{ component: 'Switch', screen: 'e6-6', smoke: '/dev/smoke/e6-6' },
			{ component: 'SearchField', screen: 'e2-2, e9-8' },
			{ component: 'Meter', screen: 'e6-2', smoke: '/dev/smoke/e6-2' },
			{ component: 'PeopleStrip', screen: 'e1-3' },
			{ component: 'AddPhotoButton', screen: 'e4-1', smoke: '/dev/smoke/e4-1' },
			{ component: 'IconButton', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'TextButton', screen: 'e4-1, e6-2', smoke: '/dev/smoke/e4-1' },
			{ component: 'DangerZone', screen: 'e6-2', smoke: '/dev/smoke/e6-2' },
			{ component: 'DangerNote', screen: 'e6-9', smoke: '/dev/smoke/e6-9' },
			{ component: 'VolumeChart', screen: 'e6-8', smoke: '/dev/smoke/e6-8' }
		]
	},
	{
		id: 'data',
		title: 'Data',
		entries: [
			{ component: 'SectionLabel', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'CircleRow', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'Avatar', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'PostCard', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'ReactionBar', screen: 'e3-1, e4-10', smoke: '/dev/smoke/e3-1' },
			{ component: 'CommentPreview', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'ReactionListRow', screen: 'e4-5', smoke: '/dev/smoke/e4-5' },
			{ component: 'EventDivider', screen: 'e3-1, e3-2', smoke: '/dev/smoke/e3-1' },
			{ component: 'SettingsRow', screen: 'e6-2, e6-6', smoke: '/dev/smoke/e6-2' },
			{ component: 'MemberRow', screen: 'e6-3', smoke: '/dev/smoke/e6-3' },
			{ component: 'SearchGroupHeader', screen: 'e2-5', smoke: '/dev/smoke/e2-5' },
			{ component: 'SearchResultRow', screen: 'e2-5', smoke: '/dev/smoke/e2-5' },
			{ component: 'ServerRow', screen: 'e1-6, e2-3', smoke: '/dev/smoke/e2-3' },
			{ component: 'FoldHeader', screen: 'e4-2' },
			{ component: 'AttachmentRow', screen: 'e4-3' },
			{ component: 'PhotoPlaceholder / PhotoGrid', screen: 'e4-1', smoke: '/dev/smoke/e4-1' },
			{ component: 'MonthLabel', screen: 'e5-1', smoke: '/dev/smoke/e5-1' },
			{ component: 'DayCard / DayGrid', screen: 'e5-1', smoke: '/dev/smoke/e5-1' },
			{ component: 'DayHeader', screen: 'e5-2' },
			{ component: 'EntryDateMark', screen: 'e3-9, e5-2' },
			{ component: 'ArchiveBanner', screen: 'e3-10' }
		]
	},
	{
		id: 'overlays',
		title: 'Overlays',
		entries: [
			{ component: 'Fab', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'CommentBar', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'Scrim / Sheet / Dialog', screen: 'e4-5', smoke: '/dev/smoke/e4-5' },
			{ component: 'Lightbox', screen: 'e4-4', smoke: '/dev/smoke/e4-4' },
			{ component: 'AvatarCrop', screen: 'e6-2' },
			{ component: 'PushBanner', screen: 'e7-4', smoke: '/dev/smoke/e7-4' }
		]
	},
	{
		id: 'admin',
		title: 'Admin',
		entries: [
			{ component: 'AdminNav', screen: 'e9-1', smoke: '/dev/smoke/e9-1' },
			{ component: 'AdminSection', screen: 'e9-1', smoke: '/dev/smoke/e9-1' },
			{ component: 'DataTable', screen: 'e9-1', smoke: '/dev/smoke/e9-1' },
			{ component: 'StackBar', screen: 'e9-1', smoke: '/dev/smoke/e9-1' },
			{ component: 'QuotaRequestRow', screen: 'e9-1', smoke: '/dev/smoke/e9-1' },
			{ component: 'CheckRow', screen: 'e9-4', smoke: '/dev/smoke/e9-4' },
			{ component: 'StatusIcon', screen: 'e9-4', smoke: '/dev/smoke/e9-4' },
			{ component: 'CodeBlock', screen: 'e9-5' },
			{ component: 'InlineInput', screen: 'e9-1', smoke: '/dev/smoke/e9-1' }
		]
	},
	{
		id: 'layouts',
		title: 'Layouts',
		entries: [
			{ component: 'PlainLayout', screen: 'e1-1, e1-3', smoke: '/dev/smoke/e1-1' },
			{ component: 'ShellLayout', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'CircleLayout', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'FormLayout', screen: 'e2-3', smoke: '/dev/smoke/e2-3' },
			{ component: 'OverlayLayout', screen: 'e4-5', smoke: '/dev/smoke/e4-5' },
			{ component: 'AdminWideLayout', screen: 'e9-1', smoke: '/dev/smoke/e9-1' }
		]
	}
];

export const SMOKE_ROUTES = [
	{ id: 'e1-1', label: 'Приглашение', href: '/dev/smoke/e1-1' },
	{ id: 'e1-2', label: 'Код из письма', href: '/dev/smoke/e1-2' },
	{ id: 'e1-5', label: 'Войти', href: '/dev/smoke/e1-5' },
	{ id: 'e2-1', label: 'Список кругов', href: '/dev/smoke/e2-1' },
	{ id: 'e2-3', label: 'Новый круг', href: '/dev/smoke/e2-3' },
	{ id: 'e2-5', label: 'Поиск', href: '/dev/smoke/e2-5' },
	{ id: 'e3-1', label: 'Хронология', href: '/dev/smoke/e3-1' },
	{ id: 'e4-1', label: 'Новая запись', href: '/dev/smoke/e4-1' },
	{ id: 'e4-4', label: 'Лайтбокс', href: '/dev/smoke/e4-4' },
	{ id: 'e4-5', label: 'Реакции', href: '/dev/smoke/e4-5' },
	{ id: 'e5-1', label: 'Дни', href: '/dev/smoke/e5-1' },
	{ id: 'e6-2', label: 'Настройки круга', href: '/dev/smoke/e6-2' },
	{ id: 'e6-3', label: 'Участники', href: '/dev/smoke/e6-3' },
	{ id: 'e6-8', label: 'Место кончилось', href: '/dev/smoke/e6-8' },
	{ id: 'e6-9', label: 'Сроки архивации', href: '/dev/smoke/e6-9' },
	{ id: 'e7-4', label: 'Пуши', href: '/dev/smoke/e7-4' },
	{ id: 'e9-1', label: 'Хранилище', href: '/dev/smoke/e9-1' },
	{ id: 'e9-4', label: 'Проверка', href: '/dev/smoke/e9-4' }
] as const;

export const ACCEPTANCE = [
	{
		rule: 'Dev-каталог показывает ~48 компонентов и 6 layouts',
		ok: true
	},
	{
		rule: 'Визуально совпадает с screens.html (390px, кегли 17 / 13.5 / 12.5 / 11.5)',
		ok: true
	},
	{
		rule: 'Shell без цвета круга; accent только через --c',
		ok: true
	},
	{
		rule: 'Danger — ink border, не red',
		ok: true
	},
	{
		rule: 'Mark только в AppBar и 5 других мест (по правилам screens.html)',
		ok: true
	}
] as const;

export function countComponents(): number {
	return COMPONENT_MAP.filter((group) => group.id !== 'layouts').reduce(
		(sum, group) => sum + group.entries.length,
		0
	);
}

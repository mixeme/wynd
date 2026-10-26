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
			{ component: 'Logo', screen: 'e1-6', smoke: '/dev/smoke/e1-6' },
			{ component: 'Loading', screen: '—' },
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
			{ component: 'BackBar', screen: 'e2-14, e2-19', smoke: '/dev/smoke/e2-14' },
			{ component: 'AdminBar', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'ComposeToolbar', screen: 'e4-2' },
			{ component: 'AudioBar', screen: 'e4-9' },
			{ component: 'ConnectionStrip', screen: 'e7-37, e7-38, e7-39' }
		]
	},
	{
		id: 'forms',
		title: 'Forms',
		entries: [
			{ component: 'ScreenTitle', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'Label', screen: 'e1-1, e2-14', smoke: '/dev/smoke/e1-1' },
			{ component: 'Input', screen: 'e1-1, e2-14', smoke: '/dev/smoke/e1-1' },
			{ component: 'FieldDisplay', screen: 'e1-1, e2-14', smoke: '/dev/smoke/e1-1' },
			{ component: 'TextArea', screen: 'e4-2', smoke: '/dev/smoke/e4-2' },
			{ component: 'Hint', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'Button', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'InviteCard', screen: 'e1-1', smoke: '/dev/smoke/e1-1' },
			{ component: 'RequisitesCard', screen: 'e10-1, e10-2, e10-6, e10-8' },
			{ component: 'CodeBox', screen: 'e1-3', smoke: '/dev/smoke/e1-3' },
			{ component: 'Chip / ChipGroup', screen: 'e2-14', smoke: '/dev/smoke/e2-14' },
			{ component: 'ColorSwatches', screen: 'e2-14', smoke: '/dev/smoke/e2-14' },
			{ component: 'Switch', screen: 'e6-14', smoke: '/dev/smoke/e6-14' },
			{ component: 'SearchField', screen: 'e2-6, e9-3' },
			{ component: 'Meter', screen: 'e6-2', smoke: '/dev/smoke/e6-2' },
			{ component: 'PeopleStrip', screen: 'e1-4' },
			{ component: 'MentionPicker', screen: 'e4-2, e4-4' },
			{ component: 'AddPhotoButton', screen: 'e4-2, e1-4', smoke: '/dev/smoke/e4-2' },
			{ component: 'IconButton', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'TextButton', screen: 'e4-2, e6-2', smoke: '/dev/smoke/e4-2' },
			{ component: 'DangerZone', screen: 'e6-2, e7-15', smoke: '/dev/smoke/e6-2' },
			{ component: 'DangerNote', screen: 'e6-21', smoke: '/dev/smoke/e6-21' },
			{ component: 'VolumeChart', screen: 'e6-17', smoke: '/dev/smoke/e6-17' },
			{ component: 'EditWindowPicker', screen: 'e2-14, e6-2', smoke: '/dev/smoke/e2-14' },
			{ component: 'IdentityForm', screen: 'e1-4' },
			{ component: 'NumberField', screen: 'e2-14, e6-2, e6-12' },
			{ component: 'DateRow', screen: 'e4-2' },
			{ component: 'DateRange', screen: 'e2-19' },
			{ component: 'FilePicker', screen: 'e4-2, e1-4, e10-2' },
			{ component: 'QrScanner', screen: 'e2-8' },
			{ component: 'GenderPicker', screen: 'e1-4, e6-12' }
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
			{ component: 'ReactionBar', screen: 'e3-1, e4-26', smoke: '/dev/smoke/e3-1' },
			{ component: 'CommentPreview', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'CommentRow', screen: 'e4-17, e4-18, e4-19' },
			{ component: 'CommentMedia', screen: 'e4-21, e4-22', smoke: '/dev/smoke/e4-21' },
			{ component: 'ReactionListRow', screen: 'e4-28', smoke: '/dev/smoke/e4-28' },
			{ component: 'EventDivider', screen: 'e3-1, e3-2', smoke: '/dev/smoke/e3-1' },
			{ component: 'FeedDayPromptCard', screen: 'лента круга' },
			{ component: 'FeedDayCard', screen: 'e3-20, e3-21, e3-22', smoke: '/dev/smoke/e3-20' },
			{ component: 'SettingsRow', screen: 'e6-2, e6-14', smoke: '/dev/smoke/e6-2' },
			{ component: 'MemberRow', screen: 'e6-5', smoke: '/dev/smoke/e6-5' },
			{ component: 'SearchGroupHeader', screen: 'e2-19', smoke: '/dev/smoke/e2-19' },
			{ component: 'SearchResultRow', screen: 'e2-19', smoke: '/dev/smoke/e2-19' },
			{ component: 'ServerRow', screen: 'e1-7, e1-2, e2-14, e2-15', smoke: '/dev/smoke/e2-14' },
			{ component: 'FoldHeader', screen: 'e4-17' },
			{ component: 'GroupFoldCard', screen: 'e2-13' },
			{ component: 'AttachmentRow', screen: 'e4-29, e4-6' },
			{ component: 'PhotoPlaceholder / PhotoGrid', screen: 'e4-2', smoke: '/dev/smoke/e4-2' },
			{ component: 'MediaTile', screen: 'e3-1, grid, album, headerMini, compose, selected' },
			{ component: 'MapBadge / MapPostSheet', screen: 'карта круга' },
			{ component: 'MonthLabel', screen: 'e5-1', smoke: '/dev/smoke/e5-1' },
			{ component: 'DayCard / DayGrid', screen: 'e5-1', smoke: '/dev/smoke/e5-1' },
			{ component: 'DayHeader', screen: 'e5-2' },
			{ component: 'EntryDateMark', screen: 'e5-4, e5-2' },
			{ component: 'ArchiveBanner', screen: 'e3-13' },
			{ component: 'PayStreetBanner', screen: 'e10-5' },
			{ component: 'MentionText', screen: 'e3-1, e4-17', smoke: '/dev/smoke/e3-1' },
			{ component: 'AttachmentList', screen: 'e4-29, e4-6, e4-7' },
			{ component: 'QrCode / InviteLinkCard', screen: 'e6-12, e6-10' },
			{ component: 'EmptyState', screen: 'e2-6, e3-1, e3-19' },
			{ component: 'PullRefresh', screen: 'e3-5' },
			{ component: 'FeedEnd', screen: 'e3-1' },
			{ component: 'ResponseEntry / PostRef', screen: 'e3-18' },
			{ component: 'PostByline', screen: 'e3-1, e4-5, e5-2' },
			{ component: 'PeekMemberList', screen: 'e1-4' },
			{ component: 'AboutFooter', screen: 'e7-1, e9-7' }
		]
	},
	{
		id: 'overlays',
		title: 'Overlays',
		entries: [
			{ component: 'Fab', screen: 'e2-1, e2-10', smoke: '/dev/smoke/e2-1' },
			{ component: 'CommentBar', screen: 'e3-1, e4-4', smoke: '/dev/smoke/e3-1' },
			{ component: 'Scrim / Sheet / Dialog', screen: 'e4-28', smoke: '/dev/smoke/e4-28' },
			{ component: 'Lightbox', screen: 'e4-30', smoke: '/dev/smoke/e4-30' },
			{ component: 'AvatarCrop', screen: 'e6-2' },
			{ component: 'PushBanner', screen: 'e7-35', smoke: '/dev/smoke/e7-35' },
			{ component: 'ConfirmDialog', screen: 'e6-2, e6-5' },
			{ component: 'ReactionsSheet', screen: 'e4-28', smoke: '/dev/smoke/e4-28' }
		]
	},
	{
		id: 'admin',
		title: 'Admin',
		entries: [
			{ component: 'AdminNav', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'AdminSection', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'DataTable', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'StackBar', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'QuotaRequestRow', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'CheckRow', screen: 'e9-11', smoke: '/dev/smoke/e9-11' },
			{ component: 'StatusIcon', screen: 'e9-11', smoke: '/dev/smoke/e9-11' },
			{ component: 'CodeBlock', screen: 'e9-12' },
			{ component: 'InlineInput', screen: 'e9-7', smoke: '/dev/smoke/e9-7' },
			{ component: 'AdminField / Panel', screen: 'e9-*' },
			{ component: 'SwitchRow', screen: 'e9-8, e9-4, e10-*', smoke: '/dev/smoke/e9-4' }
		]
	},
	{
		// Окно Мастерской (план 54): кадры docs/visual/installer.html. Живьём —
		// стенд окна cmd/wynd-installer/frontend/gallery.html (vite Мастерской),
		// не этот каталог: разделу нужен workshop.css и контейнер окна .ins.
		id: 'workshop',
		title: 'Workshop',
		entries: [
			{ component: 'TitleBar', screen: 'installer: все' },
			{ component: 'WorkshopTabs', screen: 'installer: 1, 11а' },
			{ component: 'SelfBar', screen: 'installer: 10а' },
			{ component: 'StepNav', screen: 'installer: 1–5, 7б–9г' },
			{ component: 'Notice', screen: 'installer: 2а, 5, 7а–9г' },
			{ component: 'MarkRows / MarkRow', screen: 'installer: 2, 3, 7а, 7б, 9б' },
			{ component: 'StepList', screen: 'installer: 4, 7в, 8б, 9в' },
			{ component: 'FootBar', screen: 'installer: все' },
			{ component: 'Checkbox / Radio', screen: 'installer: 1, 7б, 8а, 9б, 11в' },
			{ component: 'CopyLine', screen: 'installer: 2а, 9в' },
			{ component: 'FieldLine', screen: 'installer: 8а, 11б–11г' },
			{ component: 'CommandLog', screen: 'installer: 4а, 7д, 8г' },
			{ component: 'RawSheet', screen: 'installer: 2е' },
			{ component: 'FileRow', screen: 'installer: 11б–11г' }
		]
	},
	{
		id: 'layouts',
		title: 'Layouts',
		entries: [
			{ component: 'PlainLayout', screen: 'e1-1, e1-4', smoke: '/dev/smoke/e1-1' },
			{ component: 'ShellLayout', screen: 'e2-1', smoke: '/dev/smoke/e2-1' },
			{ component: 'CircleLayout', screen: 'e3-1', smoke: '/dev/smoke/e3-1' },
			{ component: 'FormLayout', screen: 'e2-14, e1-4', smoke: '/dev/smoke/e2-14' },
			{ component: 'OverlayLayout', screen: 'e4-28', smoke: '/dev/smoke/e4-28' },
			{ component: 'AdminWideLayout', screen: 'e9-7', smoke: '/dev/smoke/e9-7' }
		]
	}
];

export const SMOKE_ROUTES = [
	{ id: 'e1-1', label: 'Приглашение', href: '/dev/smoke/e1-1' },
	{ id: 'e1-3', label: 'Код из письма', href: '/dev/smoke/e1-3' },
	{ id: 'e1-6', label: 'Войти', href: '/dev/smoke/e1-6' },
	{ id: 'e1-2', label: 'Сервер соблюдает GDPR', href: '/dev/smoke/e1-2' },
	{ id: 'e2-1', label: 'Список кругов', href: '/dev/smoke/e2-1' },
	{ id: 'e2-3', label: 'Читает, не пишет', href: '/dev/smoke/e2-3' },
	{ id: 'e2-14', label: 'Новый круг', href: '/dev/smoke/e2-14' },
	{ id: 'e2-19', label: 'Поиск', href: '/dev/smoke/e2-19' },
	{ id: 'e3-1', label: 'Хронология', href: '/dev/smoke/e3-1' },
	{ id: 'e3-9', label: 'Круг, который читаете', href: '/dev/smoke/e3-9' },
	{ id: 'e3-20', label: 'Открытка дня', href: '/dev/smoke/e3-20' },
	{ id: 'e3-10', label: 'В круге кончилось место', href: '/dev/smoke/e3-10' },
	{ id: 'e3-11', label: 'На сервере кончилось место', href: '/dev/smoke/e3-10?server' },
	{ id: 'e4-21', label: 'Вложения в нити', href: '/dev/smoke/e4-21' },
	{ id: 'e4-2', label: 'Новая запись', href: '/dev/smoke/e4-2' },
	{ id: 'e4-2a', label: 'Новая запись: ролик сжимается', href: '/dev/smoke/e4-2a' },
	{ id: 'e4-30', label: 'Лайтбокс', href: '/dev/smoke/e4-30' },
	{ id: 'e4-28', label: 'Реакции', href: '/dev/smoke/e4-28' },
	{ id: 'e5-1', label: 'Дни', href: '/dev/smoke/e5-1' },
	{ id: 'e6-2', label: 'Настройки круга', href: '/dev/smoke/e6-2' },
	{ id: 'e6-5', label: 'Участники', href: '/dev/smoke/e6-5' },
	{ id: 'e6-17', label: 'Место кончилось', href: '/dev/smoke/e6-17' },
	{ id: 'e6-21', label: 'Сроки архивации', href: '/dev/smoke/e6-21' },
	{ id: 'e7-35', label: 'Пуши', href: '/dev/smoke/e7-35' },
	{ id: 'e7-14', label: 'Уйти с сервера насовсем', href: '/dev/smoke/e7-14' },
	{ id: 'e7-15', label: 'Уйти: вы владелец', href: '/dev/smoke/e7-15' },
	{ id: 'e7-16', label: 'Уйти: подтверждение', href: '/dev/smoke/e7-16' },
	{ id: 'e9-7', label: 'Хранилище', href: '/dev/smoke/e9-7' },
	{ id: 'e9-11', label: 'Проверка', href: '/dev/smoke/e9-11' },
	{ id: 'e9-4', label: 'Люди: уход с сервера', href: '/dev/smoke/e9-4' }
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
		rule: 'Mark только в AppBar, в Loading и 5 других мест (по правилам screens.html)',
		ok: true
	}
] as const;

export function countComponents(): number {
	return COMPONENT_MAP.filter((group) => group.id !== 'layouts').reduce(
		(sum, group) => sum + group.entries.length,
		0
	);
}

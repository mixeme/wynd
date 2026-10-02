export interface MediaSummary {
	blob_id: string;
	kind: 'photo' | 'video' | 'attachment';
	captured_at?: string;
	geo_lat?: number;
	geo_lng?: number;
	is_cover: boolean;
	filename?: string;
	size_bytes?: number;
	mime_type?: string;
	audio_artist?: string;
	audio_title?: string;
	audio_cover_blob_id?: string;
	/** Кадр обложки для ленты (4.16): квадрат в долях снимка. */
	crop?: { x: number; y: number; w: number; h: number };
}

export interface Comment {
	id: string;
	post_id: string;
	body: string;
	author_name: string;
	identity_id: string;
	author_avatar_blob_id?: string;
	created_at: string;
	edit_window_sec?: number | null;
	editable_until?: string;
}

export type ReactionKey = 'heart' | 'laugh' | 'surprise' | 'anger';

export const REACTION_KEYS: ReactionKey[] = ['heart', 'laugh', 'surprise', 'anger'];

export interface Reaction {
	id: string;
	post_id: string;
	emoji: string;
	author_name: string;
	identity_id: string;
	created_at: string;
	edit_window_sec?: number | null;
	editable_until?: string;
}

export interface FeedEvent {
	seq: number;
	summary: string;
	created_at: string;
}

export interface FeedPost {
	id: string;
	body: string;
	entry_date: string;
	author_name: string;
	identity_id: string;
	created_at: string;
	event_seq: number;
	author_avatar_blob_id?: string;
	captured_at?: string;
	edit_window_sec?: number | null;
	editable_until?: string;
	media?: MediaSummary[];
	comments?: Comment[];
	reactions?: Reaction[];
}

export interface FeedSnapshot {
	circle_id: string;
	posts: FeedPost[];
	events?: FeedEvent[];
	visible_from?: string | null;
	circle_started_at?: string;
}

export interface ArchiveCycle {
	active: true;
	cutoff_date: string;
	deadline: string;
	reminder_before_sec: number;
	cutoff_locked: boolean;
	personal_archive_bytes: number;
	personal_archive_media_count: number;
	personal_archive_post_count: number;
	download_url: string;
}

export interface CircleDetail {
	id: string;
	name: string;
	color?: string;
	status: string;
	archive_cycle?: ArchiveCycle;
	edit_window_sec?: number | null;
	is_owner?: boolean;
	can_settings?: boolean;
	identity_id?: string;
	identity_name?: string;
	avatar_blob_id?: string;
	/** «Место со снимков» — личная настройка в круге, одна на все устройства. */
	share_place?: boolean;
	/** Был ли в круге кто-то ещё: в круге из одного нет вкладки «Отклики». */
	has_others?: boolean;
	/** Новые отклики с прошлого просмотра (3.12). */
	responses_unread?: number;
}

export interface CompressionSettings {
	photo_max_px: number;
	photo_quality: number;
	video_max_height: number;
	video_bitrate_kbps: number;
	/** Битрейт сжатия звука (A6); старый сервер поля не шлёт. */
	audio_bitrate_kbps?: number;
	attachment_max_bytes: number;
}

export interface InstanceWithCompression {
	compression?: CompressionSettings;
}

export interface DaySummary {
	entry_date: string;
	post_count: number;
	title?: string;
	cover_post_id?: string;
	cover_blob_id?: string;
	/** Обложку дня не выбирали: медиа первой записи дня с фото, видео или звуком (C17). */
	fallback_cover_blob_id?: string;
	/** Фото за день — счётчик на карточке. */
	photo_count?: number;
	title_editable_until?: string | null;
	cover_editable_until?: string | null;
}

export interface DaysSnapshot {
	circle_id: string;
	days: DaySummary[];
}

export interface DaySnapshot {
	circle_id: string;
	entry_date: string;
	posts: FeedPost[];
}

export interface GridItem {
	post_id: string;
	blob_id: string;
	entry_date: string;
	created_at: string;
	is_cover: boolean;
}

export interface GridSnapshot {
	circle_id: string;
	items: GridItem[];
}

export interface MapPin {
	post_id: string;
	blob_id: string;
	entry_date: string;
	created_at: string;
	author_name: string;
	body: string;
	geo_lat: number;
	geo_lng: number;
}

export interface MapSnapshot {
	circle_id: string;
	pins: MapPin[];
}

export interface CircleSearchHit {
	post_id: string;
	comment_id?: string;
	/** Вложение, найденное по имени (kind file или audio). */
	media_id?: string;
	/** Блоб найденного вложения — по нему экран записи подсвечивает строку. */
	media_blob_id?: string;
	circle_id: string;
	author_name?: string;
	/** post, comment, day, file или audio. */
	kind: string;
	title?: string;
	snippet: string;
	thumb_blob_id?: string;
	entry_date: string;
	created_at: string;
}

export interface CircleSearchResponse {
	hits: CircleSearchHit[];
}

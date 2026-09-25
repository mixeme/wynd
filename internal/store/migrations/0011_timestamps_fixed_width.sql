-- 0011: выравнивание меток времени по фиксированной ширине.
--
-- До этой миграции метки писались через time.RFC3339Nano, который обрезает
-- хвостовые нули: рядом лежали «…T10:00:00Z», «…T10:00:00.25Z» и
-- «…T10:00:00.2501Z». SQLite сравнивает такие строки лексикографически, и
-- внутри одной секунды порядок получался неверным — а по меткам идут около
-- тридцати сравнений и столько же ORDER BY, включая отрезки видимости (TIME-1).
--
-- Новый формат — xtime.Layout: 2006-01-02T15:04:05.000000000Z, ровно 30
-- символов, всегда UTC, всегда девять знаков дроби.
--
-- Преобразование — чистый SQL, без потери точности:
--   длина 20 («…:05Z»)  → substr(c,1,19) || '.000000000Z'
--   с дробью            → substr(c,1,20) || дробь, дополненная нулями до 9, || 'Z'
-- Значения не в UTC («+03:00») и не-RFC3339 не трогаем: их в базе нет, а
-- гадать в миграции опаснее, чем оставить как есть.
--
-- Колонки собраны по схеме: каждая TEXT-колонка с именем на _at, которая
-- пишется через xtime.Format. Календарные даты (entry_date, cutoff_date) и
-- applied_at служебной таблицы миграций не трогаются.

-- Колонки (таблица.колонка):
--   accounts.created_at, accounts.last_login_at, accounts.deleted_at
--   accounts.subscription_expires_at, admin_credentials.reset_expires_at
--   admin_credentials.updated_at, blobs.created_at, circles.created_at
--   circles.updated_at, circles.cutoff_locked_at
--   circles.archive_reminder_sent_at, circles.archive_cycle_started_at
--   code_request_log.requested_at, comments.created_at, content_fts.created_at
--   day_covers.created_at, day_titles.created_at, events.created_at
--   identities.created_at, identity_names.effective_at
--   identity_names.erased_at, instance_settings.updated_at
--   instance_settings.smtp_test_sent_at, instance_settings.last_routine_at
--   instance_settings.last_backup_at, invites.expires_at, invites.revoked_at
--   invites.created_at, membership_spans.started_at, membership_spans.ended_at
--   memberships.created_at, memberships.updated_at, pay_requests.created_at
--   pay_requests.resolved_at, pending_circle_joins.created_at
--   pending_codes.expires_at, pending_codes.created_at, post_media.captured_at
--   posts.captured_at, posts.created_at, push_subscriptions.created_at
--   quota_requests.created_at, quota_requests.resolved_at
--   reactions.created_at, read_cursors.updated_at, sessions.expires_at
--   sessions.created_at, upload_sessions.expires_at
--   upload_sessions.created_at.

UPDATE accounts SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE accounts SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE accounts SET last_login_at = substr(last_login_at,1,19) || '.000000000Z' WHERE length(last_login_at) = 20 AND last_login_at LIKE '%Z';
UPDATE accounts SET last_login_at = substr(last_login_at,1,20) || substr(substr(last_login_at,21,length(last_login_at)-21) || '000000000',1,9) || 'Z' WHERE length(last_login_at) > 20 AND length(last_login_at) < 30 AND last_login_at LIKE '%Z' AND substr(last_login_at,20,1) = '.';
UPDATE accounts SET deleted_at = substr(deleted_at,1,19) || '.000000000Z' WHERE length(deleted_at) = 20 AND deleted_at LIKE '%Z';
UPDATE accounts SET deleted_at = substr(deleted_at,1,20) || substr(substr(deleted_at,21,length(deleted_at)-21) || '000000000',1,9) || 'Z' WHERE length(deleted_at) > 20 AND length(deleted_at) < 30 AND deleted_at LIKE '%Z' AND substr(deleted_at,20,1) = '.';
UPDATE accounts SET subscription_expires_at = substr(subscription_expires_at,1,19) || '.000000000Z' WHERE length(subscription_expires_at) = 20 AND subscription_expires_at LIKE '%Z';
UPDATE accounts SET subscription_expires_at = substr(subscription_expires_at,1,20) || substr(substr(subscription_expires_at,21,length(subscription_expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(subscription_expires_at) > 20 AND length(subscription_expires_at) < 30 AND subscription_expires_at LIKE '%Z' AND substr(subscription_expires_at,20,1) = '.';
UPDATE admin_credentials SET reset_expires_at = substr(reset_expires_at,1,19) || '.000000000Z' WHERE length(reset_expires_at) = 20 AND reset_expires_at LIKE '%Z';
UPDATE admin_credentials SET reset_expires_at = substr(reset_expires_at,1,20) || substr(substr(reset_expires_at,21,length(reset_expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(reset_expires_at) > 20 AND length(reset_expires_at) < 30 AND reset_expires_at LIKE '%Z' AND substr(reset_expires_at,20,1) = '.';
UPDATE admin_credentials SET updated_at = substr(updated_at,1,19) || '.000000000Z' WHERE length(updated_at) = 20 AND updated_at LIKE '%Z';
UPDATE admin_credentials SET updated_at = substr(updated_at,1,20) || substr(substr(updated_at,21,length(updated_at)-21) || '000000000',1,9) || 'Z' WHERE length(updated_at) > 20 AND length(updated_at) < 30 AND updated_at LIKE '%Z' AND substr(updated_at,20,1) = '.';
UPDATE blobs SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE blobs SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE circles SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE circles SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE circles SET updated_at = substr(updated_at,1,19) || '.000000000Z' WHERE length(updated_at) = 20 AND updated_at LIKE '%Z';
UPDATE circles SET updated_at = substr(updated_at,1,20) || substr(substr(updated_at,21,length(updated_at)-21) || '000000000',1,9) || 'Z' WHERE length(updated_at) > 20 AND length(updated_at) < 30 AND updated_at LIKE '%Z' AND substr(updated_at,20,1) = '.';
UPDATE circles SET cutoff_locked_at = substr(cutoff_locked_at,1,19) || '.000000000Z' WHERE length(cutoff_locked_at) = 20 AND cutoff_locked_at LIKE '%Z';
UPDATE circles SET cutoff_locked_at = substr(cutoff_locked_at,1,20) || substr(substr(cutoff_locked_at,21,length(cutoff_locked_at)-21) || '000000000',1,9) || 'Z' WHERE length(cutoff_locked_at) > 20 AND length(cutoff_locked_at) < 30 AND cutoff_locked_at LIKE '%Z' AND substr(cutoff_locked_at,20,1) = '.';
UPDATE circles SET archive_reminder_sent_at = substr(archive_reminder_sent_at,1,19) || '.000000000Z' WHERE length(archive_reminder_sent_at) = 20 AND archive_reminder_sent_at LIKE '%Z';
UPDATE circles SET archive_reminder_sent_at = substr(archive_reminder_sent_at,1,20) || substr(substr(archive_reminder_sent_at,21,length(archive_reminder_sent_at)-21) || '000000000',1,9) || 'Z' WHERE length(archive_reminder_sent_at) > 20 AND length(archive_reminder_sent_at) < 30 AND archive_reminder_sent_at LIKE '%Z' AND substr(archive_reminder_sent_at,20,1) = '.';
UPDATE circles SET archive_cycle_started_at = substr(archive_cycle_started_at,1,19) || '.000000000Z' WHERE length(archive_cycle_started_at) = 20 AND archive_cycle_started_at LIKE '%Z';
UPDATE circles SET archive_cycle_started_at = substr(archive_cycle_started_at,1,20) || substr(substr(archive_cycle_started_at,21,length(archive_cycle_started_at)-21) || '000000000',1,9) || 'Z' WHERE length(archive_cycle_started_at) > 20 AND length(archive_cycle_started_at) < 30 AND archive_cycle_started_at LIKE '%Z' AND substr(archive_cycle_started_at,20,1) = '.';
UPDATE code_request_log SET requested_at = substr(requested_at,1,19) || '.000000000Z' WHERE length(requested_at) = 20 AND requested_at LIKE '%Z';
UPDATE code_request_log SET requested_at = substr(requested_at,1,20) || substr(substr(requested_at,21,length(requested_at)-21) || '000000000',1,9) || 'Z' WHERE length(requested_at) > 20 AND length(requested_at) < 30 AND requested_at LIKE '%Z' AND substr(requested_at,20,1) = '.';
UPDATE comments SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE comments SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE content_fts SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE content_fts SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE day_covers SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE day_covers SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE day_titles SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE day_titles SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE events SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE events SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE identities SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE identities SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE identity_names SET effective_at = substr(effective_at,1,19) || '.000000000Z' WHERE length(effective_at) = 20 AND effective_at LIKE '%Z';
UPDATE identity_names SET effective_at = substr(effective_at,1,20) || substr(substr(effective_at,21,length(effective_at)-21) || '000000000',1,9) || 'Z' WHERE length(effective_at) > 20 AND length(effective_at) < 30 AND effective_at LIKE '%Z' AND substr(effective_at,20,1) = '.';
UPDATE identity_names SET erased_at = substr(erased_at,1,19) || '.000000000Z' WHERE length(erased_at) = 20 AND erased_at LIKE '%Z';
UPDATE identity_names SET erased_at = substr(erased_at,1,20) || substr(substr(erased_at,21,length(erased_at)-21) || '000000000',1,9) || 'Z' WHERE length(erased_at) > 20 AND length(erased_at) < 30 AND erased_at LIKE '%Z' AND substr(erased_at,20,1) = '.';
UPDATE instance_settings SET updated_at = substr(updated_at,1,19) || '.000000000Z' WHERE length(updated_at) = 20 AND updated_at LIKE '%Z';
UPDATE instance_settings SET updated_at = substr(updated_at,1,20) || substr(substr(updated_at,21,length(updated_at)-21) || '000000000',1,9) || 'Z' WHERE length(updated_at) > 20 AND length(updated_at) < 30 AND updated_at LIKE '%Z' AND substr(updated_at,20,1) = '.';
UPDATE instance_settings SET smtp_test_sent_at = substr(smtp_test_sent_at,1,19) || '.000000000Z' WHERE length(smtp_test_sent_at) = 20 AND smtp_test_sent_at LIKE '%Z';
UPDATE instance_settings SET smtp_test_sent_at = substr(smtp_test_sent_at,1,20) || substr(substr(smtp_test_sent_at,21,length(smtp_test_sent_at)-21) || '000000000',1,9) || 'Z' WHERE length(smtp_test_sent_at) > 20 AND length(smtp_test_sent_at) < 30 AND smtp_test_sent_at LIKE '%Z' AND substr(smtp_test_sent_at,20,1) = '.';
UPDATE instance_settings SET last_routine_at = substr(last_routine_at,1,19) || '.000000000Z' WHERE length(last_routine_at) = 20 AND last_routine_at LIKE '%Z';
UPDATE instance_settings SET last_routine_at = substr(last_routine_at,1,20) || substr(substr(last_routine_at,21,length(last_routine_at)-21) || '000000000',1,9) || 'Z' WHERE length(last_routine_at) > 20 AND length(last_routine_at) < 30 AND last_routine_at LIKE '%Z' AND substr(last_routine_at,20,1) = '.';
UPDATE instance_settings SET last_backup_at = substr(last_backup_at,1,19) || '.000000000Z' WHERE length(last_backup_at) = 20 AND last_backup_at LIKE '%Z';
UPDATE instance_settings SET last_backup_at = substr(last_backup_at,1,20) || substr(substr(last_backup_at,21,length(last_backup_at)-21) || '000000000',1,9) || 'Z' WHERE length(last_backup_at) > 20 AND length(last_backup_at) < 30 AND last_backup_at LIKE '%Z' AND substr(last_backup_at,20,1) = '.';
UPDATE invites SET expires_at = substr(expires_at,1,19) || '.000000000Z' WHERE length(expires_at) = 20 AND expires_at LIKE '%Z';
UPDATE invites SET expires_at = substr(expires_at,1,20) || substr(substr(expires_at,21,length(expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(expires_at) > 20 AND length(expires_at) < 30 AND expires_at LIKE '%Z' AND substr(expires_at,20,1) = '.';
UPDATE invites SET revoked_at = substr(revoked_at,1,19) || '.000000000Z' WHERE length(revoked_at) = 20 AND revoked_at LIKE '%Z';
UPDATE invites SET revoked_at = substr(revoked_at,1,20) || substr(substr(revoked_at,21,length(revoked_at)-21) || '000000000',1,9) || 'Z' WHERE length(revoked_at) > 20 AND length(revoked_at) < 30 AND revoked_at LIKE '%Z' AND substr(revoked_at,20,1) = '.';
UPDATE invites SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE invites SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE membership_spans SET started_at = substr(started_at,1,19) || '.000000000Z' WHERE length(started_at) = 20 AND started_at LIKE '%Z';
UPDATE membership_spans SET started_at = substr(started_at,1,20) || substr(substr(started_at,21,length(started_at)-21) || '000000000',1,9) || 'Z' WHERE length(started_at) > 20 AND length(started_at) < 30 AND started_at LIKE '%Z' AND substr(started_at,20,1) = '.';
UPDATE membership_spans SET ended_at = substr(ended_at,1,19) || '.000000000Z' WHERE length(ended_at) = 20 AND ended_at LIKE '%Z';
UPDATE membership_spans SET ended_at = substr(ended_at,1,20) || substr(substr(ended_at,21,length(ended_at)-21) || '000000000',1,9) || 'Z' WHERE length(ended_at) > 20 AND length(ended_at) < 30 AND ended_at LIKE '%Z' AND substr(ended_at,20,1) = '.';
UPDATE memberships SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE memberships SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE memberships SET updated_at = substr(updated_at,1,19) || '.000000000Z' WHERE length(updated_at) = 20 AND updated_at LIKE '%Z';
UPDATE memberships SET updated_at = substr(updated_at,1,20) || substr(substr(updated_at,21,length(updated_at)-21) || '000000000',1,9) || 'Z' WHERE length(updated_at) > 20 AND length(updated_at) < 30 AND updated_at LIKE '%Z' AND substr(updated_at,20,1) = '.';
UPDATE pay_requests SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE pay_requests SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE pay_requests SET resolved_at = substr(resolved_at,1,19) || '.000000000Z' WHERE length(resolved_at) = 20 AND resolved_at LIKE '%Z';
UPDATE pay_requests SET resolved_at = substr(resolved_at,1,20) || substr(substr(resolved_at,21,length(resolved_at)-21) || '000000000',1,9) || 'Z' WHERE length(resolved_at) > 20 AND length(resolved_at) < 30 AND resolved_at LIKE '%Z' AND substr(resolved_at,20,1) = '.';
UPDATE pending_circle_joins SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE pending_circle_joins SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE pending_codes SET expires_at = substr(expires_at,1,19) || '.000000000Z' WHERE length(expires_at) = 20 AND expires_at LIKE '%Z';
UPDATE pending_codes SET expires_at = substr(expires_at,1,20) || substr(substr(expires_at,21,length(expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(expires_at) > 20 AND length(expires_at) < 30 AND expires_at LIKE '%Z' AND substr(expires_at,20,1) = '.';
UPDATE pending_codes SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE pending_codes SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE post_media SET captured_at = substr(captured_at,1,19) || '.000000000Z' WHERE length(captured_at) = 20 AND captured_at LIKE '%Z';
UPDATE post_media SET captured_at = substr(captured_at,1,20) || substr(substr(captured_at,21,length(captured_at)-21) || '000000000',1,9) || 'Z' WHERE length(captured_at) > 20 AND length(captured_at) < 30 AND captured_at LIKE '%Z' AND substr(captured_at,20,1) = '.';
UPDATE posts SET captured_at = substr(captured_at,1,19) || '.000000000Z' WHERE length(captured_at) = 20 AND captured_at LIKE '%Z';
UPDATE posts SET captured_at = substr(captured_at,1,20) || substr(substr(captured_at,21,length(captured_at)-21) || '000000000',1,9) || 'Z' WHERE length(captured_at) > 20 AND length(captured_at) < 30 AND captured_at LIKE '%Z' AND substr(captured_at,20,1) = '.';
UPDATE posts SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE posts SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE push_subscriptions SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE push_subscriptions SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE quota_requests SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE quota_requests SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE quota_requests SET resolved_at = substr(resolved_at,1,19) || '.000000000Z' WHERE length(resolved_at) = 20 AND resolved_at LIKE '%Z';
UPDATE quota_requests SET resolved_at = substr(resolved_at,1,20) || substr(substr(resolved_at,21,length(resolved_at)-21) || '000000000',1,9) || 'Z' WHERE length(resolved_at) > 20 AND length(resolved_at) < 30 AND resolved_at LIKE '%Z' AND substr(resolved_at,20,1) = '.';
UPDATE reactions SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE reactions SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE read_cursors SET updated_at = substr(updated_at,1,19) || '.000000000Z' WHERE length(updated_at) = 20 AND updated_at LIKE '%Z';
UPDATE read_cursors SET updated_at = substr(updated_at,1,20) || substr(substr(updated_at,21,length(updated_at)-21) || '000000000',1,9) || 'Z' WHERE length(updated_at) > 20 AND length(updated_at) < 30 AND updated_at LIKE '%Z' AND substr(updated_at,20,1) = '.';
UPDATE sessions SET expires_at = substr(expires_at,1,19) || '.000000000Z' WHERE length(expires_at) = 20 AND expires_at LIKE '%Z';
UPDATE sessions SET expires_at = substr(expires_at,1,20) || substr(substr(expires_at,21,length(expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(expires_at) > 20 AND length(expires_at) < 30 AND expires_at LIKE '%Z' AND substr(expires_at,20,1) = '.';
UPDATE sessions SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE sessions SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';
UPDATE upload_sessions SET expires_at = substr(expires_at,1,19) || '.000000000Z' WHERE length(expires_at) = 20 AND expires_at LIKE '%Z';
UPDATE upload_sessions SET expires_at = substr(expires_at,1,20) || substr(substr(expires_at,21,length(expires_at)-21) || '000000000',1,9) || 'Z' WHERE length(expires_at) > 20 AND length(expires_at) < 30 AND expires_at LIKE '%Z' AND substr(expires_at,20,1) = '.';
UPDATE upload_sessions SET created_at = substr(created_at,1,19) || '.000000000Z' WHERE length(created_at) = 20 AND created_at LIKE '%Z';
UPDATE upload_sessions SET created_at = substr(created_at,1,20) || substr(substr(created_at,21,length(created_at)-21) || '000000000',1,9) || 'Z' WHERE length(created_at) > 20 AND length(created_at) < 30 AND created_at LIKE '%Z' AND substr(created_at,20,1) = '.';

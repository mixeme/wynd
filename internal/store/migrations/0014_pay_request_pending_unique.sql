-- 0014: одна ожидающая заявка на оплату на учётку.
--
-- Аудит 2026-09-22: проверка «есть ли pending» и INSERT шли вне одной
-- транзакции, два параллельных POST /pay/requests давали две заявки, и админ
-- мог утвердить обе — одно продление за один платёж дважды. Код теперь делает
-- проверку внутри транзакции, а схема закрепляет правило частичным уникальным
-- индексом.
--
-- Если дубли уже есть, младшие помечаются отклонёнными, чтобы индекс собрался;
-- файл скриншота у них остаётся (blob_deleted = 0) и уйдёт обычной чисткой.

UPDATE pay_requests
SET status = 'rejected',
    resolved_at = strftime('%Y-%m-%dT%H:%M:%f', 'now') || '000000Z'
WHERE status = 'pending'
  AND EXISTS (
    SELECT 1 FROM pay_requests older
    WHERE older.account_id = pay_requests.account_id
      AND older.status = 'pending'
      AND (older.created_at < pay_requests.created_at
           OR (older.created_at = pay_requests.created_at AND older.id < pay_requests.id))
  );

CREATE UNIQUE INDEX idx_pay_requests_one_pending
    ON pay_requests (account_id) WHERE status = 'pending';

-- 0015: индекс на (email, requested_at) в code_request_log.
--
-- Аудит 2026-09-22 (SEC-8): лимитер кодов считает запросы двумя подзапросами
-- — по client_ip и по email. Индекс был только на (client_ip, requested_at),
-- поэтому счёт по почте шёл полным перебором журнала на каждом /auth/code.

CREATE INDEX IF NOT EXISTS idx_code_request_log_email_time
    ON code_request_log (email, requested_at);

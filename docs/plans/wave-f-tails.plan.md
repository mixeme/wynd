# Хвосты волны F

Панель по макету закрыта в 0.1.34; развилки — в [server-reference.md](../reference/server-reference.md) и [client-reference.md](../reference/client-reference.md). Здесь только то, что при приёмке осталось.

**Эталон:** [screens.html](../visual/screens.html) `#e9-1`–`#e9-10`.  
**Не трогать:** `docs/visual/screens.html`, `docs/wynd.html`, закрытые развилки F.

Инструкция: если в пункте не сказано «выбери» — выбирать нельзя. Thinking-модель не нужна. Один пункт = один коммит, если не сказано иначе. Приёмка: браузер по затронутым экранам + `go test ./...` + `npm run check`.

---

## Не делать (закрыто в F)

- Менять или удалять `POST /admin/quota_requests/{id}/approve` (`+=`, без `quota_custom`)
- Событие квоты в хронике; лица в GET учётки; SQL `DELETE FROM accounts` из карточки людей
- GDPR-экран; отсечку из панели; bootstrap/Caddy; новые проверки на 9.5
- Новые `.svelte` в `$ui` и `layouts/`
- Переоткрывать таблицу развилок F

---

## Очередь

- [ ] `DeleteAccount`: Leave и soft-delete в **одной** транзакции. Сейчас `Chronicle.Leave` коммитит круг, потом отдельный `BeginTx` на identities/email/сессии (`accounts.go`). Срыв второго шага оставляет членства `gone`, учётка жива.
- [ ] `cleanEmptyAccounts` (`jobs/routine.go`) всё ещё `DELETE FROM accounts` — каскад блобов. Перевести на soft-delete или не трогать строки с блобами.
- [ ] `ListAccounts.circle_count` считает все memberships, включая `gone`; карточка — только `active`. После добровольного ухода список и 9.9 расходятся.
- [ ] Тесты API, которых не было в приёмке F: `PUT /admin/storage/default_quota` не трогает круги; `PUT /admin/circles/{id}/quota` (custom false/true/null, pending → approved абсолют); `ttl_sec` на `POST /admin/invites`; sentinel DELETE → 404; повторная регистрация той же почты после soft-delete; identities `account_id` NULL и посты на месте.
- [ ] Макет 9.7: QR в правой колонке + подпись «та же ссылка кодом». Сейчас по развилке F — под ссылкой, в `.qr`.
- [x] Макет 9.8: поле `.sfield` «почта». Поле фильтра = поиск — `SearchField`, не `Input admin`.
- [ ] Макет 9.9: в строке круга «участник · с {дата}». Сейчас только роль. GET лиц и дат вступления не отдаёт — не выдумывать поля.

**Файлы:** `internal/auth/accounts.go`, `internal/chronicle/member.go`, `internal/jobs/routine.go`, `internal/auth/admin_access.go`, `internal/api/admin_wave_f_test.go`, `web/src/routes/admin/access/+page.svelte`, `web/src/routes/admin/people/`

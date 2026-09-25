# Сопровождение после рецензии

**Тип:** рецензия и план сопровождения  
**Статус:** открыт  
**Дата:** 2026-09-07  
**Срез:** после закрытия `code-review-tails.plan.md` (`ListAccountCircles` batch).  
**Эталоны не трогать:** `docs/visual/screens.html`, `docs/wynd.html`.

Инструкция исполнителю: **развилки ниже закрыты**. Не предлагать PostgreSQL, E2E, федерацию, группы кругов, пагинацию ленты, донаты в 0.x, SPDX в каждом файле, покрытие тестами ради процента. Один пункт = один PR, если не сказано иначе. Приёмка: затронутые экраны в браузере (если GUI) + `go test ./...` + `cd web && npm run check && npm run check:ui && npm run test`.

Решения волн 0–5 и batch списка кругов — в справочниках: [server-reference.md](../reference/server-reference.md), [client-reference.md](../reference/client-reference.md), [ui-components.md](../reference/ui-components.md).

---

## Закрытые развилки

Не переоткрывать.

| Тема | Решение |
|------|---------|
| PostgreSQL / `store.Store` | Драйвер один — SQLite. Интерфейс — Open/Close/Ping/Version. |
| E2E, федерация, группы кругов, TWA, UnifiedPush, камера, QR-сканер, GDPR-экран | Вне scope. |
| Пожертвования | Спека «не в 0.x». Код не писать. |
| Пагинация ленты | Не делать. Потолок снимка 2000 (`SnapshotPostLimit`). |
| SSE | Опрос раз в 2 с. Не вводить брокер. |
| `ttl_days` | Алиас до 1.0. Не удалять. |
| `0001_init.sql` | Оставить пустым. Перенумеровывать миграции запрещено. |
| Bits UI | Только внутри `$ui`. На экранах — запрет. |
| PhoneFrame | Корень layout'а в бою. StatusBar — `/dev` и `app={false}`. |
| SettingsRow + Switch | Слот `control` уже в библиотеке. Notify/app уже собраны. Новый `$ui`-файл не заводить. |
| `UNIQUE` + NULL identities | Не чинить частичным индексом: несколько отвязанных лиц на круг после soft-delete — следствие модели. |
| Макеты | `screens.html` / `wynd.html` не синхронизировать с номером версии. |
| CI-облако, SPDX-шапки | Не в этой очереди. |
| `ListAccountCircles` N+1 | Закрыто: три batch-запроса, не цикл на круг. |

---

## Очередь

### Сервер

- [ ] `notifyCircle`: ошибка членства/prefs/`SendSignal` в лог; не `_ =`. `http.Server.Shutdown` — 10 с, таймаут горутины — 15 с: процесс уходит раньше. Привязать к shutdown (WaitGroup / общий ctx) или не превышать срок Shutdown.
- [ ] `mail.ErrNotConfigured` / `push.ErrNotConfigured` не отдавать как 500 `"internal"`. SMTP-test уже мапит почту; `writeDomainError` и push-test — нет.
- [ ] `writeJSON`: ошибка encode не глотать; 500 домена — в лог.
- [ ] `sanitizeFilename` режет по байтам (`name[:255]`), не по рунам.

### GUI (без новых `$ui`-файлов, пока хватает библиотеки)

- [ ] Circle denied в `circles/[id]/+layout.svelte` — через layout, не сырой `<div class="ph app">`.
- [ ] CommentBar (уже `$ui`): `class="inp fld"` и `class:off` на `.send` — в рамки библиотеки / токенов, без page-CSS, который отменяет `.fld`.
- [ ] Сырые `.row2` вне notify/app: архив, identity, servers, deadlines, лист реакций ленты. Собирать из `SettingsRow` (и `control`, если справа контрол). Не добавлять продуктовые строки из макета. `/dev` не трогать ради этого.
- [ ] Compose: сырой `textarea.compose-text`; иконка места без обработчика; в правке фото/файл — не кнопки. Сначала `TextArea`; новый variant/`$ui`-файл — только план пробела Wynd UI, затем стоп.
- [ ] Админ 9.1 в бою — `DataTable` / системные ряды, не инлайн-`<table class="tbl">`.
- [ ] Лента: второй клик-слой поверх OverlayLayout+Scrim; магические 69px / `#d6cec2` — токены.
- [ ] Каталог: smoke `TextButton` → `/dev/smoke/e4-1`, маршрута нет; AvatarCrop на `/dev/ui`.

### Сторож

- [ ] `check:ui` не ловит сырой `.act` / `.row2` / `.compose-text` на экранах. Расширить guard духа правила, не буквы `btn`/`lab`/`fld`.

### Вне очереди (не делать из этого плана)

Пагинация API ленты, GDPR HTTP, донаты, Postgres, E2E, Playwright, SPDX, удаление `ttl_days`, перенумерация миграций, новые npm UI-пакеты, правка `screens.html`.

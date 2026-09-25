# Сторож сырого row2 и compose-text

**Тип:** сторож UI  
**Статус:** открыт  
**Дата:** 2026-09-10  
**Срез:** после закрытия `code-review-followup`.

Экраны из очереди followup уже без сырого `div.row2` и `textarea.compose-text` (`SettingsRow`, `TextArea`). `check:ui` это не закрепляет: ловит только сырой `<button class="row2|one|cm|…">` на prod-маршрутах.

Инструкция исполнителю: не предлагать новые `$ui`-файлы, не трогать `/dev` ради этого, не расширять сторож на Bits UI. Один PR. Приёмка: `cd web && npm run check:ui && npm run test`.

---

## Очередь

- [ ] `ui-guard.mjs`: на prod-маршрутах (не `/dev`) запретить сырой `<div class="row2">` и класс `compose-text` вне `$ui`.
- [ ] Тест в `ui-guard.test.ts`.
- [ ] [ui-components.md](../reference/ui-components.md) — строка про сторож; этот план закрыть и удалить.

---

## Вне очереди

Новые `$ui`-файлы, `/dev`, Bits UI, правка `screens.html`.

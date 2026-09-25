# Лента — реакции и превью комментария (пробел Wynd UI)

**Тип:** пробел Wynd UI  
**Статус:** закрыт  
**Дата:** 2026-09-09  
**Экран / макет:** `/circles/[id]` (#e3-1), `/circles/[id]/posts/[postId]` (#e4-2) · [screens.html](../visual/screens.html)

`npm run check:ui` падает на сырой `<button class="one|add|rcho|cm">` и `<div class="row2">` в оверлее списка реакций. Исключений в `ui-guard.mjs` нет — закрытие плана = зелёный guard.

---

## Почему нельзя собрать из имеющихся

| Смотрели | Почему не хватает |
|----------|-------------------|
| `PostCard` + snippet `reactions` | Карточка только слот; разметка `.rx` / `.rxpick` / `button.one` / `button.add` / `button.rcho` остаётся на маршруте. |
| `Chip` / `Button` | Другой вид (прямоугольные чипы настроек, полноширинные `.btn`), не чипы реакций из макета. |
| `IconButton` | Только иконка, без подписи «Кот, Петя» и без ряда `.rx`. |
| `SettingsRow` / `MemberRow` | Строка настроек/участника; не собирает группу реакций и пикер. `MemberRow` не даёт слот «иконка реакции справа» для оверлея списка. |

Расширить `PostCard` реакциями внутри — нельзя: в макете превью комментария (`.cm`) **сосед** карточки, не внутри `.post` (см. [client-reference.md](../reference/client-reference.md)).

---

## Добавить в библиотеку

| Компонент | Путь | Зачем |
|-----------|------|-------|
| `ReactionBar` | `$ui/data/ReactionBar.svelte` | Блок `.rx`: сгруппированные `button.one`, `button.add`, при открытии — `.rxpick` с `button.rcho`. Props: группы реакций, `pickerOpen`, `ownEmoji`, `onopenList`, `onadd`, `onpick`. |
| `CommentPreview` | `$ui/data/CommentPreview.svelte` | Корень `button.cm`, внутри snippet `children`. Prop `onclick` → обсуждение. |
| `ReactionListRow` | `$ui/data/ReactionListRow.svelte` | Строка оверлея «кто отреагировал»: `button.row2` или `div.row2` + `Avatar` + имя + `Icon` справа (как сейчас в ленте/обсуждении). |

Иконки — через существующий `$ui/Icon.svelte` и `reactionIconName` из `$lib` (в компонент передавать уже resolved `IconName`, не тянуть journal в `$ui`).

---

## Задачи

### 1. Библиотека (эта задача)

- [x] `ReactionBar.svelte` — разметка и классы как в `screens.html` #e3-1; вложенные кнопки со `stopPropagation` не нужны (клик ловит `PostCard.onRootClick` через `.rxpick` в `closest`).
- [x] `CommentPreview.svelte` — только обёртка `.cm`, контент с маршрута snippet'ом.
- [x] `ReactionListRow.svelte` — заменить сырой `div.row2` в оверлеях.
- [x] Vitest: mount + `click` на `ReactionBar` / `CommentPreview` (как `SettingsRow.test.ts`).
- [x] `web/src/routes/dev/ui/catalog.ts` + фигура в `/dev/ui` (можно рядом с `PostCard` #e3-1).
- [x] [ui-components.md](../reference/ui-components.md) — таблица Data.

### 2. Маршруты (следующая задача, после закрытия п.1)

- [x] `circles/[id]/+page.svelte` — snippet `reactions` → `<ReactionBar …>`; превью комментария → `<CommentPreview>`; оверлей списка → `{#each …}<ReactionListRow …>{/each}`.
- [x] `circles/[id]/posts/[postId]/+page.svelte` — то же для `reactions` и оверлея.
- [x] Логика (`groupReactions`, `pickReaction`, `togglePicker`, URL `?reactions=`) остаётся на маршруте; в `$ui` только разметка и `onclick`-колбэки.
- [x] `npm run check:ui` — без срабатываний `raw semantic <button class=…>` на этих маршрутах.

### 3. Связанное

Пункт «сырые `.row2` … лист реакций ленты» закрыт `ReactionListRow` в п.2.

---

## Приёмка

`cd web && npm run check && npm run check:ui && npm run test`  
В браузере: лента и обсуждение — чипы реакций, «+», пикер, клик в список реакций, превью комментария под карточкой, оверлей списка.

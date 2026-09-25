# Wynd UI — справочник

**Wynd UI** — design system Wynd: компоненты (`$ui/...` → `web/src/lib/components/`, без barrel `$ui/index.ts`), layouts (`$lib/layouts/`, без алиаса), CSS-токены (`tokens.css`, `ui.css`). Dev-каталог: `/dev/ui`. Guard: `npm run check:ui` (`web/scripts/check-ui.mjs` + `ui-guard.mjs`), не ESLint. Сторож агента: `.cursor/hooks/ui-screens.mjs`, правило `.cursor/rules/wynd-ui-screens.mdc`.

Сжатая выжимка из закрытого плана. Макеты: [screens.html](../visual/screens.html). Стек и спайк Bits UI vs shadcn: [stack.html](../stack.html).  
Таблица «компонент → экран» в `web/src/routes/dev/ui/catalog.ts`.

**Выбор:** ветка **Б** — Bits UI + свой CSS на токенах (`ui.css`, классы `.cbar`, `.post`, `.r`).

**Правила:** shell без цвета круга; accent через `--c` / `--ct`; Danger — ink border; Mark только в AppBar и пять других мест по макету. Новые npm-пакеты для UI не ставить. [screens.html](../visual/screens.html) и [wynd.html](../wynd.html) не синхронизировать с `VERSION`.

**Пробел библиотеки:** экран — только существующие `$ui` и `$lib/layouts`. Не хватает куска — сначала расширить уже лежащий компонент. Если объективно нельзя, это **отдельная задача** на Wynd UI: `docs/plans/<slug>.plan.md`, затем стоп. В той же задаче `.svelte` в `$ui` не заводить и дыру на экране не верстать. Сторож пропустит новый файл позже, только если открытый план его перечисляет в таблице. `/dev/spike` в проверку состава не входит.

```markdown
# … — пробел Wynd UI

**Тип:** пробел Wynd UI
**Статус:** открыт
**Экран / макет:** /circles/[id]/… · screens.html #e…

## Почему нельзя собрать из имеющихся

Какие `$ui` смотрели. Почему не расширить уже лежащий (какой).

## Добавить в библиотеку

| Компонент | Путь | Зачем |
|-----------|------|-------|
| Foo | `$ui/forms/Foo.svelte` | … |

- [ ] Библиотека по таблице, catalog.ts, /dev/ui, этот справочник
- [ ] Экран — следующая задача, не эта
```

**Интерактив:** корень кнопки — `<button type="button">`, не `div`/`span` + `role="button"`. `Button.onclick` обязателен; без действия в dev/smoke — `onclick={() => {}}`. Загрузка — prop `loading` (текст `…`, вид `.off`); не `class:off` на экранах. `Chip` без `onclick` — `<span>`, с действием — `<button aria-pressed>`. Навигационные строки (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow` в режиме transfer) несут `onclick` на корне. `CircleRow` в режиме `card` — `div`-карточка: действие и чипы групп под строкой, без вложенных `button`. Ссылки с URL остаются `<a class="under" href>`. Identity в `CircleBar` — `<button type="button" class="idn">` (без `circleId` — `span.idn`), не `IconButton`. Compose: `TextButton` `bar` / `barAction`. Вложенные `<button>` запрещены: меню в `MemberRow` только если строка не кликабельна целиком. `AdminNav` при `links` — `<button type="button">`, не `span` + `role="button"`. Карточки ленты и дней: опциональный `onclick` на корневом `div` (`PostCard` / `DayCard`), без обёртки; карточку не делать `<button>`. `PostCard.onRootClick` игнорирует `button, a, input, textarea, select, label, .rxpick` — чипам реакций `stopPropagation` не нужен; альбом и прочие не-кнопки по-прежнему останавливают всплытие сами.

**Формы:** ввод — `Input` / `TextArea` / `SearchField`; статика — `FieldDisplay` (бывший `Field`). Админка: `Input admin={true}` и `FieldDisplay admin={true}` (класс `.inp`), не отдельный `AdminInput`. `SearchField` — редактируемый поиск и поля фильтров (тот же виджет: `/search`, поиск в круге, 9.8 почта); `BackBar` свой `.sfield`. `TextArea variant`: `area` \| `field` \| `compose` \| `comment`.

---

## Layout-шаблоны

`web/src/lib/layouts/`

| Layout | CSS | Примеры экранов |
|--------|-----|-----------------|
| PlainLayout | `.ph` | e1-1, e1-3 |
| ShellLayout | `.ph.shell` + при `app` `.shell-body` | e2-1, e2-2, e7-* |
| CircleLayout | `.ph.{color}` | e3-*, e4-*, e5-*, e6-* |
| FormLayout | `.ph.{color\|shell}` | e1-*, e2-3, e2-7, e6-1 |
| OverlayLayout | absolute | sheets, dialogs, push; `ondismiss` → Scrim |
| AdminWideLayout | `.ph.wide.shell` | e9-* |

---

## Компоненты по папкам

### `chrome/`

PhoneFrame, StatusBar, AppBar, CircleBar (4 таба), BackBar, AdminBar

### `forms/`

**Label**, **Input**, **FieldDisplay**, **TextArea**, ScreenTitle, Hint, **Button**, **Chip**, ChipGroup, Switch, ColorSwatches, CodeBox, InviteCard, **SearchField**, DangerZone (опц. `style`), Meter, PeopleStrip, AddPhotoButton, DangerNote, VolumeChart, **IconButton**, **TextButton**

Интерактивные примитивы (фаза 1–4):

| Компонент | Корень | Обязательные props | Поведение |
|-----------|--------|-------------------|-----------|
| `Button` | `<button class="btn">` | `onclick` | `variant`, `disabled`/`loading` → класс `.off`, текст `…` |
| `Chip` | `<button class="chip">` или `<span>` | — | с `onclick` — кнопка, `aria-pressed={selected}` |
| `IconButton` | `<button class="ib">` | `name`, `label`, `onclick` | рендер только при `onclick` |
| `TextButton` | `<button>` | `onclick` | `variant`: `link` (`.under`), `admin` (`.act` в `.chk`), `adminBox` (`.inp`), `bar` (`.t`), `barAction` (`.rt`/`.rt.on`) |

Формы (фаза 5):

| Компонент | Корень | Поведение |
|-----------|--------|-----------|
| `Label` | `<div class="lab">` | подпись поля |
| `Input` | `<input class="fld">` или `.inp` | `admin`, `active`, `gray`, `bind:value` |
| `FieldDisplay` | `<div class="fld">` или `.inp` | статика (бывший `Field`); `admin` → `.inp` (9.7 URL инвайта) |
| `TextArea` | `<textarea class="ta">`, `.fld`, `.compose-text` или `.inp` | `variant`: `area` \| `field` \| `compose` \| `comment`, `bind:value`; `compose` — зеркало + `.men` для `@имя` |
| `SearchField` | `.sfield` + `<input type="search">` | иконка, `bind:value`; поиск и фильтры |
| `VolumeChart` | `.chart` + SVG | `volume`, `cutoffLabel`, `bind:cutoffX`, `oncutoff(index)`; жест только при `oncutoff` |

Guard: `npm run check:ui` — экран = существующие `$ui` + `$lib/layouts` (не Bits UI, не одноразовый `.svelte` у маршрута, в `routes/` только `+page`/`+layout`/`+error`); новый файл в библиотеке — только по открытому плану «пробел Wynd UI»; в `web/src` запрещён импорт `$lib/components` (использовать `$ui`); в `routes/` запрещены `role="button"`, сырой `class="btn"`, сырой `class="lab"`, `<input class="fld">`, `<textarea class="fld|ta">`, сырой `<button class="row2|one|cm|…">`, сырой `<div class="row2">` и сырой `class="compose-text"` (prod-маршруты, не `/dev`).

**Интерактив на `<button>` (Wynd UI, фазы 1–3):** в `ui.css` селектор `button.X` сильнее `.X`. У layout-классов (`btn`, `row2`, `chip`, `cm`, …) вёрстку из `.X` дублируют в `button.X` или `.контекст button.X`. Нативный `<button>` не растягивается как `div`: `button.btn` — `width: calc(100% - 32px)` (поля `.btn` 16+16, как `input.fld`); в `.rowin` / `.chk` — `width:auto`. Текстовые (`act`, `t`, `rt`, `under`) — padding:0 намеренно. Список и проверка: `BUTTON_LAYOUT_SPECS` / `BUTTON_TEXT_CLASSES` в `web/scripts/ui-guard.mjs`; `npm run check:ui` падает при рассинхроне.

### `data/`

SectionLabel, Avatar, EventDivider, CircleRow, PostCard, **ReactionBar**, **CommentPreview**, **ReactionListRow**, **SettingsRow**, MemberRow, SearchGroupHeader, **SearchResultRow**, **ServerRow**, FoldHeader, AttachmentRow, PhotoPlaceholder, PhotoGrid, MonthLabel, DayCard, DayGrid, DayHeader, EntryDateMark, ArchiveBanner

Строки с опциональным `onclick`: корень `button.row2` / `button.r` или `div` (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow`). `SettingsRow` с snippet `control` — всегда `div.row2`, справа контрол (например `Switch`); `chevron`/`value` не рендерятся; title без `font-weight:600`. `PostCard` / `DayCard` — `div` с опциональным `onclick`, не `<button>`. `FoldHeader` — `button.fold` при `onclick`, сворачивание через `expanded`. `DayHeader`: `ontitle` / `oncover`. `CircleRow.groupChips` только при `card`.

Лента (3.1 / 4.5 / 4.10):

| Компонент | Корень | Поведение |
|-----------|--------|-----------|
| `ReactionBar` | `.rx` + при открытии `.rxpick` | `groups` → `button.one`; `showAdd` → `button.add`; `pickerOpen` → `button.rcho` (`selectedKey` → `.on`); колбэки `onopenList` / `onadd` / `onpick`. Иконка — уже resolved `IconName` (`reactionIconName` живёт в `$lib`, не в `$ui`). Группировка, плюс, `?reactions=` — на маршруте. Ночь: `.ph.dark .rx button.one` подложка `#3A2E29`, иконка `--c`. |
| `CommentPreview` | `button.cm` | Сосед `PostCard`, не внутри `.post` (слот `comments` у карточки — другое, напр. ошибка очереди). Контент — snippet. |
| `ReactionListRow` | `div.row2` | Оверлей 4.5: Avatar + имя + Icon. Без `onclick`. Не расширять `MemberRow`: справа знак реакции, не subtitle/меню. |

### `overlays/`

Fab, CommentBar (`oncompose` — фото и шеврон; пустое поле на таче ведёт на compose, на ПК с мышью только фокус; без `oncompose` — полоса комментария), Scrim, Sheet, Dialog, PushBanner, Lightbox, AvatarCrop (светлые токены на корне `.crop`, не следует `.ph.dark`)

### `admin/`

AdminNav (`ADMIN_NAV`: Оплата после «Люди»; кадры 9.1–9.10 без пункта), AdminSection, DataTable, StackBar, CheckRow, StatusIcon, CodeBlock (`lines[]`, `.hi` / `span.cmt`), InlineInput, QuotaRequestRow (`Button` `.btn` / `.btn.gh` на «Дать» / «Отказать», не `.act`)

### Бренд

`Mark.svelte`, `Logo.svelte`, `Icon.svelte` + `web/static/icons.svg`

---

## Тема круга

```css
.ph { --c: var(--terracotta); --ct: #F1E1DA; }
.ph.teal { --c: var(--teal); --ct: #DCE8E9; }
/* + .shell, .dark, .wide */
```

Токены: `web/src/lib/styles/tokens.css`, классы: `web/src/lib/styles/ui.css`.

---

## Вне scope

Не менять [screens.html](../visual/screens.html). Не собирать все 58 экранов в `routes/` — только prod-маршруты по [client-reference.md](client-reference.md).

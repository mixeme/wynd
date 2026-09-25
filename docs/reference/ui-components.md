# Wynd UI — справочник

**Wynd UI** — design system Wynd: компоненты (`$ui/...` → `web/src/lib/components/`, без barrel `$ui/index.ts`), layouts (`$lib/layouts/`, без алиаса), CSS-токены (`tokens.css`, `ui.css`). Dev-каталог: `/dev/ui`. Guard: `npm run check:ui` (`web/scripts/check-ui.mjs` + `ui-guard.mjs`), не ESLint. Сторож агента: `.cursor/hooks/ui-screens.mjs`, правило `.cursor/rules/wynd-ui-screens.mdc`.

Сжатая выжимка из закрытого плана. Макеты: [screens.html](../visual/screens.html). Стек и спайк Bits UI vs shadcn: [stack.html](../stack.html).  
Таблица «компонент → экран» в `web/src/routes/dev/ui/catalog.ts`.

**Выбор:** ветка **Б** — Bits UI + свой CSS на токенах (`ui.css`, классы `.cbar`, `.post`, `.r`).

**Правила:** shell без цвета круга; accent через `--c` / `--ct`; Danger — ink border; Mark только в AppBar и пять других мест по макету. Новые npm-пакеты для UI не ставить. [screens.html](../visual/screens.html) не менять.

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

**Интерактив:** корень кнопки — `<button type="button">`, не `div`/`span` + `role="button"`. `Button.onclick` обязателен; без действия в dev/smoke — `onclick={() => {}}`. Загрузка — prop `loading` (текст `…`, вид `.off`); не `class:off` на экранах. `Chip` без `onclick` — `<span>`, с действием — `<button aria-pressed>`. Навигационные строки (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow` в режиме transfer) несут `onclick` на корне; обёртки-`div` не нужны. Ссылки с URL остаются `<a class="under" href>`. Identity в `CircleBar` — `<button type="button" class="idn">` (без `circleId` — `span.idn`), не `IconButton`. Compose: `TextButton` `bar` / `barAction`. Вложенные `<button>` запрещены: меню в `MemberRow` только если строка не кликабельна целиком. `AdminNav` при `links` — `<button type="button">`, не `span` + `role="button"`. Карточки ленты и дней: опциональный `onclick` на корневом `div` (`PostCard` / `DayCard`), без обёртки; карточку не делать `<button>` — вложенные реакции и альбом остаются кнопками со `stopPropagation`.

**Формы:** ввод — `Input` / `TextArea` / `SearchField`; статика — `FieldDisplay` (бывший `Field`). Админка: `Input admin={true}` и `FieldDisplay admin={true}` (класс `.inp`), не отдельный `AdminInput`. `SearchField` — редактируемый поиск и поля фильтров (тот же виджет: `/search`, поиск в круге, 9.8 почта); `BackBar` свой `.sfield`. `TextArea variant`: `area` \| `field`.

---

## Layout-шаблоны

`web/src/lib/layouts/`

| Layout | CSS | Примеры экранов |
|--------|-----|-----------------|
| PlainLayout | `.ph` | e1-1, e1-3 |
| ShellLayout | `.ph.shell` | e2-1, e2-2, e7-* |
| CircleLayout | `.ph.{color}` | e3-*, e4-*, e5-*, e6-* |
| FormLayout | `.ph.{color\|shell}` | e1-*, e2-3, e6-1 |
| OverlayLayout | absolute | sheets, dialogs, push |
| AdminWideLayout | `.ph.wide.shell` | e9-* |

---

## Компоненты по папкам

### `chrome/`

PhoneFrame, StatusBar, AppBar, CircleBar (4 таба), BackBar, AdminBar

### `forms/`

**Label**, **Input**, **FieldDisplay**, **TextArea**, ScreenTitle, Hint, **Button**, **Chip**, ChipGroup, Switch, ColorSwatches, CodeBox, InviteCard, **SearchField**, DangerZone, Meter, PeopleStrip, AddPhotoButton, DangerNote, VolumeChart, **IconButton**, **TextButton**

Интерактивные примитивы (фаза 1–4):

| Компонент | Корень | Обязательные props | Поведение |
|-----------|--------|-------------------|-----------|
| `Button` | `<button class="btn">` | `onclick` | `variant`, `disabled`/`loading` → класс `.off`, текст `…` |
| `Chip` | `<button class="chip">` или `<span>` | — | с `onclick` — кнопка, `aria-pressed={selected}` |
| `IconButton` | `<button class="ib">` | `name`, `label`, `onclick` | рендер только при `onclick` |
| `TextButton` | `<button>` | `onclick` | `variant`: `link` (`.under`), `admin` (`.act`), `bar` (`.t`), `barAction` (`.rt`/`.rt.on`) |

Формы (фаза 5):

| Компонент | Корень | Поведение |
|-----------|--------|-----------|
| `Label` | `<div class="lab">` | подпись поля |
| `Input` | `<input class="fld">` или `.inp` | `admin`, `active`, `gray`, `bind:value` |
| `FieldDisplay` | `<div class="fld">` или `.inp` | статика (бывший `Field`); `admin` → `.inp` (9.7 URL инвайта) |
| `TextArea` | `<textarea class="ta">` или `.fld` | `variant`: `area` \| `field`, `bind:value` |
| `SearchField` | `.sfield` + `<input type="search">` | иконка, `bind:value`; поиск и фильтры |

Guard: `npm run check:ui` — экран = существующие `$ui` + `$lib/layouts` (не Bits UI, не одноразовый `.svelte` у маршрута, в `routes/` только `+page`/`+layout`/`+error`); новый файл в библиотеке — только по открытому плану «пробел Wynd UI»; в `web/src` запрещён импорт `$lib/components` (использовать `$ui`); в `routes/` запрещены `role="button"`, сырой `class="btn"`, сырой `class="lab"`, `<input class="fld">` и `<textarea class="fld|ta">`.

### `data/`

SectionLabel, Avatar, EventDivider, CircleRow, PostCard, **SettingsRow**, MemberRow, SearchGroupHeader, **SearchResultRow**, **ServerRow**, FoldHeader, AttachmentRow, PhotoPlaceholder, PhotoGrid, MonthLabel, DayCard, DayGrid, DayHeader, EntryDateMark, ArchiveBanner

Строки с опциональным `onclick`: корень `button.row2` / `button.r` или `div` (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow`). `SettingsRow` с snippet `control` — всегда `div.row2`, справа контрол (например `Switch`); `chevron`/`value` не рендерятся; title без `font-weight:600`. `PostCard` / `DayCard` — `div` с опциональным `onclick`, не `<button>`.

### `overlays/`

Fab, CommentBar (`oncompose` — фото и шеврон; без него полоса комментария), Scrim, Sheet, Dialog, PushBanner, Lightbox, AvatarCrop

### `admin/`

AdminNav, AdminSection, DataTable, StackBar, CheckRow, StatusIcon, CodeBlock, InlineInput, QuotaRequestRow (`Button` `.btn` / `.btn.gh` на «Дать» / «Отказать», не `.act`)

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

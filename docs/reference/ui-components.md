# Wynd UI — справочник

**Wynd UI** — design system Wynd: компоненты (`$ui/...` → `web/src/lib/components/`, без barrel `$ui/index.ts`), layouts (`$lib/layouts/`, без алиаса), CSS-токены (`tokens.css`, `ui.css`). Dev-каталог: `/dev/ui`. Guard: `npm run check:ui` (`web/scripts/check-ui.mjs` + `ui-guard.mjs`), не ESLint. Сторож агента: `.cursor/hooks/ui-screens.mjs`, правило `.cursor/rules/wynd-ui-screens.mdc`.

Библиотека интерфейса: правила и компоненты по папкам. Макеты: [screens.html](../visual/screens.html). Стек и выбор Bits UI: [stack.html](../stack.html).  
Таблица «компонент → экран» в `web/src/routes/dev/ui/catalog.ts`.

**Выбор:** ветка **Б** — Bits UI + свой CSS на токенах (`ui.css`, классы `.cbar`, `.post`, `.r`).

**Правила:** shell без цвета круга; accent через `--c` / `--ct`; Danger — ink border; Mark только в AppBar, в `Loading` и пять других мест по макету. Новые npm-пакеты для UI не ставить. [screens.html](../visual/screens.html) и [wynd.html](../wynd.html) не синхронизировать с `VERSION`.

**Пробел библиотеки:** экран — только существующие `$ui` и `$lib/layouts`. Не хватает куска — сначала расширить уже лежащий компонент. Если объективно нельзя, это **отдельная задача** на Wynd UI: `docs/plans/<slug>.plan.md`, затем стоп. В той же задаче `.svelte` в `$ui` не заводить и дыру на экране не верстать. Сторож пропустит новый файл позже, только если открытый план его перечисляет в таблице.

**`/dev/*` в сборке.** `/dev/ui` (каталог компонентов) и `/dev/smoke/*` (кадры для сверки) остаются и закрыты проверкой `dev` в `routes/dev/+layout.ts`: вне `vite dev` маршрут отвечает 404 — SvelteKit не умеет исключать маршруты из сборки. `/dev/spike/*` (отчёт о сравнении библиотек) удалён в 0.7.3.

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

**Интерактив:**

- Корень кнопки — `<button type="button">`, не `div`/`span` + `role="button"`. `Button.onclick` обязателен; без действия в dev/smoke — `onclick={() => {}}`. Загрузка — prop `loading` (текст `…`, вид `.off`); не `class:off` на экранах. `Chip` без `onclick` — `<span>`, с действием — `<button aria-pressed>`.
- Навигационные строки (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow` в режиме transfer) несут `onclick` на корне. `CircleRow` в режиме `card` — `div`-карточка: действие и чипы групп под строкой, без вложенных `button`. Ссылки с URL остаются `<a class="under" href>`.
- Identity в `CircleBar` — `<button type="button" class="idn">` (без `circleId` — `span.idn`), не `IconButton`. Compose: `TextButton` `bar` / `barAction`. Вложенные `<button>` запрещены: меню в `MemberRow` только если строка не кликабельна целиком. `AdminNav` при `links` — `<button type="button">`, не `span` + `role="button"`.
- Карточки ленты и дней: `PostCard` — корневой `div.post` (вложенные контролы, не `<button>`); клик по телу через action, не `onclick` на разметке. `DayCard` при `onclick` — `<button type="button">`, иначе `div`.
- `PostCard` action игнорирует `button, a, input, textarea, select, label, .rxpick` — чипам реакций `stopPropagation` не нужен; альбом и прочие не-кнопки по-прежнему останавливают всплытие сами.

**Формы:** `Label aside="необязательно"` — мелкая подпись справа без капители. Ввод — `Input` / `TextArea` / `SearchField`; статика — `FieldDisplay` (бывший `Field`). Админка: `Input admin={true}` и `FieldDisplay admin={true}` (класс `.inp`), не отдельный `AdminInput`. `Input mono` — моноширинный (адрес, почта), `Input small` — 12,5 px для длинного значения в узком поле. Размер и шрифт поля — пропами, не служебными классами: `input.fld` / `input.inp` сильнее одного класса (`.w72`, `.sz-12`, `.mono` на поле не действуют). `SearchField` — редактируемый поиск и поля фильтров (тот же виджет: `/search`, поиск в круге, 9.3 почта); `BackBar` свой `.sfield`. `TextArea variant`: `area` \| `field` \| `compose` \| `comment`.

---

## Layout-шаблоны

`web/src/lib/layouts/`

| Layout | CSS | Примеры экранов |
|--------|-----|-----------------|
| PlainLayout | `.ph` | e1-1, e1-3 |
| ShellLayout | `.ph.shell` + при `app` `.shell-body` | e2-1, e2-3, e7-* |
| CircleLayout | `.ph.{color}` | e3-*, e4-*, e5-*, e6-* |
| FormLayout | `.ph.{color\|shell}` | e1-3, e1-4, e2-4, e2-7, e6-1 |
| OverlayLayout | absolute | sheets, dialogs, push; `ondismiss` → Scrim и модальность; `label` → `aria-label` |
| AdminWideLayout | `.ph.wide.shell` | e9-* |
| PayGateLayout | — (шлюз, не каркас) | `+layout` групп `/circles/*` и `/search/*`: пока доступ не оплачен — вместо экрана «Доступ закрыт» с реквизитами или заявкой в ожидании (**#e10-***), иначе children |

`FormLayout`: при `circleTitle` — шапка вступления **#e1-3** (`div.cbar`, имя по центру, без `BackBar`); список участников на join (`?members=1`) — `color` + `subtitle` (цветная `.cbar` с назад), не серый `BackBar`. Ветки: `compose`, `circleTitle`, snippet `bar`, `color && subtitle`, иначе `BackBar`.

Сиблинги `.dlg` / `.sheet` / `.scrim` вне `.ph.app` — колонка `--app-max-width` (не `inset:auto` на scrim: сбивает `left:50%`). Лайтбокс `fixed` — весь вьюпорт. `VolumeChart` — шаг 26px или сжатый, пачка по центру; отсечка квоты из `volumeChart.ts`.

---

## Компоненты по папкам

### `chrome/`

PhoneFrame, StatusBar, AppBar, CircleBar (4 таба), BackBar, AdminBar, **ComposeToolbar**

`ComposeToolbar` — нижняя полоса новой записи (**#e4-2**): snippet `tools` (кнопки вложений), опц. snippet `note` — мелкая подпись справа («до 32 КБ · как Мышь»). Ставится в `footer` у `FormLayout compose`.

`AudioBar` — полоса плеера (**#e4-20**): что играет, откуда, ход в цвете круга, пауза и крестик; ставится один раз в корневом `+layout`. Видна на любом экране, кроме ленты круга звука и экрана его записи. Пока видна, задаёт `--player-h` — полоса ввода, плюс и нижние панели встают над ней. Только вид: `title`, `subtitle`, `coverUrl`, `color`, `progress`, `loading`, `playing`, `onopen`, `ontoggle`, `onstop`. Что играет, когда прятать и `--player-h` — `useAudioBar()` из `$lib/media/audioBar.svelte` (план 48).

### `forms/`

**Label**, **Input**, **FieldDisplay**, **TextArea**, ScreenTitle, Hint, **Button**, **Chip**, ChipGroup, Switch, ColorSwatches, CodeBox, InviteCard, **RequisitesCard**, **SearchField**, DangerZone (опц. `style`), Meter, PeopleStrip, **MentionPicker**, AddPhotoButton, DangerNote, VolumeChart, **IconButton**, **TextButton**, **EditWindowPicker**, **IdentityForm**, **NumberField**, **DateRow**, **DateRange**, **FilePicker**, **QrScanner**, **GenderPicker**

Интерактивные примитивы (фаза 1–4):

| Компонент | Корень | Обязательные props | Поведение |
|-----------|--------|-------------------|-----------|
| `Button` | `<button class="btn">` | `onclick` | `variant`, `disabled`/`loading` → класс `.off`, текст `…` |
| `Chip` | `<button class="chip">` или `<span>` | — | с `onclick` — кнопка, `aria-pressed={selected}` |
| `IconButton` | `<button class="ib">` | `name`, `label`, `onclick` | рендер только при `onclick`; опц. pointer-события (длинное нажатие FAB); `pressed` — переключатель: `true` в цвете круга, `false` приглушён (`.ib.off`), с `aria-pressed` |
| `TextButton` | `<button>` | `onclick` | `variant`: `link` (`.under`), `admin` (`.act` в `.chk`), `adminBox` (`.inp`), `bar` (`.t`), `barAction` (`.rt`/`.rt.on`) |
| `AddPhotoButton` | `button.addph` | `onclick` | без `previewUrl` — плюс; с `previewUrl` — `.addph.preview`, cover-фон, `aria-label` «сменить фото» (**#e1-3**) |

`CodeBox` — шесть клеток `.codebox`; без `bind:value` — display (`digits` / `active`, каталог). С `bind:value` — прозрачный `input.code-input` поверх (**.code-wrap**), `inputmode="numeric"`, `autocomplete="one-time-code"`, обрезка до `length`; опц. `bind:el`, `autofocus` (**#e1-2**).

`DangerNote` — красная зона «Необратимо»: пояснение — children; с `title` — заголовок действия («Удалить с сервера») и snippet `action` с кнопкой под текстом (`.danger.titled`).

`EditWindowPicker` — «Окно правок» (**#e2-4**, **#e6-2**): шесть чипов в два ряда и при «Своё…» поле часов; `value`, `bind:customHours`, `onpick(key)`, опц. `oncustomchange` — настройки круга сохраняют сразу. Подсказку о сдвиге часов экран ставит сам.

`GenderPicker` — «Пол» для строк журнала (**#e1-3**, **#e6-7**; план 46, A5): чипы «Мужской» / «Женский», `bind:value` (`''` \| `'m'` \| `'f'`), повторное касание снимает выбор. Стоит в `IdentityForm` и в «Кто вы в этом круге».

`QrScanner` — видоискатель QR (**#e2-17**): задняя камера, кадр раз в 250 мс; `onread(text)` → `true` — хватит (камера гаснет), `false` — ждать следующий; `onerror(message)` — камеры нет или её не дали. Камера гаснет и при уходе с экрана.

`FilePicker` — скрытый выбор файлов, который открывает своя кнопка: `accept`, `multiple`, `capture`, `onfiles(files)`; экран держит его через `bind:this` и зовёт `open()`. Поле сбрасывается само — тот же файл можно выбрать снова. Сырые `<input type="file" hidden>` на экранах не ставить.

`DateRange` — период «с — по» двумя полями даты поровну (**#e2-9**, поиск в круге): `bind:from`, `bind:to`.

`DateRow` — строка, которая открывает системный выбор дня (**#e4-2** «Отнести к дате»): `bind:value` (`2026-10-01`), `title`, `subtitle`, `icon`, `style`. Поле даты скрыто под строкой (`.date-pick`), выбор открывается у неё; без `showPicker` — по клику.

`NumberField` — число «Своё…» с единицей справа: `Input` 72 px и подпись `.hint` в `.rowin` (окно правок, кэш, квота, люди и дни приглашения). `bind:value` (число), `min`, `max`, `unit` («часов», «ГБ, до 100»), опц. `onchange` — подтверждение поля. Строки админки с подписью слева — `AdminField`.

`IdentityForm` — «Как вас зовут в этом круге?» (**#e1-3**) у вступающего и у создателя круга: заголовок, `AddPhotoButton` + кадрирование `AvatarCrop`, «Имя», первая запись с «необязательно». `color`, `bind:name`, `bind:firstPost`, `bind:avatar` (откадрированное фото; загружает экран — `setIdentityAvatar` из `$lib/circles/settings`), опц. `postLabel` / `postPlaceholder` (дневник), `onerror`. Кнопку и ошибку ставит экран.

`RequisitesCard` — платёжные реквизиты в `div.req` (**#e10-1** / **#e10-2** / **#e10-6** / **#e10-8**): проп `text` или snippet `children`; стили в `ui.css`, не кликабельна.

`MentionPicker` — оболочка `div.men-pick` для списка `@` (**#e4-2**, **#e4-3**): snippet `children` (`MemberRow` с экрана). Базовые стили `.men-pick` в `ui.css`; в полосе комментария — `.comp-wrap .men-pick` (отступы, `max-height`). Фильтрация — в `CommentBar` / compose, не в компоненте.

Формы (фаза 5):

| Компонент | Корень | Поведение |
|-----------|--------|-----------|
| `Label` | `<div class="lab">` | подпись поля |
| `Input` | `<input class="fld">` или `.inp` | `admin`, `active`, `gray`, `bind:value`. `gray` — `.fld.gr` (`--faint`); не для живого ввода, который надо прочитать |
| `FieldDisplay` | `<div class="fld">` или `.inp` | статика (бывший `Field`); `admin` → `.inp` (9.2 URL инвайта) |
| `TextArea` | `<textarea class="ta">`, `.fld`, `.compose-text` или `.inp` | `variant`: `area` \| `field` \| `compose` \| `comment`, `bind:value`; `compose` — зеркало + `.men` для `@имя` |
| `SearchField` | `.sfield` + `<input type="search">` | иконка, `bind:value`; поиск и фильтры |
| `VolumeChart` | `.chart` + SVG | `volume`, `cutoffLabel`, `bind:cutoffX`, `oncutoff(index)`; при `oncutoff` — `role="slider"`, Tab, стрелки ±месяц, PageUp/PageDown ±год, Home/End (UI-4); без него — `aria-hidden` |

Guard: `npm run check:ui` — экран = существующие `$ui` + `$lib/layouts` (не Bits UI, не одноразовый `.svelte` у маршрута, в `routes/` только `+page`/`+layout`/`+error`); новый файл в библиотеке — только по открытому плану «пробел Wynd UI»; в `web/src` запрещён импорт `$lib/components` (использовать `$ui`); в `routes/` запрещены `role="button"`, сырой `class="btn"`, сырой `class="lab"`, `<input class="fld">`, `<textarea class="fld|ta">`, сырой `<button class="row2|one|cm|…">`, сырой `<div class="row2">` и сырой `class="compose-text"` (prod-маршруты, не `/dev`). Классы сторож читает по разметке (скрипт и стили вырезаны), из открывающего тега целиком: слова `class="…"` в любом месте, строковые литералы в `class={…}`, в интерполяциях значения и внутри `${…}` шаблонных строк, директивы `class:x` (`markupElements`, `RAW_CLASS_RULES`; GUARD-1). Динамический импорт `.svelte` (`import(…)`, `import.meta.glob`) и `<svelte:component>` в экранах сторож отклоняет: компонент из переменной обошёл бы проверку импортов (GUARD-4).

Свои `<style>` есть у шести экранов: корневые `+layout`/`+page` и четыре `admin/pay/**`. Они перечислены в `STYLE_BLOCK_SCREENS`; `<style>` в любом другом боевом экране сторож отклоняет, а строку экрана, где `<style>` убрали, просит вычеркнуть — список ходит только вниз (GUARD-3).

**Классы на `<button>`.** Браузерные умолчания снимает один сброс `:where(button)` в начале `ui.css` (нулевая весомость), поэтому любой `.X` ложится на кнопку как на `div` — зеркал `button.X` нет (с 0.7.3).

Нативный `<button>` не растягивается как `div`: `.btn` сам задаёт `display:block` и `width: calc(100% - 32px)` (поля 16+16, как `input.fld`); в `.rowin` / `.chk` — `width:auto`. Текстовые (`act`, `t`, `rt`, `under`) — padding:0 намеренно. Какие классы можно вешать на `<button>` в `$ui` — `BUTTON_LAYOUT_CLASSES` / `BUTTON_TEXT_CLASSES` в `web/scripts/ui-guard.mjs`; новый класс вне списков роняет `npm run check:ui`.

### `data/`

SectionLabel, Avatar, EventDivider, **FeedDayPromptCard**, CircleRow, PostCard, **ReactionBar**, **CommentPreview**, **CommentRow**, **ReactionListRow**, **SettingsRow**, MemberRow, SearchGroupHeader, **SearchResultRow**, **ServerRow**, FoldHeader, **GroupFoldCard**, AttachmentRow, PhotoPlaceholder, PhotoGrid, **MediaTile**, **MapBadge**, **MapPostSheet**, MonthLabel, DayCard, DayGrid, DayHeader, EntryDateMark, ArchiveBanner, **PayStreetBanner**, **MentionText**, **AttachmentList**, **QrCode**, **InviteLinkCard**, **EmptyState**, **PullRefresh**, **FeedEnd**, **ResponseEntry**, **PostRef**, **PostByline**, **PeekMemberList**, **AboutFooter**

`PayStreetBanner` — баннеры оплаты на улочке (`/circles`, кадр **#e10-5**): `variant` `donate` \| `reminder` \| `pending`. Donate — `text`, `onclick` (help), опционально `dismissible` / `ondismiss`. Reminder — `expiresAtLabel`, `reminderDaysLeft`, `onclick` (extend). Pending — `pendingAtLabel`, опционально `expiresAtLabel`; без корневой кнопки. Стили `.pay-banner*` в `ui.css`; кликабельные зоны — `button.pay-banner-main`, `button.pay-reminder`.

`MentionText` — текст записи или комментария (`body`): `@имя` цветом круга, переносы и пустые строки как написаны (`white-space: pre-wrap` на `.mention-text`).

`AttachmentList` — вложения записи, не фото и не видео (**#e4-13**, **#e4-15**, **#e4-18**): звуки подряд — одна рамка `.att-group` со строками `grouped`, одиночный звук — `AttachmentRow audio`, файл — строка «скачать». `items`, `origin`, `circleId`, `circleName`, `color`, `postId`, `coverUrls` (обложки звуков по blob id).

`PeekMemberList` — «Кто уже здесь» до входа (**#e1-3**): `members` из приглашения или заявки; строки `MemberRow`, цвета по порядку.

`AboutFooter` — подвал «Wynd x.y · AGPL-3.0 · исходный код · лицензии» (настройки приложения, панель): `wrap` — ссылки второй строкой, `class` — отступ. Адрес исходников экран загружает сам (`loadSourceUrl`).

`PostByline` — левая часть шапки карточки записи, для snippet `author` у `PostCard`: аватар (`initial`, `color`, `src`), `name`, `time`, опц. `icon` перед временем (часы у записи в очереди). Справа в шапке — `EntryDateMark` (`label`, `icon`: `day` — к какому дню, `clock` — «внесено сегодня»).

`CommentPreview` — комментарии под карточкой в ленте (`button.cm`): `first` (с временем `time`), `more` («ещё N»), `onclick`; `children` — своё содержимое (кадры /dev).

`ResponseEntry` — строка «Откликов» (**#e3-13**): ссылка туда, где отклик живёт (`href`, `onopen`), внутри `CommentRow bare`; `initial`, `name`, `color`, `src`, `icon` (знак реакции), `label` («комментарий · 14:02»), под словами — children (текст, `PostRef`). Между строками — черта.

`PostRef` — рамка «к чему это» под откликом: `author` (жирным), `date`, `excerpt`, `cover` (место под обложку сразу) и `coverUrl`. Может заменить шапку `MapPostSheet` / `SearchResultRow` (план 47, 3.1).

`FeedEnd` — низ ленты круга (**#e3-1**): знак и откуда лента видна. `since` — человек видит круг не с начала («Вы здесь с…», «что было раньше — не ваше», знак в цвете круга); без него — «Здесь начинается круг», знак приглушён. `started` — дата начала круга (при `since` — с годом). Даты — готовыми строками.

`PullRefresh` — полоса «потянуть, чтобы обновить» (3.5): знак дорисовывается с жестом и держится, пока идёт обновление; проп `pull` (состояние). Жест ведёт контроллер `PullRefresh` из `$lib/gestures/pullRefresh.svelte` (`new PullRefresh(scrollTop, refresh)`, обработчики `start` / `move` / `end` на прокручиваемом списке, `destroy()` при уходе). Полоса — над списком; список — `.feed` (`overscroll-behavior: contain`, иначе браузер показывает своё обновление).

`EmptyState` — экран или вкладка без содержимого: заголовок по центру, пояснение (children), кнопки (snippet `actions`). `place` — отступы по кадрам: `screen` (экран-состояние, 48 px, поля пояснения 10/30), `list` (пустая улочка **#e2-3**, 24 px), `tab` (пустая вкладка **#e3-14**, заголовок посередине, 8/34), `feed` (пустая лента **#e3-1** со знаком, 8/34, кнопка с полями 60 px).

`QrCode` — QR-код ссылки в рамке `.qr` (**#e6-7**, **#e6-21**, админка «Доступ»): `value` (пусто — ничего), `size` `md` (168 px) \| `sm` (150 px), `class`. SVG строит сам из `qrcode`; `{@html}` в экранах запрещён сторожем.

`InviteLinkCard` — ссылка-приглашение целиком (`FieldDisplay.invite-url`) и «Поделиться / Скопировать» (**#e6-7**, **#e6-21**): `url`, `shared` / `copied` (подписи «Отправлено» / «Скопировано»), `onshare`, `oncopy`. В листе — те же поля экрана, что у ссылки; лист ставится в `footer` макета, чтобы кнопкам достался цвет круга. QR над ней экран ставит сам.

`CommentRow` — строка треда (`div.cmt`, опц. `.q`): аватар, `name`, snippet `time`, snippet `children` (текст / правка); `onedit` / `ondelete` → `.acts` (**#e4-5**–**#e4-7**). Колонка `.acts` в треде стоит всегда — текст одной ширины у своих и чужих; `bare` — без неё (строка-ссылка, `ResponseEntry`). Не путать с `CommentPreview` (`button.cm` в ленте).

`FeedDayPromptCard` — служебная карточка в ленте (`div.post.day-prompt`): `title`, snippet текста, `primaryLabel` / `secondaryLabel`, `onprimary` / `onsecondary` (`Button` в `.rowin`). Стили `.day-prompt` и раскладка ленты (`.feed`, `.ptr`, `.empty`, `.feed-end`) — в `ui.css`.

`SearchResultRow` — строка найденного (**#e2-9**): `author`, `time`, превью — либо snippet `preview`, либо по `kind` (`post` / `comment` — «· комментарий» / `day` — «день») из `snippet` с найденным словом `query` в ёлочках (`quoteMatch` из `$lib/journal/search`); миниатюра `thumb` / `thumbUrl` / `thumbVariant`.

`MapBadge` / `MapPostSheet` — вкладка «Карта» круга: счётчик пинов (`.map-badge`) и нижняя плашка выбранной записи (`button.map-sheet`, thumb + автор + время + текст). Стили в [`map.css`](../../web/src/lib/styles/map.css); сброс кнопки — `button.map-sheet` в `ui.css`.

`MediaTile` — фото/видео в ленте, сетке, альбоме, шапке поста и compose: `variant` `feed` \| `grid` \| `album` \| `headerMini` \| `compose`. `feed` — `div.pic` + `stopPropagation` при `onclick`; `grid` / `album` / `headerMini` / `compose` — `button`. Стили `.cell`, `.thumbs`, `.pic.sq.mini` в `ui.css`; счётчик сетки — `.g3 .cnt`. `Lightbox`: опционально `fixed`, `dotCount` / `dotIndex` / `onDotSelect` (snippet `dots` в приоритете).

Строки с опциональным `onclick`: корень `button.row2` / `button.r` или `div` (`SettingsRow`, `CircleRow`, `ServerRow`, `SearchResultRow`, `MemberRow`).

- `SettingsRow divided` (`top` \| `bottom` \| `both`) — черта над и/или под строкой, когда она стоит среди текста (не в списке строк). `DateRow` пробрасывает `divided` и `class`.
- `Meter` — полоса заполнения; `inline` — короткая полоска в строке таблицы (`.qbar`, квота круга в админке), `color` — свой цвет заполнения.
- `PhotoGrid album` — сетка альбома с полями 12 px по бокам и 16 снизу.
- `Avatar size="lg"` — 96 px по центру, буква 38 px («Кто вы в этом круге»).
- `SettingsRow` с snippet `control` — всегда `div.row2`, справа контрол (например `Switch`); `chevron`/`value` не рендерятся; title без `font-weight:600`.
- `Switch`: `bind:checked` — для формы с кнопкой «Сохранить»; `onchange(checked)` — когда нажатие сразу пишет на сервер: вызывается только от нажатия, не от смены `checked` извне (иначе обновление карточки перезаписало бы сервер старым значением).
- `PostCard` — `div.post`, клик через action; `DayCard` — `<button>` при `onclick`, иначе `div`. `FoldHeader` — `button.fold` при `onclick`, сворачивание через `expanded`. `DayHeader`: `ontitle` / `oncover` (`button.pic` при `oncover`).
- `CircleRow`: при `card` — оболочка `.circle-row-card`, `actionLabel` / опционально `actionLabel2` под строкой (`circle-row-action`). `GroupFoldCard` — та же оболочка для **группы** на улочке (**#e2-14**): `FoldHeader` + до двух `circle-row-action`; long-press и сворачивание — rest на `FoldHeader`, опционально `foldStyle`.

Лента (3.1 / 4.12 / 4.10):

| Компонент | Корень | Поведение |
|-----------|--------|-----------|
| `ReactionBar` | `.rx` + при открытии `.rxpick` | `groups` → `button.one`; `showAdd` → `button.add`; `pickerOpen` → `button.rcho` (`selectedKey` → `.on`); колбэки `onopenList` / `onadd` / `onpick`. Иконка — уже resolved `IconName`; группировка, плюс, `?reactions=` — на маршруте (ниже). |
| `CommentPreview` | `button.cm` | Сосед `PostCard`, не внутри `.post` (слот `comments` у карточки — другое, напр. ошибка очереди). Контент — snippet. |
| `ReactionListRow` | `div.row2` | Оверлей 4.12: Avatar + имя + Icon. Без `onclick`. Не расширять `MemberRow`: справа знак реакции, не subtitle/меню. |

`ReactionBar` получает иконку готовой: `reactionIconName` живёт в `$lib`, не в `$ui`. Ночная тема: `.ph.dark .rx .one` — подложка `#3A2E29`.

### `overlays/`

`Fab` — кружок `.fab` в `.fab-wrap`; опц. `menuOpen` + `items[]` — карточка `.fab-menu` над плюсом (**#e2-11**), `role="menu"`: при открытии фокус на первом пункте, стрелки по кругу, Escape → `onclose` (UI-3; `ShellLayout` — `onfabmenuclose`). `ShellLayout` прокидывает `fabMenuOpen` / `fabMenuItems`; snippet `fab` — только содержимое кружка. Fab, CommentBar (`oncompose` — фото и шеврон; пустое поле на таче ведёт на compose, на ПК с мышью только фокус; без `oncompose` — полоса комментария), Scrim (`button.scrim`), Sheet, Dialog, PushBanner, Lightbox (`.mid` — `role="region"`), AvatarCrop, **ConfirmDialog**, **ReactionsSheet**

`ConfirmDialog` — вопрос с двумя кнопками на `Scrim` + `Dialog` (`.dlgq`, `.rowin.ask`): `title`, `confirmLabel`, опц. `cancelLabel` («Отмена»), `loading`, `onconfirm`, `oncancel`, пояснение — children. Все подтверждения «удалить / исключить / передать / выйти» — на нём.

`ReactionsSheet` — лист «Кто отреагировал» (**#e4-12**) на `Scrim` + `Sheet`: `reactions`, `color`, `ondismiss`; строки — `ReactionListRow`.

**Модальность оверлеев (UI-1, UI-2).** `Dialog` и `Sheet` с `ondismiss`, `Lightbox` с `fixed` и `onclose` — `role="dialog"`, `aria-modal`, action `modal` из `$lib/a11y/modal`: фокус при открытии — на первый фокусируемый элемент, Tab по кругу внутри, фокус снаружи возвращается внутрь, Escape → `ondismiss`, при закрытии фокус — туда, где был. Оверлеи в стеке: клавиши слушает только верхний. Без `ondismiss` (каталог `/dev/ui`) оверлей статичен и ничего не перехватывает. Экран даёт оверлею имя через `label` — обычно его заголовок. `AvatarCrop` на том же action с `initialFocus: 'last'`: фокусируемы только кнопки панели, фокус при открытии и при возврате снаружи — на «Готово», Escape → «Отмена» (во время сохранения — ничего). (светлые токены на корне `.crop`, не следует `.ph.dark`)

**Строки `row2` — на общей основе `Row`** (UI-5): `ServerRow`, `SettingsRow`, `MemberRow`, `SearchResultRow` — тонкие обёртки над `$ui/data/Row.svelte` (сниппеты `leading` / `main` / `trailing`; с `onclick` — `<button>`, без — `<div>`). Экраны зовут именные строки: их имена и параметры — словарь кадров, кадры не меняются. `CircleRow` (`.r`, другая форма) держит одно содержимое в сниппете вместо четырёх копий. Разметку всех пяти во всех вариантах сторожат слепки `rows.test.ts`, снятые с прежних компонентов. Новая строка `row2` — ещё одна обёртка над `Row`, не копия разметки.

### `admin/`

AdminNav (`ADMIN_NAV`: Проверка, Общие, Доступ, Люди, Хранилище, Сжатие, Оплата; кадры 9.1–9.10 без «Оплата»), AdminSection, DataTable, StackBar, CheckRow, StatusIcon, CodeBlock (`lines[]`, `.hi` / `span.cmt`), InlineInput, **CheckRow** (`dotColor` — цветной квадратик вместо значка статуса), QuotaRequestRow (`Button` `.btn` / `.btn.gh` на «Дать» / «Отказать», не `.act`), **Panel**, **AdminField**, **SwitchRow**

`SwitchRow` — переключатель админки с заголовком и пояснением (кадры 9.x, 10.x): `bind:checked`, `title` (он же подпись `Switch` для чтения с экрана), пояснение — children. Переключатель на 2 px ниже верха строки, как на кадрах.

`Panel` — карточка админки `.panel` (рамка, фон карточки, скругление; отступ — служебным классом: `pad-12`, `pad-16`). `AdminField` — строка формы «подпись · поле · единица» (**#e9-***): `label`, `width` 88 \| 120 (короткие и длинные подписи, чтобы поля стояли столбцом), опц. `unit`, поле — children. Сетку мастера (`form-grid`) не заменяет.

### Бренд

`Mark.svelte`, `Logo.svelte`, `Icon.svelte` + `web/static/icons.svg`

**`Loading.svelte`** — экран, пока грузится: знак Wynd по центру покачивается в цвете круга (`--c`, вне круга — `--faint`), появляется через 0,3 с, при `prefers-reduced-motion` неподвижен, для экранного чтеца — `role="status"` и «Загрузка…». Вместо строки «Загрузка…» на экранах: на весь экран — `<Loading />` (стартовая, список кругов, круг и его вкладки, разметка панели), внутри раздела панели — `<Loading compact />`. Кадра в макетах нет. Добавлен в 0.10.4 по прямой просьбе владельца, без плана-пробела.

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

# План 47. Сверка экранов с Wynd UI

Пункт C3 плана 46. Требование: экраны собираются из библиотеки `$ui`, а не рисуют свои компоненты разметкой, inline-стилями и экранными классами. Ниже — аудит 2026-10-01 (63 экрана без `dev/**`, 58 находок: 15 высоких, 30 средних, 13 низких) и порядок работ в конце («Что делать — по порядку»). Выполненное отмечается в этом файле.

# C3. Сверка экранов с Wynd UI — аудит

Дата: 2026-10-01. Ветка `main` @ c3695cc. Проект не менялся: отчёт только читает код.

## Итог

- Проверено экранов: **63** файла `web/src/routes/**/*.svelte` (без `dev/**`). Библиотека: 83 компонента `$ui` и 6 макетов.
- Находок: **58**, из них **высокая — 15**, **средняя — 30**, **низкая — 13**.
- Инлайн-стилей в экранах **328** (столько же в `inline-style-budget.json`). Из них **239 (73 %) уже сейчас заменяются служебными классами**, которые есть в конце `ui.css`. Ещё 24 заменяются частично, 47 требуют нового класса или пропа, 19 динамические и оправданы (высота жеста, цвет круга).
- Главные дыры:
  1. Два экрана — побайтные копии: `circles/+layout.svelte` ≡ `search/+layout.svelte` (отличаются только концами строк).
  2. Одни и те же виджеты переписаны от руки в нескольких местах: диалог подтверждения (6 раз), лист реакций (2), блок звуков `.att-group` (2), форма «имя/фото/первая запись» (2), выбор окна правок (2).
  3. `AudioBar` попал в библиотеку в обход плана пробела: запрет на новые файлы живёт только в хуке Cursor, а `check:ui` его не проверяет.

### Что сторож (`ui-guard.mjs`) уже ловит — и чего не видит

Ловит:
- импорты только из `$ui/…` и `$lib/layouts/…`, а также что такой компонент есть в библиотеке;
- `<svelte:component>`, динамический `import()` и `role="button"`;
- сырые классы: `btn` на любом теге, `input.fld`, `textarea.fld|ta`, `lab`; на `<button>` в боевых экранах — `row2|rcho|one|add|cm|att|act`; `div.row2`; `compose-text`;
- `$lib/components` вместо `$ui`;
- `<style>` в экранах (кроме 6 разрешённых);
- храповик числа инлайн-стилей, по файлу;
- новые классы `<button>` внутри `$ui`.

Не ловит — отсюда почти все находки ниже:
1. **Сырые классы библиотечных компонентов на `div`/`span`/`a`**: `.hint`, `.h1s`, `.tm`, `.att` на `div`, `.chk`, `.danger`, `.qr`, `.men`, `.panel`, `.under`. Список `RAW_CLASS_RULES` короткий.
2. **Классы, собранные в переменной**: `const row = 'flex-mid gap-10'` (admin/general:23), `cardStyle`/`descStyle` (admin/bootstrap:22–23). Токены внутри `{row}` не литералы, и `classTokens` их не видит.
3. **Экранный CSS в `ui.css`**: правил для `ui.css` нет вовсе. Классы вида `.resp-*`, `.label-row`, `.invite-url` дописываются в глобальный файл без ограничений.
4. **Качество инлайн-стилей**. Храповик считает только их число. Стиль, который один в один повторяет существующий служебный класс (`margin:16px` = `.gutter`), проходит.
5. **Дубли разметки между экранами**: сходство никто не проверяет.
6. **Новые файлы в `$ui` без плана пробела**. `classifyNewSvelte` вызывается только из `.cursor/hooks/ui-screens.mjs`. В `checkProject` нет сверки «каждый файл библиотеки ↔ план или базовый список». Сессии Claude Code (у них нет хуков в `.claude/`) и ручные правки этот запрет обходят — так и появился `chrome/AudioBar.svelte`.
7. **`{@html}` в экранах**: QR-код в 3 местах.
8. **Сырые `<input type="file" hidden>` и `<a class="under">`**: `rawFldInput` смотрит только на класс `fld`.

---

## Библиотека: что есть

Почти у всех компонентов есть `class` и `style` (не указаны ниже); в обработчиках пишется `on*`.

| Компонент | Назначение | Ключевые пропсы |
|---|---|---|
| `Icon`, `Logo`, `Mark`, `Loading` | значок, логотип, знак круга, загрузка | `name,size` · `height` · — · `compact,logo` |
| **layouts/** `PlainLayout` | пустой экран | `color,shell,app,dark,height` |
| `FormLayout` | экран-форма со шапкой «назад» | `title,circleTitle,subtitle,right,onright,search,compact,bar(snippet),compose,publishLabel,canPublish,publishing,oncancel,onpublish,footer(snippet),onback` |
| `CircleLayout` | экран круга: шапка, вкладки, строка ввода | `title,identity,avatar,avatarSrc,tabs,active,commentBar,commentPlaceholder,commentDraft,commentMembers,commentBusy,circleId,onback,onsearch,searchPlaceholder,searchQuery,onComment*` |
| `ShellLayout` | улочка: AppBar и FAB | `fab,fabMenuOpen,fabMenuItems,onfabmenuclose,onsearch,searchDisabled,onsettings` |
| `OverlayLayout` | лист или диалог со скримом | `variant:'sheet'\|'dialog',scrim,grip,label,ondismiss` |
| `AdminWideLayout` | широкий экран панели | `active,server,nav,cols,height` |
| **chrome/** `AppBar`, `BackBar`, `CircleBar`, `AdminBar`, `StatusBar`, `PhoneFrame` | шапки и рамка | (через макеты) |
| `AudioBar` | полоса плеера (0.18.0) | **без пропсов**, сама читает стор и маршрут |
| **data/** `Row` | базовая строка `.row2` | `onclick,link,opacity,leading,main,trailing` |
| `SettingsRow` | строка настройки | `title,subtitle,value,icon,link,chevron,onclick,control(snippet)` |
| `MemberRow` | строка участника | `initial,name,subtitle,color,src,menu,faded,onmenu,onclick` |
| `CircleRow` | строка круга на улочке | `initial,name,preview,time,badge,dot,color,card,actionLabel(2),onaction(2),onclick,touch/mouse*` |
| `ServerRow` | строка сервера | `name,subtitle,variant,card,onclick` |
| `CommentRow` | комментарий | `initial,name,color,src,queued,time(snippet),children,onedit,ondelete` |
| `CommentPreview` | превью комментариев под записью | `children,onclick` |
| `PostCard` | запись в ленте | `author,text,media,reactions,comments,headerRight (snippets),queued,onclick` |
| `MediaTile` | плитка медиа | `variant: feed\|grid\|album\|headerMini\|compose` + свои поля |
| `AttachmentRow` | файл или звук | `filename,size,onclick,audio,grouped,origin,blobId,coverUrl,meta,onDownload,preview` |
| `Avatar` | аватар | `initial,color,src` (**размера нет**) |
| `EntryDateMark` | «день привязки» справа в шапке записи | `label` (**значок `day` зашит**) |
| `EventDivider` | черта события или «выше — новое» | `text,variant` |
| `SectionLabel` | подпись секции `.lab` | `raw` |
| `DayCard`, `DayGrid`, `DayHeader`, `MonthLabel`, `PhotoGrid`, `PhotoPlaceholder` | дни, сетки | см. файлы |
| `ReactionBar`, `ReactionListRow` | реакции | `groups,keys,showAdd,pickerOpen,selectedKey,on*` · `initial,name,color,icon` |
| `FeedDayPromptCard`, `ArchiveBanner`, `PayStreetBanner` | карточки и баннеры ленты и улочки | `title,primaryLabel,…` · `title,action` · `variant` |
| `FoldHeader`, `GroupFoldCard` | группа на улочке | `label,count,expanded,…` |
| `SearchGroupHeader`, `SearchResultRow` | поиск | `color,name,count` · `author,time,preview,thumb*` |
| `MapBadge`, `MapPostSheet` | карта | `badge` · `thumbUrl,author,time,body,onclick` |
| **forms/** `Button` | кнопка `.btn` | `variant,disabled,loading,onclick` |
| `TextButton` | кнопка-ссылка | `variant(link,bar,barAction,admin,adminBox…),active,disabled,loading` |
| `IconButton` | кнопка-значок | `name,label,size,pressed,stopPropagation,onpointer*` |
| `Chip`, `ChipGroup` | чипы | `selected,disabled,onclick` |
| `Input`, `TextArea`, `SearchField`, `CodeBox` | поля | `value,active,mono,gray,admin` · `variant: area\|field\|compose\|comment` · `placeholder,autofocus` |
| `Label`, `Hint`, `ScreenTitle` | подпись поля, пояснение, заголовок экрана | — · `centered` · `centered` |
| `FieldDisplay`, `RequisitesCard` | поле только для чтения | `value,active,gray,mono,admin` |
| `Switch`, `Meter`, `ColorSwatches` | переключатель, полоса заполнения, цвета | `checked,label,onchange` · `value,max` · `value` |
| `DangerNote`, `DangerZone` | красная зона | `label,children` · `items,onitem` |
| `AddPhotoButton`, `PeopleStrip`, `InviteCard`, `MentionPicker`, `VolumeChart` | разное | `onclick,previewUrl` · `people` · … |
| **overlays/** `Sheet`, `Dialog`, `Scrim`, `Fab`, `Lightbox`, `AvatarCrop`, `CommentBar`, `PushBanner` | оверлеи | `Sheet/Dialog: label,ondismiss,grip` |
| **admin/** `AdminSection`, `AdminNav`, `CheckRow`, `StatusIcon`, `CodeBlock`, `DataTable`, `InlineInput`, `QuotaRequestRow`, `StackBar` | панель | `AdminSection: title` · `CheckRow: status,name,title,description,actions` |

---

## 1. Ручная разметка вместо существующего компонента

**1.1 Сырой `.hint` вместо `Hint` — высокая.** Компонент `Hint` рендерит ровно `<div class="hint" class:ctr>`; `centered` уже есть, детьми может быть и `TextButton`.
- `+page.svelte:138`
- `circles/[id]/+layout.svelte:278`
- `circles/[id]/+page.svelte:854`
- `circles/[id]/compose/+page.svelte:831`
- `circles/[id]/days/[date]/+page.svelte:233`
- `circles/[id]/join/+page.svelte:217,221,230`
- `circles/new/you/+page.svelte:142`
- `circles/[id]/settings/identity/+page.svelte:152,156`
- `circles/[id]/settings/invites/+page.svelte:155`

→ `<Hint centered class="mt-…">`. Подпись единицы у числового поля (`<span class="hint m-0">`) — см. 2.9.

**1.2 Сырой `.h1s` вместо `ScreenTitle` — высокая.** `ScreenTitle` = `<div class="h1s" class:ctr>`.
- `+page.svelte:126`
- `circles/[id]/+layout.svelte:277`
- `circles/[id]/+page.svelte:824`
- `circles/[id]/join/+page.svelte:226,251`
- `circles/new/you/+page.svelte:138`
- `circles/[id]/responses/+page.svelte:155`

→ `<ScreenTitle centered class="mt-48">`.

**1.3 `pay/+page.svelte:101–106` — карточка файла `.att` от руки — высокая.** Повторяет ветку `AttachmentRow` без `onclick` (`div.att > .g > имя + .sz`), только без значка и с жирным именем. → `<AttachmentRow filename=… size="… · как вложение записи" class="mx-…">`. Если нужен жирный заголовок, это проп `AttachmentRow`, а не копия.

**1.4 `invite/[token]/+page.svelte:166–169` — сервер в `FieldDisplay` двумя `div` с инлайн-стилями — высокая.** На соседних экранах тот же «сервер: имя и адрес» — это `ServerRow card`: `+page.svelte:132`, `join/+page.svelte:192`, `join/[token]/+page.svelte:100`. → `<ServerRow name={peek.server_name} subtitle={peek.host || displayHost('')} card />`.

**1.5 `admin/people/[id]/+page.svelte:144–153` — строка `.chk > .dot + .g > .n/.d` от руки — высокая.** Это разметка `CheckRow`, только вместо `StatusIcon` цветная точка. → добавить в `CheckRow` проп `leading` (snippet) или `dotColor` и использовать его. **Сделано в 0.18.28:** `CheckRow dotColor`. Попутно: общего `.dot` (14×14) из макета в `ui.css` не было — квадратик цвета круга в карточке человека был нулевым и не виден.

**1.6 `admin/people/[id]/+page.svelte:179–193` — красная зона `.danger.adm-del` с `dl/dt/dd` от руки — высокая.** Есть `DangerNote` (подпись и текст) и `DangerZone` (подпись и действия). → `DangerNote` с пропами `title` и `action` (snippet). Тогда CSS `.danger.adm-del` (ui.css:1683) больше не нужен. **Сделано в 0.18.28:** `DangerNote title` + snippet `action`, правила переименованы в `.danger.titled`; все классы компонентов теперь запрещены сторожем на голых тегах, список «только вниз» пуст.

**1.7 Время со значком: `span.tm` + flex + `Icon` от руки — высокая.** `EntryDateMark` рендерит то же самое (`.tm`, flex, gap 5 px, значок), но значок `day` в нём зашит.
- `circles/[id]/+page.svelte:634` — «в очереди», clock;
- `circles/[id]/days/[date]/+page.svelte:251` — «внесено сегодня», clock;
- `circles/[id]/posts/[postId]/+page.svelte:549` — то же без `.tm`, внутри `CommentRow time`.

→ проп `icon` у `EntryDateMark` (по умолчанию `day`) и проп отступа слева (сейчас `margin-left:auto` зашит). **Сделано в 0.18.27:** `EntryDateMark icon`; левая часть шапки — `PostByline` (`icon` у времени) в ленте, очереди, дне и на экране записи. Отступ слева оставлен зашитым: «внесено сегодня» на кадре 5.2 тоже справа — на экране дня оно стояло слева, исправлено.

**1.8 `circles/[id]/+page.svelte:732–744` — содержимое `CommentPreview` собрано вручную (`div` + `span.tm` + `div.mo`) — средняя.** Классы `.mo` и `.tm` внутри `.cm` — внутренности компонента, экран их знать не должен. → пропы `CommentPreview`: `first`, `time`, `more`. **Сделано в 0.18.27.**

**1.9 `circles/[id]/settings/identity/+page.svelte:144–150` — крупный аватар: обёртка-`div` и `style` на `Avatar` (96 px, шрифт 38 px) — средняя.** → проп `size` у `Avatar` (`sm|md|lg`). Тот же крупный аватар нужен и форме из 5.8. **Сделано в 0.18.29:** `Avatar size="lg"` (`md` по умолчанию).

**1.10 Заголовок админ-экрана `<h4 style="margin-bottom:…">` + `div.note` вместо заголовка `AdminSection` — средняя.** `AdminSection` уже рисует `<h4>` по `title`, но эти экраны зовут его без заголовка и пишут свой.
- `admin/check:95–96`, `admin/fix:79–80`
- `admin/pay/accounts/[id]:110–111`, `admin/pay/donate:71`
- `admin/pay/requests/[id]:142–143`, `admin/pay/subscription:131`
- `admin/people:63`, `admin/people/[id]:119–120`

→ пропы `title` и `subtitle` (snippet) у `AdminSection`. **Оставлено (0.18.29), причина:** в пяти экранах из восьми заголовок — почта человека или заявки, он есть только после загрузки, а `AdminSection title` рисуется до неё; у «Сбора» и «Подписки» над заголовком ссылка «← Оплата» — `title` перевернул бы порядок; у «Людей» рядом поле поиска, у «Проверки» справа колонка статуса. Общий компонент свёлся бы к обёртке над `<h4>`; inline-стиль остался один (`check`: `margin-bottom:5px`).

## 2. Повторяющиеся паттерны: кандидаты в библиотеку

**2.1 Экран «Доступ закрыт» (шлюз оплаты) скопирован целиком — высокая.** `circles/+layout.svelte:1–100` ≡ `search/+layout.svelte:1–100` (после нормализации CRLF/LF `diff` пуст). Хвост «Вы вошли как…» повторяется ещё и в `circles/+page.svelte:551–557`. → поднять шлюз в общий родительский макет группы маршрутов (`routes/(app)/+layout.svelte` над `circles` и `search`). Такой файл сторож разрешает: это `+layout.svelte`, библиотеку менять не нужно. Другой путь — компонент `PayGate` в `$ui`.

**2.2 Диалог подтверждения: заголовок, пояснение и две кнопки — высокая (6 копий, одна дословная).**
- `circles/[id]/+page.svelte:882–897`
- `circles/[id]/settings/+page.svelte:537–556`, `:560–577`
- `circles/[id]/settings/leave/+page.svelte:86–107`
- `circles/[id]/settings/members/+page.svelte:264–276`, `:280–293`

Заголовок везде `div` с `font-size:17px;font-weight:600;margin-bottom:10px` — это и есть класс `.dlgq`, но members им пользуется, а остальные пишут стиль от руки. Ряд кнопок — `.rowin` с `style="margin:18px 0 0"` и `Button style="flex:1;margin:0"` — это `.rowin.ask`. Диалог «Сначала передайте владение» в `settings:537–556` и `leave:86–107` совпадает дословно.

→ новый `$ui/overlays/ConfirmDialog.svelte` (`title`, `children`, `confirmLabel`, `cancelLabel`, `loading`, `onconfirm`, `oncancel`) или пропы `title` и `actions` у `OverlayLayout variant="dialog"`.

**2.3 Экран-состояние «заголовок + пояснение + действие» — средняя.**
- `join/[token]/+page.svelte:99–102`
- `invite/[token]/+page.svelte:134–137`
- `circles/+layout.svelte:71–73` (и копия в `search/+layout`)
- `circles/[id]/+layout.svelte:276–281`
- `circles/+page.svelte:538–543`
- `circles/[id]/+page.svelte:822–836` (`.empty` + `Mark`)
- `circles/[id]/responses/+page.svelte:155–158`

Отступы везде разные: `mt-48` / `margin-top:80px` / `28px` / `mt-24`, у пояснения — `hint-inset` / `margin:10px 30px 0` / `8px 34px 0`. → `EmptyState` (`title`, `mark?`, `children`, `actions` snippet). **Сделано в 0.18.18:** `$ui/data/EmptyState.svelte` с `place` вместо `mark`: отступы взяты из кадров, а они разные — экран-состояние (48 px, 10/30), улочка 2.3 (24 px), вкладка 3.14 (заголовок посередине, 8/34), лента 3.1 (знак, 8/34). Шесть мест; «Доступ закрыт» (10.1) на кадре выровнен влево — другой вид, остался в `PayGateLayout`. Видно: «Откликов пока нет» — посередине, как на 3.14; «Нет доступа» — 48 px вместо 80; кнопка пустой ленты — поля 60 px, как на кадре.

**2.4 Лист «Реакции» — высокая.** `circles/[id]/+page.svelte:863–879` ≡ `circles/[id]/posts/[postId]/+page.svelte:570–586`: заголовок, строки и пояснение совпадают дословно. → `ReactionsSheet` (`reactions`, `color`, `ondismiss`). **Сделано в 0.18.10:** `$ui/overlays/ReactionsSheet.svelte` в ленте и на экране записи.

**2.5 Блок вложений записи (звуки рамкой `.att-group` и файлы) — высокая.** `circles/[id]/+page.svelte:688–730` ≡ `circles/[id]/posts/[postId]/+page.svelte:414–455`: тот же цикл `attachmentBlocks`, тот же snippet `audioRow`, та же рамка `<div class="att-group">`. Разница — только источник обложек: `mediaUrls` или `audioCoverUrls`. → `AttachmentList` (или `AttachmentGroup` + `AttachmentRow`), который сам рисует рамку; подробнее в 3.2 и 5.6. **Сделано в 0.18.10:** `$ui/data/AttachmentList.svelte` (`items`, `origin`, `circleId`, `circleName`, `color`, `postId`, `coverUrls`) — рамку `.att-group` и `grouped` ставит сам; в ленте и на экране записи.

**2.6 Автор записи: `Avatar` + `div.n` + `div.tm` — средняя.**
- `circles/[id]/+page.svelte:626–637`, `:753–763`
- `circles/[id]/days/[date]/+page.svelte:282–292`
- `circles/[id]/posts/[postId]/+page.svelte:464–476`

→ пропы `authorName`, `authorTime`, `avatar*` у `PostCard`; snippet `author` оставить как запасной путь.

**2.7 Текст с упоминаниями (`splitMentionBody` → `span.men`) — средняя.**
- `circles/[id]/+page.svelte:666–669`
- `circles/[id]/days/[date]/+page.svelte:255–258`
- `circles/[id]/posts/[postId]/+page.svelte:409–411`, `533–535`, `554–556`

→ `MentionText` (`body`). **Сделано в 0.18.8:** `$ui/data/MentionText.svelte` во всех пяти местах; заодно чинит переносы строк (pre-wrap).

**2.8 Жест «потянуть, чтобы обновить» (`.ptr` / `.ptr-mark` + `Mark`) — средняя.** `circles/+page.svelte:502–508`, `circles/[id]/+page.svelte:573–579`. → `PullRefresh` (`height`, `markHeight`); стили `.ptr*` уйдут из экранного раздела `ui.css`. **Сделано в 0.18.19:** вид — `$ui/data/PullRefresh.svelte`, жест — `PullRefresh` в `$lib/gestures/pullRefresh.svelte.ts` (таймер снимается в `destroy`). Улочка, лента и — новое — «Отклики» (план 46, C8).

**2.9 Числовое поле с единицей («Своё…») — средняя.** Везде `div.rowin` + `Input` шириной 72 px + `span.hint`:
- `circles/new/+page.svelte:102–113`
- `circles/[id]/quota/request/+page.svelte:141–152`
- `circles/[id]/settings/+page.svelte:394–406`
- `circles/[id]/settings/invite/+page.svelte:233–245`, `254–266`
- `settings/app/+page.svelte:173–185`

Половина копий на служебных классах (`w72 m-0`), половина на инлайн-стилях. В админке тот же паттерн на `note`: `admin/access:220–251`, `admin/+page:330–375`, `admin/compress:85–140`. → `NumberField` (`unit`, `min`, `max`, `value`) или проп `unit` у `Input`. **Сделано в 0.18.16:** `$ui/forms/NumberField.svelte` во всех пяти местах и в `EditWindowPicker`; сырые `span.hint` ушли из экранов. Админские строки с подписью — `AdminField` (0.18.14); поле с единицей без подписи в `admin/access` и `admin/pay/donate` — на `.note`, оставлено до сверки кадров админки.

**2.10 Выбор «Окно правок» — высокая.** `circles/new/+page.svelte:77–113` ≡ `circles/[id]/settings/+page.svelte:379–407`: шесть чипов в двух `ChipGroup` и поле «часов». → `EditWindowPicker` (`value`, `customHours`, `onchange`). **Сделано в 0.18.10:** `$ui/forms/EditWindowPicker.svelte` (`value`, `bind:customHours`, `onpick`, `oncustomchange`) в «Новом круге» и настройках; подсказка о сдвиге часов остаётся в настройках.

**2.11 Фильтры поиска — средняя.** `circles/[id]/search/+page.svelte:223–240` и `search/+page.svelte:229–241`: чипы «Период / С фото / С местом» и ряд из двух дат `div style="display:flex;gap:8px;margin:8px 16px 0"` + `Input style="flex:1"`. Snippet превью совпадает дословно: `circles/[id]/search:259–268` и `search:261–270`. → `DateRange` (`from`, `to`) и проп `kind` у `SearchResultRow` вместо snippet ради «день» / «· комментарий». **Сделано в 0.18.24:** `$ui/forms/DateRange.svelte`, у `SearchResultRow` пропы `kind` / `snippet` / `query` (snippet `preview` остался для своего превью); две копии `highlight` — `quoteMatch` в `$lib/journal/search` с тестом.

**2.12 QR, ссылка, «Поделиться / Скопировать» — средняя.** `circles/[id]/settings/invite/+page.svelte:200–213` и `circles/[id]/settings/invites/+page.svelte:143–154`; QR ещё в `admin/access/+page.svelte:281–285`. QR вставляется через `{@html}` в 3 местах, `QRCode.toString` вызывается в каждом экране. → `QrCode` (`value`, `size`) и `InviteLinkCard` (`url`, `onshare`, `oncopy`, `shared`, `copied`). **Сделано в 0.18.15:** `$ui/data/QrCode.svelte` и `$ui/data/InviteLinkCard.svelte` (плюс `inSheet` для листа живой ссылки); `{@html}` в экранах больше нет, сторож запрещает его совсем.

**2.13 Экран «Кто уже здесь» — средняя.** `circles/[id]/join/+page.svelte:193–210` и `invite/[token]/+page.svelte:139–155`: `FormLayout` + `MemberRow` по `peek.members`. Можно оставить экранами, но список стоит сделать одним компонентом (`MemberList`) или общим маршрутом. **Сделано в 0.18.31:** `$ui/data/PeekMemberList.svelte` (экраны остались свои: шапки разные — с именем круга и без).

**2.14 Маленькая обложка (`.pic` + размер + `img`) — средняя.** Встречается в 5 местах:
- `AttachmentRow` → `.att-cover`
- `AudioBar` → `.audio-bar-cover` (ui.css:1220)
- `responses:190` → `.resp-pic` (ui.css:998)
- `admin/pay/subscription:188` → `.pay-thumb` (ui.css:2828)
- `map:109` → `.map-pin-thumb`, строкой HTML

У трёх правило `… img {width:100%;height:100%;object-fit:cover}` повторяет то, что уже даёт `.pic img`. → `Thumb` (`src`, `size`, `radius`). **0.18.33:** повторы `img` у `.post-ref-pic` и `.audio-bar-cover` удалены — их даёт `.pic img`. `Thumb` не заведён: три обложки с `.pic` уже внутри компонентов (`AttachmentRow`, `AudioBar`, `PostRef`), превью оплаты — голая картинка 42×28 в таблице админки, метка карты — HTML-строка для Leaflet; общий компонент экранам ничего не дал бы.

**2.15 Тонкая полоса хода или заполнения — средняя.** Пять вариантов одного и того же:
- `.att-bar i` и `.audio-bar-line i` (ui.css:1160, 1194)
- `.tile-load-bar i` (783)
- `.meter u` (Meter)
- `.qbar u` (admin/+page:415,419, ui.css:2246)

→ расширить `Meter`: `color`, `thin`, `inline`, `loading`. **Сделано в 0.18.30:** `Meter inline` и `color` — полоска квоты в таблице админки. Остальные три полосы (`.att-bar`, `.audio-bar-line`, `.tile-load-bar`) живут внутри компонентов (`AttachmentRow`, `AudioBar`, `MediaTile`) и экранам не видны — `thin`/`loading` не заводились.

**2.16 Ссылка-текст `<a class="under">` — средняя.** **Оставлено (0.18.33):** все шесть — настоящие ссылки с `href` (исходники, лицензии, «К кругам», регистрация), справочник прямо разрешает `<a class="under" href>`; кнопки-ссылки без адреса — `TextButton`. Компонент свёлся бы к переименованию тега.
- `+page:139`
- `circles/new:71`
- `circles/[id]/+layout:279`
- `join:218`
- `settings:47–48`
- `admin/+page:500–501`

`TextButton` — это `<button>`, а для перехода по `href` компонента нет. → `TextLink` (`href`).

**2.17 Скрытый `<input type="file">` с кнопкой-триггером — средняя.**
- `circles/new/you:145`, `circles/[id]/join:233`, `identity:160`
- `compose:874,882`
- `pay:109`, `join:230`, `invite:76`

Везде своя ручная логика `bind:this` → `.click()` → `input.value=''`. → `FilePicker` (`accept`, `multiple`, `capture`, `onfiles`, `el` bindable). **Сделано в 0.18.25:** `$ui/forms/FilePicker.svelte` (`open()` через `bind:this` вместо `el`) во всех семи местах, включая `IdentityForm` и `CommentBar`. Попутно: на «Я оплатил» поле не сбрасывалось при ошибке — повторный выбор того же файла молчал.

**2.18 Админ: строка «подпись · поле · единица» — средняя.**
- `admin/general:169–260` — через `const row = 'flex-mid gap-10'`, сторож этого не видит
- `admin/+page:330–375`
- `admin/compress:85–140` (`span.sz-12.w120` + `Input` + `span.note`)
- `admin/access:220–251`
- `admin/pay/donate:104–107`

→ `AdminField` (`label`, `unit`, children). **Сделано в 0.18.14 частично:** `$ui/admin/AdminField.svelte` (`label`, `width` 88 | 120, `unit`, children) — `admin/general` (9 строк; подписи стали как на кадре — 12,5 px, без капители) и `admin/compress` (5). Осталось: `admin/+page` (подпись-заголовок `.ttl.flab` 13,5 px жирная — другой вид на кадре, решить: проп или свой компонент), `admin/access` и `admin/pay/donate` (поле с единицей без подписи — ближе к `NumberField`, 2.9).

**2.19 Админ: «переключатель + заголовок + пояснение» — средняя.** `admin/pay/donate:86–102` (2 раза), `admin/pay/subscription:137–149`, `admin/people/[id]:167–177`. Разметка везде `div.flex-top.gap-12 > Switch + div > .ttl + .note.mt-4.lh-15`. → `SwitchRow` (в духе `SettingsRow` с `control`, но для админки). **Сделано в 0.18.23:** `$ui/admin/SwitchRow.svelte` во всех четырёх местах. Переключатель на 2 px ниже, как на всех трёх кадрах: раньше так было только у «Вход открыт».

**2.20 Админ: карточка `.panel` — средняя.**
- `admin/+page:434`
- `admin/compress:144` (с инлайн `padding:16px 18px`)
- `admin/pay:83`
- `admin/bootstrap:162,175,200` — через `const cardStyle = 'panel pad-16'`

→ `Panel` (`padding`). **Сделано в 0.18.14:** `$ui/admin/Panel.svelte` (`class`, `style`; отступ — служебным классом) во всех шести местах; в мастере строки классов `cardStyle`/`descStyle`/`sideLabel`/`fillInput` ушли в разметку.

**2.21 Подвал «Wynd x.y · AGPL · исходный код · лицензии» — низкая.** `settings/+page.svelte:45–49` и `admin/+page.svelte:498–502`. → `AboutFooter`. **Сделано в 0.18.31:** `$ui/data/AboutFooter.svelte` (`wrap` — две строки в настройках).

**2.22 `SettingsRow` с рамкой сверху и снизу инлайн-стилем — средняя.**
- `compose:777,801`
- `quota:149`
- `quota/deadlines:147,159`
- `archive:45,73`
- `settings:446`

→ проп `divided` (`top|bottom|both`) у `SettingsRow` или обёртка `SettingsGroup`. **Сделано в 0.18.29:** `SettingsRow divided` во всех восьми местах (и через `DateRow`); отступ сверху — служебным классом.

**2.23 Цветная точка `span.dot style="background:…"` — низкая.** `admin/+page:408`, `admin/people/[id]:145`; у `SearchGroupHeader` своя точка. → `ColorDot` (`color`) — или пропы, как в 1.5. **Оставлено (0.18.31):** в карточке человека точка ушла в `CheckRow dotColor` (0.18.28); в таблице кругов админки она одна и стилизована `.tbl .dot` — компонент ради одного места не заводится.

## 3. Экранный CSS в `ui.css`: компоненты под видом классов

**3.1 `.resp*` (ui.css:967–1024) — средняя.**
- Где используется: только `circles/[id]/responses`. Правила: `a.resp`, `a.resp + a.resp .cmt`, `.resp .cmt .acts{display:none}`, `.resp .who .tm .ic`, `.resp-ref` / `.resp-pic` / `.resp-line`, `.resp-more`.
- Что не так: экран лезет во внутренности `CommentRow` (`.cmt`, `.acts`, `.who`, `.tm`) и прячет их чужим CSS.
- Классы `.resp-list` и `.resp-rx` стоят в разметке, а правил для них нет — мёртвые.
- Что сделать: `ResponseRow` (или проп `href` у `CommentRow`, без колонки действий, когда нет `onedit/ondelete`) и `PostRef` (обложка, «автор · дата · начало текста»). `PostRef` может заменить и шапку в `MapPostSheet` / `SearchResultRow`. `.resp-more` = `.gutter` на `Button`. **Сделано в 0.18.21:** `ResponseEntry` (имя `ResponseRow` занято типом в `$lib`) и `PostRef`; у `CommentRow` проп `bare` вместо `.resp .cmt .acts{display:none}`, знак реакции — `.resp-kind` внутри компонента, черта между строками — на самих ссылках. Экран больше не лезет во внутренности `CommentRow`. Шапки `MapPostSheet` / `SearchResultRow` на `PostRef` не переведены — у них другая раскладка, решить при их сверке.

**3.2 `.att-group` + `.att.audio.grouped …` (ui.css:1255–1278) — средняя.**
- Что не так: рамку рисует экран (`div.att-group`, в 2 маршрутах), а вид строк внутри рамки — компонент через проп `grouped`. Ответственность разорвана: без рамки экрана `grouped` ломает строку (border 0, без фона).
- Что сделать: `AttachmentGroup` в `$ui`, который сам ставит рамку и `grouped` детям (см. 2.5).

**3.3 `.label-row` / `.label-aside` (ui.css:2523–2531, в блоке служебных классов) — средняя.**
- Где используется: только `circles/new/you:148–150`. `circles/[id]/join:236–240` делает то же инлайн-стилем.
- Что сделать: проп `aside` («необязательно») у `Label`. **Сделано в 0.18.29:** `Label aside`; `.label-row` теперь класс компонента (`.lab.label-row`).

**3.4 `.invite-url` (ui.css:2050–2055) — средняя.**
- Где используется: только `invites:146`. `invite:205` пишет те же три свойства инлайном.
- Что сделать: часть `InviteLinkCard` (2.12) или проп `wrap` у `FieldDisplay`.

**3.5 `.dlgq`, `.rowin.ask` (ui.css:2640–2651) — средняя.**
- Что не так: «служебные классы», которые на деле составляют диалог. Пользуются ими 2 экрана из 6 (members, invites).
- Что сделать: войдут в `ConfirmDialog` (2.2).

**3.6 `.ptr*` (671–686), `.empty` (687–696), `.feed-end*` (697–715), `.sep` (517) — средняя.**
- Где используются: только `circles/+page` и `circles/[id]/+page`.
- Что сделать: `PullRefresh` (2.8), `EmptyState` (2.3) и `FeedEnd` (знак, «Вы здесь с…» / «Здесь начинается круг»). **Сделано в 0.18.20:** все три (`PullRefresh` — 0.18.19, `EmptyState` — 0.18.18, `FeedEnd` — 0.18.20); `.feed-end*` теперь классы компонента, inline-стили низа ленты ушли.

**3.7 `.compose-bar*` (171–190), `.date-row` / `.date-pick` (1545–1550) — средняя.**
- Где используются: только `compose`. `footer`-snippet экрана повторяет панель инструментов, которая по сути принадлежит `FormLayout compose`.
- Что сделать: `ComposeToolbar` (`tools` snippet, `note`) и `DateRow` (скрытый `input type=date` + `SettingsRow`). Поле даты ещё и в `quota/deadlines:153–160` через `getElementById(...).showPicker()`. **Сделано в 0.18.22:** `$ui/chrome/ComposeToolbar.svelte` и `$ui/forms/DateRow.svelte` в «Новой записи». В «Сроках» поле даты видимое (`Input type=date`), строка лишь открывает его — другой паттерн, оставлен.

**3.8 `.qr`, `.qr.sm`, `.qr > svg`, `.qr-video` (2065–2080, 662) — средняя.**
- Что сделать: `QrCode` (2.12) и `QrScanner` для `invite/scan:75`. **Сделано:** `QrCode` — 0.18.15, `$ui/forms/QrScanner.svelte` — 0.18.32 (камера, цикл и сообщения об ошибке; что делать с прочитанным — экран).

**3.9 Классы только для админки в блоке служебных — низкая.**
- `.qbar` (2246), `.pay-thumb` (2828), `.shot` (2859), `.danger.adm-del` (1683), `.flab`, `.w110/.w120/.w180` и т. п.
- Что сделать: `Meter`-вариант, `Thumb`, `DangerNote` (1.6), `AdminField` (2.18).

**3.10 `.cbar .top .top-back` (ui.css:2532–2538) — низкая.**
- Что не так: CSS макета `FormLayout` случайно оказался в блоке служебных классов экранов.
- Что сделать: перенести в раздел «шапки».

Оставить:
- `.tile-load*` (771–795) — используется только внутри `$ui/data/MediaTile`, это CSS самого компонента;
- `.street-list`, `.feed`, `.map-wrap`, `.thread` — контейнеры прокрутки, на них завязаны правила `circle-body:has(...)` (ui.css:110).

## 4. Инлайн-стили

Итого 328 на 52 экранах. Разбивка по экранам: всего (заменяемых классами `ui.css` целиком / частично / нужен новый класс или проп / динамических).

| Экран | Всего | Целиком | Част. | Нужен новый | Динам. |
|---|---|---|---|---|---|
| `circles/[id]/settings` | 28 | 20 | 1 | 4 | 3 |
| `circles/[id]/+page` | 23 | 11 | 3 | 7 | 2 |
| `circles/[id]/join` | 11 | 10 | 1 | 0 | 0 |
| `circles/[id]/posts/[postId]` | 11 | 7 | 2 | 2 | 0 |
| `circles/[id]/search` | 11 | 10 | 1 | 0 | 0 |
| `circles/[id]/settings/invite` | 11 | 10 | 1 | 0 | 0 |
| `search/+page` | 11 | 10 | 1 | 0 | 0 |
| `circles/+page` | 10 | 1 | 1 | 6 | 2 |
| `circles/[id]/settings/identity` | 10 | 7 | 0 | 3 | 0 |
| `circles/[id]/settings/members` | 10 | 7 | 0 | 1 | 2 |
| `circles/+layout`, `search/+layout` | 9 + 9 | 9 + 9 | 0 | 0 | 0 |

Что это за стили:
- **`Hint` с отступами — около 110 штук**: `margin:16px` ×23 (= `.gutter`), `margin:24px 16px` ×11 (= `.gutter-24`), `margin-top:8/12/14/16/22/26` (= `.mt-*`), `margin:10px 30px 0` ×5 (= `.hint-inset`).
- **`Label style="margin-top:18px"` ×11 ничего не делают**: у `.lab` и так `margin:18px 16px 8px` (ui.css). Удалить без замены. — **низкая**
- **`Button style="flex:1;margin:0"` ×12 и `style="flex:1"` ×3**: внутри `.rowin`, где `margin:0` уже стоит (`.rowin .btn`). → `.rowin.ask` или `.grow-flat`. — **низкая**
- **Заголовок диалога `font-size:17px;font-weight:600;margin-bottom:10px` ×4** = `.dlgq`. — **низкая**
- **`padding-top:2px` у первой строки после подписи — 12 раз**: `SettingsRow` ×6, `MemberRow` по условию `i === 0` ×4, `invite/from:108`, `notify:106`. → одно правило `.lab + .row2 {padding-top:2px}` в `ui.css`; условные `style={i===0 ? …}` уйдут вместе с ним. — **низкая**
- **`Hint style="text-align:center"`**: `archive:85`, `invite:203`, `posts/album:124` → проп `centered`. — **низкая**
- **Стили, которые просят проп**: `SettingsRow` с `opacity:{faded?0.6:1}` (`invite/from:108`) → `faded`, как у `MemberRow`; `PhotoGrid style="padding:0 12px 16px"` в обоих альбомах (`days/[date]/album:144`, `posts/album:112`) → значение по умолчанию или `variant="album"`; `TextButton variant="bar" style="font-size:13.5px"` (`days/[date]/album:125`) → в сам вариант `bar`; `Avatar` 96 px (1.9); `IconButton style="width:100%;height:100%"` (`circles/+page:467`) → проп `fill` или часть `Fab`. — **низкая**
- **Классы в JS-переменных в обход сторожа**: `admin/general:23` `row`, `admin/bootstrap:22–23` `cardStyle` / `descStyle`. Формально это не инлайн-стиль, но по сути — те же скрытые от `check:ui` компоненты (2.18, 2.20). — **средняя**

Динамические и оправданные: высота жеста (`style:height`), цвет круга и заливка в таблице админки, фон выбранной строки. Их оставить (или передавать пропом в будущие `PullRefresh` / `Meter`).

## 5. Сделанное в 0.16–0.18 в спешке

**5.1 `circles/[id]/responses/+page.svelte:170–200` — строка отклика — высокая.**
- `<a class="resp">` оборачивает `CommentRow`, а пустую колонку действий прячет чужой CSS `.resp .cmt .acts{display:none}`. Получается строка-ссылка поверх компонента, который ссылкой быть не умеет.
- Внутри `CommentRow` экран добавляет свою рамку `.resp-ref` (`188–197`) с обложкой `.pic.resp-pic`.
- `onclick` с `preventDefault` + `goto` (`131–134`) дублирует `href`.

→ проп `href` у `CommentRow` (колонку `.acts` не рисовать, когда нет `onedit/ondelete`) или отдельный `ResponseRow`; плюс `PostRef` (3.1).

**5.2 Мелочи в `responses` — низкая.**
- Пустое состояние на сыром `.h1s` (155): см. 1.2, 2.3.
- Мёртвые классы `resp-list` (160) и `resp-rx` (181): правил для них нет, значок красит `.resp .who .tm .ic`.
- `Button class="resp-more"` (203) → `class="gutter"`.

**5.3 `chrome/AudioBar.svelte` заведён без плана пробела — высокая (процесс).**
- В `docs/plans` о нём только строка в 46-м плане (C3); плана с «Тип: пробел Wynd UI» и таблицей «Добавить в библиотеку» нет.
- Класс `audio-bar-main` при этом вписан в `BUTTON_LAYOUT_CLASSES` сторожа. Значит, сторож правили, но запрет на новый файл не сработал: он есть только в хуке Cursor (см. «Не ловит», п. 6).

→ задним числом оформить план пробела (и закрыть его). В `checkProject` добавить проверку: каждый файл `src/lib/{components,layouts}` есть в зафиксированном списке библиотеки или в открытом плане. **Сделано:** проверка — `checkLibraryRegistry` (0.18.14), план — `48-audio-bar-gap.plan.md`, закрыт (0.18.26).

**5.4 `AudioBar` — не презентационный компонент — средняя.** Остальные `$ui` управляются пропсами, а этот:
- сам импортирует `goto`, `page` из `$app/state` и стор `audioPlay`;
- решает по `pathname`, где себя прятать (`28–35`), — правило маршрутизации внутри библиотеки;
- пишет `--player-h` в `document.documentElement` (`37–42`);
- форматирует подписи (`formatBytes`, `audioTimeLabel`).

→ разделить: в `$ui` — `AudioBar` с пропсами (`title`, `subtitle`, `coverUrl`, `color`, `progress`, `loading`, `playing`, `onopen`, `ontoggle`, `onstop`); подписка, видимость и `--player-h` — в `routes/+layout.svelte` или в `$lib/media/audioBar.svelte.ts`. **Сделано в 0.18.26:** `$ui/chrome/AudioBar` — только пропы; `useAudioBar()` в `$lib/media/audioBar.svelte.ts` (подписка, правило «где прятать», подписи, `--player-h`), зовётся в корневом `+layout`.

**5.5 CSS `AudioBar` дублирует соседей — низкая.**
- `.audio-bar-cover img` повторяет `.pic img`, а класс `.pic` тянет в полосу фон-«плейсхолдер» плитки ленты.
- Полоса `.audio-bar-line` — копия `.att-bar`.

→ `Thumb` (2.14) и `Meter` (2.15). `IconButton style="color:{color}"` (89) лучше заменить пропом цвета.

**5.6 `.att-group` в `circles/[id]/+page.svelte:713–717` и `circles/[id]/posts/[postId]/+page.svelte:439–443` — без отдельного счёта, см. 2.5 и 3.2.**
- Весь блок `688–730` / `414–455` скопирован вместе со snippet `audioRow` и сборкой `meta`.
- Рамка живёт в экране, а вид строк в рамке — в `AttachmentRow grouped`.
- В ленте и на экране записи блок будет расходиться при каждой правке.

**5.7 Лист живой ссылки — `circles/[id]/settings/invites/+page.svelte:139–159` — средняя.**
- Собран из сырых `div.qr` + `{@html}` (144), `FieldDisplay class="invite-url"` (146), `div.rowin.ask` (147) и `div.hint.ctr` вокруг `TextButton` (155).
- Он же — второй экземпляр блока «QR + ссылка + Поделиться / Скопировать» из `settings/invite:200–213`. Там ссылка на инлайн-стилях вместо `.invite-url`, а кнопки — на инлайн `flex:1` вместо `.rowin.ask`.
- «Отозвать ссылку» необратимо, но срабатывает без подтверждения.

→ `InviteLinkCard` + `QrCode` (2.12), `Hint centered` (1.1), подтверждение отзыва — через `ConfirmDialog` (2.2).

**5.8 `circles/new/you/+page.svelte` — копия формы вступления `circles/[id]/join/+page.svelte` — высокая.** Совпадает около 70 строк. **Сделано в 0.18.11:** `$ui/forms/IdentityForm.svelte` (заголовок, фото с кадрированием, имя, первая запись; `bind:name`/`firstPost`/`avatar`) и `setIdentityAvatar` в `$lib/circles/settings`; оба экрана на них, `today()` → `localDayOf`, инлайн-стиль «необязательно» у join ушёл в `.label-row`.

Разметка (you ↔ join):

| you | join | что |
|---|---|---|
| 138–140 | 226–228 | заголовок «Как вас зовут в этом круге?» (сырой `.h1s`; you на `mt-22 lh-125`, join инлайн) |
| 141 | 229 | `AddPhotoButton` |
| 142–144 | 230–232 | сырой `.hint.ctr` + `TextButton` «добавить фото» |
| 145 | 233 | скрытый `input type=file` |
| 146–147 | 234–235 | `Label` «Имя» + `Input autocomplete=name` |
| 148–151 | 236–241 | подпись с «необязательно» (you — новые классы `.label-row/.label-aside`, join — тот же инлайн-стиль) |
| 152–158 | 242–248 | `TextArea` первой записи (в you другой placeholder для дневника) |
| 159–161 | 249 | `Button variant=colored` |
| 162–164 | 260–262 | ошибка `Hint` |
| 167–174 | 266–273 | `AvatarCrop` |

Скрипт (you ↔ join):

| you | join | что |
|---|---|---|
| 37–40 | 47–50 | состояние `cropFile` / `pendingAvatar` / `avatarPreview` / `fileInput` |
| 49–51 | 107–109 | `openPhotoPicker` |
| 53–63 | 111–121 | `onPhotoSelected` |
| 65–70 | 123–128 | `onCropDone` |
| 105–112 | 134–145 | загрузка аватара + `updateIdentity` |
| 81–85 | 149–153 | проверка «Введите имя» |
| 122 | 175 | `?joinAvatar=fail` |
| 132–134 | 188–190 | `onDestroy` с `revokeObjectURL` |
| 72–77 `today()` | — | повторяет `localDayOf` из `$lib/journal/present.ts:114` |

Третья частичная копия логики фото — `settings/identity/+page.svelte:144–165` (смена и удаление фото, скрытый `input`).

→ компонент `$ui/forms/IdentityForm.svelte` (`name` / `firstPost` bindable, `avatarPreview`, `firstPostLabel`, `placeholder`, `submitLabel`, `loading`, `error`, `onphoto`, `onsubmit`) и общий помощник выбора, обрезки и превью фото в `$lib/media`. Оба экрана тогда сводятся к логике отправки.

---

## Что делать — по порядку

### Заменить на существующий компонент или проп (без новых файлов)
1. Сырые `.hint` → `Hint centered` (1.1); `.h1s` → `ScreenTitle centered` (1.2). **Сделано (0.18.9):** 12 `Hint`, 7 `ScreenTitle` в 10 экранах; `span.hint` в рядах с полем оставлены — строчный элемент.
2. `pay`: `.att` → `AttachmentRow` (1.3); `invite/[token]`: `FieldDisplay`-сервер → `ServerRow card` (1.4). **Сделано (0.18.9):** `pay` — `AttachmentRow icon="photo" strong` (у `AttachmentRow` пропы `icon` и `strong`, как в макете 9.x). **1.4 оставлено:** макет 1.1 рисует сервер полем (имя и адрес, без значка), экран ему следует — `ServerRow card` разошёлся бы с макетом.
3. Диалоги на `.dlgq` + `.rowin.ask` сразу, до появления `ConfirmDialog`: это снимет около 20 инлайн-стилей (2.2, 4). **Сделано (0.18.9) сразу компонентом:** `$ui/overlays/ConfirmDialog.svelte` (`title`, `confirmLabel`, `cancelLabel`, `loading`, `onconfirm`, `oncancel`, тело — children), все шесть диалогов на нём.
4. Сделать один проход «инлайн → служебный класс» по 239 стилям, которые уже покрыты. **Сделано (0.18.9):** 174 замены скриптом с проверкой специфичности и сверкой вычисленных стилей на живых экранах, 11 пустых `Label style` убраны; бюджет 329 → 144. Оставшиеся 124: 81 без готового класса, 26 проиграли бы правилам вроде `.rowin .btn`, `.chips + .chips`, `.comp .f .inp` (их снимут компоненты из второго блока). Первыми — удалить 11 пустых `Label style="margin-top:18px"`; затем `Hint style="margin:16px"` → `gutter` и `margin:24px 16px` → `gutter-24`. Записать новые числа в бюджет.
5. `circles/+layout` ≡ `search/+layout` → один макет группы маршрутов `(app)` (2.1). **Сделано (0.18.9):** тело шлюза — `$lib/layouts/PayGateLayout.svelte`, оба `+layout` — обёртка в одну строку (без переноса маршрутов в группу).
6. Удалить мёртвые классы `resp-list` и `resp-rx`; `.resp-more` → `gutter` (5.2). **Сделано (0.18.9).**
7. Мелкие пропы у существующих компонентов (каждый — отдельная задача по библиотеке):
   - `EntryDateMark icon` (1.7)
   - `Avatar size` (1.9)
   - `AdminSection subtitle` (1.10)
   - `CheckRow leading/dotColor` (1.5)
   - `DangerNote title/action` (1.6)
   - `SettingsRow faded/divided` (2.22, 4)
   - `Label aside` (3.3)
   - `CommentPreview first/time/more` (1.8)
   - `CommentRow href` (5.1)
   - `SearchResultRow kind` (2.11)
   - `Meter color/thin/inline` (2.15)
   - `PhotoGrid` с отступом по умолчанию (4) — **сделано (0.18.32):** проп `album`

### Завести новый компонент в библиотеке (пробел Wynd UI — по плану `docs/plans/<slug>.plan.md`)
Высокий приоритет — дубли целых виджетов:
1. `IdentityForm` + помощник фото (5.8) — **сделано (0.18.11)**
2. `ConfirmDialog` (2.2) — **сделано (0.18.9)**
3. `ReactionsSheet` (2.4) — **сделано (0.18.10)**
4. `AttachmentList` / `AttachmentGroup` (2.5, 3.2) — **сделано (0.18.10):** `AttachmentList`
5. `EditWindowPicker` (2.10) — **сделано (0.18.10)**

Средний приоритет — паттерны в 2+ экранах:
- `EmptyState` (2.3), `PostRef` / `ResponseRow` (3.1), `MentionText` (2.7), `PullRefresh` (2.8), `NumberField` (2.9), `DateRange` (2.11), `QrCode` + `InviteLinkCard` (2.12), `Thumb` (2.14), `TextLink` (2.16), `FilePicker` (2.17);
- для админки: `AdminField` (2.18), `SwitchRow` (2.19), `Panel` (2.20);
- `ComposeToolbar` / `DateRow` (3.7), `FeedEnd` (3.6).

Низкий приоритет: `AboutFooter` (2.21), `ColorDot` (2.23), `QrScanner` (3.8).

Новые компоненты блока (`ConfirmDialog`, `MentionText`, `ReactionsSheet`, `AttachmentList`, `EditWindowPicker`, `IdentityForm`, макет `PayGateLayout`) записаны в справочник `docs/reference/ui-components.md` и в `catalog.ts`; формы и `MentionText` показаны в `/dev/ui` (0.18.11).

Отдельно — `AudioBar`:
- оформить план пробела задним числом;
- разделить на презентационную полосу и логику (5.3, 5.4).
— **сделано (0.18.26)**, план 48.

### Сторож (`ui-guard.mjs`) — чтобы находки не возвращались
1. Расширить `RAW_CLASS_RULES` на `div`/`span`/`a` с классами `hint`, `h1s`, `tm`, `att`, `chk`, `danger`, `qr`, `under`, `men`, `panel` в боевых экранах; запретить `{@html}` в экранах. **Сделано в 0.18.14:** `checkLibraryClasses` — классы компонентов на голых HTML-тегах боевых экранов. `LIBRARY_CLASS_BANNED` (h1s, att, men, codebox, addph, sfield, panel) запрещены совсем; оставшиеся (tm, hint, qr, chk, danger, pic) — `LIBRARY_CLASS_RATCHET`, по экранам, только вниз; `{@html}` — `HTML_TAG_SCREENS` (три QR). `a.under` с `href` — по правилу справочника, не нарушение.
2. В `checkProject` проверять, что каждый файл `src/lib/{components,layouts}/*.svelte` есть в базовом списке библиотеки или в открытом плане пробела. Сейчас этот запрет есть только в хуке Cursor. **Сделано в 0.18.14:** `checkLibraryRegistry` — файл без упоминания в `ui-components.md` и без открытого плана пробела не проходит. `AudioBar` вписан в справочник.
3. Предупреждать о `style="…"`, если такой же набор свойств уже есть у служебного класса в `ui.css` (на парсере `parseCssRules` это просто). **Оставлено (0.18.14), причина:** из 31 такого стиля 20 перебивают более сильное правило (`.rowin .btn`, `.chk .btn`, `.comp .f .inp`, `.chips + .chips`) — служебный класс той же силы проиграл бы, экран поменялся бы. Отличить такие статически без разбора каскада нельзя, а ложная тревога на каждом таком месте приучит её глушить. Безопасные 12 заменены (0.18.12), повторы `.chips + .chips` убраны.
4. Ловить строки служебных классов в `<script>` экранов (`const x = 'flex-mid gap-10'`). **Сделано в 0.18.14:** `checkScriptClassStrings` — строка из двух и больше слов, где каждое — простой класс `ui.css`. Шесть таких было в `admin/bootstrap` и `admin/general`, ушли вместе с `Panel` и `AdminField`.
5. Классы `ui.css`, которые встречаются только в одном экране, держать списком «только вниз», как `STYLE_BLOCK_SCREENS`. **Сделано в 0.18.14:** `checkSingleScreenClasses` — класс выше служебных, нужный одному экрану и ни одному компоненту; `SINGLE_SCREEN_CLASSES` — 26 нынешних, только вниз.

### Оставить и почему
- `.tile-load*`: это CSS компонента `MediaTile`, а не экрана.
- `.street-list`, `.feed`, `.map-wrap`, `.thread`: контейнеры прокрутки, на них завязаны правила `circle-body:has(...)`.
- Динамические инлайн-стили (19): высота жеста, цвет круга и заливка в таблице админки. Это данные, а не вёрстка; со временем уйдут в пропсы `PullRefresh` / `Meter`.
- `<table>` внутри `DataTable` в админке: таблица — содержимое snippet, обёртка уже библиотечная.
- `<video>` и `<img>` внутри snippet `media` у `Lightbox` (альбомы): медиа передаётся компоненту, а не рисуется вместо него.
- `circles/[id]/map`: хост-`div` Leaflet и HTML маркера строкой. Leaflet требует DOM-узел и HTML-строку; кандидат в `MapView`, но позже.
- `<style>` в 6 экранах из `STYLE_BLOCK_SCREENS`: уже учтены храповиком. `.logo-wrap` в `+page.svelte` уйдёт вместе с `EmptyState` / проп `Logo`.

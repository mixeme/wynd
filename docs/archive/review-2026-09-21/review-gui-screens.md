# Ревью GUI: боевые экраны `web/src/routes/**` (без `dev/**`)

Статус файла: В РАБОТЕ (пополняется по мере чтения экранов).
Объём: 12 064 строки в 66 файлах. Пути ниже — от `web/src/routes/`.

## Сырые цифры (собраны grep/wc)

Разбивка 9 крупнейших экранов (script / разметка / style):

| Экран | Всего | script | разметка | style |
|---|---|---|---|---|
| `circles/[id]/+page.svelte` (лента) | 726 | 437 | 289 | 0 |
| `circles/[id]/compose/+page.svelte` | 644 | 483 | 161 | 0 |
| `circles/+page.svelte` (улочка) | 569 | 324 | 245 | 0 |
| `circles/[id]/settings/+page.svelte` | 529 | 315 | 214 | 0 |
| `circles/[id]/posts/[postId]/+page.svelte` | 511 | 324 | 187 | 0 |
| `admin/+page.svelte` | 502 | 314 | 188 | 0 |
| `circles/[id]/search/+page.svelte` | 284 | 217 | 67 | 0 |
| `circles/[id]/days/[date]/+page.svelte` | 283 | 181 | 102 | 0 |
| `admin/general/+page.svelte` | 283 | 155 | 128 | 0 |

Вывод: 60–75 % крупного экрана — это script. Правило «без one-off компонентов и без fetch» действительно выдавило автоматы состояний в `+page.svelte`.

Page-level `<style>`: только 6 боевых файлов, ~80 строк (`+layout.svelte` 31+39 `@font-face` в head, `+page.svelte` 8, `admin/pay/+page` 9, `admin/pay/donate` 9, `admin/pay/requests/[id]` 19, `admin/pay/subscription` 6). `!important`, отрицательных margin, `z-index`, `calc(100vh - N)` в маршрутах НЕТ.
Настоящие «костыли компоновки» — inline `style="…"`: **578 штук в 56 боевых файлах, 206 разных значений**; 21 разное значение `margin-top` (2,3,4,6,7,8,10,12,14,16,18,20,22,24,26,28,30,34,44,48,80 px); `font-size:12.5px` ×53, `11.5px` ×23, `13.5px` ×11.
Топ повторов: `margin-top:12px` ×35, `margin:16px` ×30, `margin-top:8px` ×27, `margin:24px 16px` ×19, `flex:1` ×19, `font-size:12.5px;color:var(--muted)` ×12, `padding-top:2px` ×11, `width:88px` ×7.
Лидеры по файлам: `admin/+page` 39, `admin/compress` 33, `admin/general` 31, `circles/[id]/settings/+page` 27, `admin/bootstrap` 21, `admin/people/[id]` 19.

## Находки (черновик, по мере чтения)

### Лента `circles/[id]/+page.svelte`

1. **[high, CONFIRMED] Дубли ключей в `{#each splitMentionBody(...)}`** — `circles/[id]/+page.svelte:552`: ключ `(part.kind + part.value)`. `splitMentionBody` (`$lib/journal/mentions.ts:35-54`) возвращает части подряд; текст «@Аня и @Петя и @Аня» даёт два `mention@Аня` и два `text и `. Svelte 5 на дубль ключа бросает `each_key_duplicate` (dev) / ломает сверку списка при перерисовке после refetch (prod). Сценарий: запись с двумя одинаковыми упоминаниями или двумя одинаковыми связками между упоминаниями роняет отрисовку ленты. Решение: убрать ключ совсем (`{#each parts as part}`) — список статичен и пересоздаётся целиком; проверить то же место в `posts/[postId]` и `days/[date]`.
2. **[medium, CONFIRMED] Закрытие листа реакций пушит историю** — `:298-304`: `openReactions` и `closeReactions` оба `goto()` без `replaceState`. История: лента → `?reactions=X` → лента(новая запись). «Назад» после закрытия снова открывает лист, ещё раз «назад» — лента, и только третий — улочка. Решение: закрытие — `history.back()`, если лист открыт из ленты (флаг `openedHere`), иначе `goto(..., { replaceState: true })`.
3. **[medium, CONFIRMED] `loading` держится до скачивания всех обложек** — `:138-156`: `resolveMediaUrls` последовательно `await getMediaUrl` для каждой записи (аватар + обложка), а `loading=false` стоит в `finally` после него. `loadFeed` без пагинации (`$lib/journal/feed.ts`, 30 строк, ни `limit`, ни курсора). На 2000 записей «Загрузка…» висит, пока не пройдут до 4000 последовательных обращений к IDB/сети. Решение: `loading=false` сразу после присвоения `posts`, URL-ы догружать без await, пачками (`Promise.all` по 8) — в `$lib/media/resolveUrls.ts`.
4. **[medium, CONFIRMED] Нет защиты от двойной отправки** — `sendFromBar` `:231-268` и `pickReaction` `:351-381`: нет флага `sending`; два быстрых тапа по «отправить» = две записи (поле чистится только после ответа сервера); два тапа по реакции = set и сразу remove. Решение: `let busy = $state(false)` + ранний `return`, в `CircleLayout` передавать `commentSending`.
5. **[medium, CONFIRMED] Pull-to-refresh — самодельный автомат в экране** — `:383-407` + `:456-465`: состояние жеста хранится в `feedEl.dataset.pullStart` (DOM как хранилище), магические 80/48/69/72/280, `window.setTimeout(…, 280)` без `clearTimeout` при уходе (колбэк пишет состояние в размонтированный экран и дёргает сеть). Только touch-события. Решение: вынести в `$lib/gestures/pullToRefresh.ts` (Svelte action с `destroy`, константы именованные), экран получает только `onrefresh`.
6. **[low, CONFIRMED] «Три точки» = мгновенное удаление черновика из очереди** — `:525-531`: иконка `dots` с label «Удалить из очереди» вызывает `removeQueueItem` без подтверждения; иконка обещает меню, а не удаление. Потеря офлайн-записи с фото одним тапом. Решение: иконка `trash` + тот же диалог подтверждения, что у удаления записи.
7. **[low, CONFIRMED] Прямой `history.replaceState`** — `:170`, `:179`: в обход `replaceState` из `$app/navigation`; SvelteKit 2 предупреждает и может разойтись с собственным состоянием роутера. Решение: один хелпер `$lib/nav/url.ts: dropParams(names[])` на `replaceState` из `$app/navigation`.
8. **[low, CONFIRMED] Чистая логика в экране** — `todayEntryDate` `:429-432`, `isBackdated` `:434-436` (условие избыточно: `a !== b && a < b` ≡ `a < b`, `.slice(0,10)` лишний), `formatIsoDay` `:409-413`, `applyReaction` `:328-349`, `showReactionPlus` `:321-326`. «ещё {n} комментариев» `:590` без склонения («ещё 1 комментариев»).

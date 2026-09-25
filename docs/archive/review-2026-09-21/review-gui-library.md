# Ревью GUI-библиотеки Wynd UI (`$ui`, layouts, styles, guard, dev-маршруты)

Статус файла: В РАБОТЕ (дописывается по мере чтения).
Формат находки: **[severity | CONFIRMED/PLAUSIBLE]** путь:строка — что не так → цена → ОДНО решение.

---

## Стили и токены

### S1. `font:inherit` ПОСЛЕ `font-size`/`font-weight` в том же блоке стирает их — жирность и кегль кнопок теряются
**[high | CONFIRMED чтением каскада; визуально не проверялось — сборку запускать нельзя]**
Шортхенд `font` сбрасывает все font-* лонгхенды, объявленные раньше в том же блоке; а `button.X` (0,1,1) сильнее `.X` (0,1,0), так что «правильное» значение из `.X` не спасает.
- `web/src/lib/styles/ui.css:293` `button.btn { … font:inherit; … }` при `.btn { font-weight:600 }` (`ui.css:290`) → главная кнопка получает унаследованные 400, а не 600. `.btn.c` вес не задаёт → цветная тоже 400.
- `ui.css:220-222` `button.cm { … font-size:12.5px; … font:inherit; … }` → превью комментария 13.5px вместо 12.5px.
- `ui.css:262-264` `.rx button.one { … font-size:12.5px; … font:inherit }` → чип реакции 13.5px.
- `ui.css:274-275` `.danger button.di { … font-weight:600; … font:inherit }` → пункты DangerZone не жирные.
- `ui.css:314-315` `button.fab-menu-item { … font-weight:600; … font:inherit }` → пункты меню FAB не жирные.
- `ui.css:492-494` `button.inp { … font-size:12.5px; … font:inherit }` → админ-кнопка 13.5px.
- `ui.css:46-47` `.cbar .top.compose-top .rt { … font-weight:600; … font-size:13.5px; … font:inherit }` → «Опубликовать» теряет 600.
Правильный порядок есть рядом (`button.fold` `ui.css:137`, `.rx button.add` `:267`, `button.chip` `:281`, `button.circle-row-action` `:253`) — т.е. это не замысел, а ловушка, в которую попали 7 раз из ~35. Guard (`checkButtonCssSync`) проверяет только НАЛИЧИЕ свойств (`padding`, `width`…), не font-* и не порядок — поэтому «зелёный».
**Цена:** расхождение с макетом `screens.html` на самых частых элементах (кнопка, комментарий, реакция); каждая новая `button.X` — шанс повторить.
**Решение:** см. S2 (системный сброс убирает `font:inherit` из всех `button.X` разом). До него — точечно переставить `font:inherit` первым объявлением в семи блоках.

### S2. 35 ручных сбросов `<button>`; вёрстка `.X` продублирована в `button.X`
**[high | CONFIRMED]** `ui.css:81,86,121-138,169-176,220,246-294,314,359,373,386,492,499,536,543` + `.sw` `:342`, `.sws button` `:451`.
Счёт: `font:inherit` — 37 вхождений, `-webkit-tap-highlight-color:transparent` — 27, ~35 правил-сбросов. Полные дубли вёрстки: `.fold`/`button.fold` (134/136), `.pic`/`button.pic` (166/169, включая градиент и `#C3AF95`), `.cm`/`button.cm` (193/220), `.row2`/`button.row2` (337/246), `.r`/`button.r` (99/249), `.att`/`button.att` (230/255), `.rcho`/`button.rcho` (216/259), `.rx .one`/`.rx button.one` (188/262) + тёмная пара (190/265), `.rx .add` (191/266), `.addph`/`button.addph` (437/269), `.comp .send`/`button.send` (332/277), `.chip`/`button.chip` (301/280), `.cell`/`button.cell` (358/359), `.scrim`/`button.scrim` (385/386), `.inp`/`button.inp` (486/492), `.pay-banner`/`button.pay-reminder` (113/129). ≈16 пар, ≈70 строк чистого дубля (13% файла), плюс 130 строк guard-кода (`BUTTON_LAYOUT_SPECS`, `checkButtonCssSync`, `checkUnknownUiButtonClasses`, `ui-guard.mjs:278-456`), который существует только чтобы сторожить этот дубль.
Посылка справочника («`button.X` сильнее `.X`», `ui-components.md:99`) верна только потому, что сбросы написаны как `button.X`. UA-стили кнопки проигрывают ЛЮБОМУ авторскому правилу, т.е. `.row2{display:flex;padding:13px 16px}` и так применяется к `<button class="row2">`. Не хватает лишь border/background/font/color/text-align/width.
**Цена:** правка отступа строки = две правки; S1 — прямое следствие; div-ветка и button-ветка одного компонента расходятся молча (`.pic` без `display:block;width:100%`).
**Решение (одно):** в начало `ui.css` один сброс нулевой специфичности
`:where(button){appearance:none;border:0;background:none;padding:0;margin:0;font:inherit;color:inherit;text-align:inherit;cursor:pointer;box-sizing:border-box;-webkit-tap-highlight-color:transparent}`
и `width:100%` (+`box-sizing:border-box`) перенести в сами `.row2/.r/.fold/.att/.cm/.cell/.pic` (для `div` это no-op). После этого удалить все `button.X`-зеркала, `BUTTON_LAYOUT_SPECS`, `checkButtonCssSync`, `checkUnknownUiButtonClasses`. `:where()` = специфичность 0, любой `.X` побеждает без дублей. `all:unset` НЕ брать (снимает `display`, фокус-обводку).

### S3. Переменная `--tint` нигде не объявлена
**[low | CONFIRMED]** `ui.css:358,359,370` — `background:var(--tint)` без fallback; в `map.css:34,87` тот же var с fallback `#c3af95`. Grep `--tint:` по `web/src` — 0 объявлений. → у `.cell`/`.thumb` фон невалиден = прозрачный, пока грузится картинка (в ленте у `.pic` есть плейсхолдер `#C3AF95`, в сетке compose — нет).
**Решение:** объявить `--tint:#C3AF95` в `tokens.css :root` и убрать fallback в map.css.

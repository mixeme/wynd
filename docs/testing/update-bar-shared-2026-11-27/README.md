# Общая полоса обновления для окон — 27.11.2026

Владелец 27.11: «А почему полоса — это не общий компонент?» → «Приступай».
Полоса обновления была собрана дважды в оболочках; вынесена одна
`$ui/chrome/UpdateBar`, обе оболочки переведены, Мастерская получила ход
скачивания (одинаковый с Настольным).

## Основа

| Что | Путь | Кадр | Коммит |
| --- | --- | --- | --- |
| План-основание (пробел Wynd UI) | `docs/plans/77-update-bar.plan.md` | — | это изменение |
| Полоса Настольного | `cmd/wynd-desktop/frontend/src/App.svelte`, `app.css` | план 63 | — |
| Полоса Мастерской (A25) | `cmd/wynd-workshop/frontend/src/Window.svelte`, `app.css` | 10г–10е | `0079a519`, C151 — `75d9bbb3` |
| Ход скачивания Настольного | [desktop-update-progress-2026-11-27](../desktop-update-progress-2026-11-27/README.md) | кадр принят 27.11 | `0a09a488`, `05785b37` |
| Старый `SelfBar` (10а) | `web/src/lib/components/workshop/SelfBar.svelte` | 10а (история A25) | удалён |

## Изменение

- `$ui/chrome/UpdateBar.svelte` — общая разметка и состояния: `head`, `bad`,
  `error`, `busy`, `done`/`total` (`Meter` под заголовком, «Скачиваем N %…»),
  `attestation`, `blocked`/`hint`, `onapply`/`onhide`/`onreleases`. Место полосы,
  фон и мерки кнопки задаёт оболочка классом (`desk-update-inner` у окна,
  `selfbar` у Мастерской).
- Настольный: разметка полосы заменена компонентом; `.desk-update-head/bad/
  actions` и правило `.meter` из его `app.css` ушли.
- Мастерская: то же; `SelfBar` удалён из библиотеки и каталога `/dev/ui`;
  `internal/selfupdate.Progress` — общий тип хода для обеих оболочек;
  `main.App.SelfUpdateProgress` и опрос раз в 300 мс в окне Мастерской.
- Новый кадр 10ж: полоса Мастерской с `Meter` и «Скачиваем 46 %…» — принят
  владельцем 27.11.

## Сверка «до/после» (настоящий рендер стендов)

Настольный — мок-стенд `mock/mock.html`, 760 × 780, DPR 1; сцена `update=idle`
и `update=progress`. Мастерская — галерея `gallery.html?size=baseline`,
1024 × 760; сцены `7a-selfbar`, `10b-self`, `10c-self-bad`.

| Кадр | До (SHA-256) | После | Итог |
| --- | --- | --- | --- |
| Настольный, покой | `a1acb19664aab3de9b5b4bfc06dbd87f07ffb9934994040111599d135fc784b3` | тот же | побайтно |
| Настольный, ход | `68c6bfb54dad16fb8036ca12cfbce1bb3adfa5ecdb77820136ec91cf00055cb6` | тот же | побайтно |
| Мастерская, полоса | `f250e6f7a214034b0c5d6dd257e26d88c3b917a18390f59e2fe209346e9e26af` | тот же | побайтно |
| Мастерская, «Обновляем…» | `85f47b711ba5bd11235058bd24fd3a4fd9507067f98dd877c206dbd57e5379a0` | тот же | побайтно |
| Мастерская, отказ подписи | `2cd13bcd3cffd4649ec7336048c5fb923548f0b86cabc9e98572518599910c83` | тот же | побайтно |

Кадры «после» Мастерской — в этом каталоге; «до» — те же файлы (хеши равны).
Настольный — кадры в [протоколе хода](../desktop-update-progress-2026-11-27/README.md).

## Кадры Мастерской

| Файл | Что | SHA-256 |
| --- | --- | --- |
| [workshop-selfbar.png](workshop-selfbar.png) | полоса, покой (7a), 1024 × 760 | `f250e6f7a214034b0c5d6dd257e26d88c3b917a18390f59e2fe209346e9e26af` |
| [workshop-busy.png](workshop-busy.png) | «Обновляем…» (10b) | `85f47b711ba5bd11235058bd24fd3a4fd9507067f98dd877c206dbd57e5379a0` |
| [workshop-bad.png](workshop-bad.png) | отказ подписи (10c) | `2cd13bcd3cffd4649ec7336048c5fb923548f0b86cabc9e98572518599910c83` |
| [workshop-progress.png](workshop-progress.png) | ход скачивания (10ж), принят 27.11 | `52d84fb70b9022f9fe5f1b0d4ee3d731aef207cffbd42e309088d6c9a76f9bcf` |

Замеры 10ж: полоса 57,8 px (как в покое и «Обновляем…»), `Meter` 9 px, ширина
699 (колонка заголовка); кнопка «Скачиваем 46 %…». Высота не растёт: колонка
заголовка 30,6 px меньше ряда кнопок 36,8 px.

## Проверки

- `npm run check:ui` в `web` — OK (в т.ч. check-spacing: 19 файлов, UpdateBar).
- `npm run check` в `web`, `cmd/wynd-workshop/frontend`, `cmd/wynd-desktop/frontend` —
  svelte-check 0/0.
- `npm test` в `cmd/wynd-workshop/frontend` — 30/30.
- `go test ./internal/selfupdate ./internal/desktop ./internal/workshop`,
  `go vet` и сборка с тегами `desktop,production` — зелёные.

## Ответ владельца

Рефакторинг — по слову «Приступай». Кадр 10ж (ход скачивания в Мастерской)
**принят 27.11.2026 («Принимаем»)**. Правка в `main` (`62bac32c`); в 0.4.11 и
0.5.5 её нет. Живая проверка — реестр, §24.

## Уборка

Черновики съёмки — `dev/workshop-updatebar-capture.mjs`,
`dev/frame-desktop-update-progress.mjs` (вне репозитория); кадры-доказательства
Мастерской — в этом каталоге.

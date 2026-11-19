# Wynd UI · шапки 48 и 60 px

19 ноября 2026. Предпросмотр по итогам [ревизии](wynd-header-audit-2026-11-19.md).
Для проверки взяты шесть штатных маршрутов из `2104d0d1` с одинаковыми
синтетическими данными и изолированной сессией.

[Все шесть кадров рядом](../../dev/screens/shots/wynd-header-scale-2026-11-19/index.html).

| Кадр | Основа в `docs/visual/screens.html` (`825921cc`) | Сейчас, CSS px | Предпросмотр, CSS px |
| --- | --- | ---: | ---: |
| [Улочка](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-street.png) | `e2-1` | 48 | 48 |
| [Настройки](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-settings.png) | `e7-46` | 45,64 | 48 |
| [Новая запись](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-compose.png) | `e4-2` | 52 | 48 |
| [Общий поиск](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-search.png) | `e2-19` | 60,19 | 60,19 |
| [Обсуждение](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-circle.png) | `e4-17` | 60 | 60 |
| [Круг, вкладка «Дни»](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-circle-tabs.png) | `e5-1` | 92,56 | 92,56 |

Настольная основа редактора — `docs/plans/55-desktop-frames.svelte.txt#pc6`
из `54ecac07`. Круг с вкладками использует верхний блок 48 px
(верхний отступ 8 px и строка 40 px). Строка вкладок занимает ещё 44,56 px.
Поиск с настоящим полем имеет естественную высоту 60,19 px.

В изолированной копии `BackBar.svelte` добавлено
`style:min-height={compact ? '60px' : '48px'}` у `div.bar`.
В ветке редактора `FormLayout.svelte` вместо нижнего отступа
`<div style:height={compact ? '4px' : '12px'}></div>` использовано
`{#if !compact}<div style="height:12px"></div>{/if}`.
Компактный редактор получает 48 px, а правка, черновик из очереди и
обложка дня сохраняют 60 px. Изменения в приложение пока не включены.

Снимки получены через Vite dev, Edge Headless и CDP `Page.captureScreenshot`.
Viewport 360 × 728 CSS px, DPR 2, светлая тема, системные полосы вне кадра.
Просмотрены все варианты. Для улочки, поиска, обсуждения и круга с
вкладками исходные и новые кадры совпали попиксельно.

- Настройки. [До](../../dev/screens/shots/wynd-header-scale-2026-11-19/current-settings.png) · [48 px](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-settings.png).
- Редактор. [До](../../dev/screens/shots/wynd-header-scale-2026-11-19/current-compose.png) · [48 px](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-compose.png).

Положение и размер кнопок редактора совпали, а тело поднялось на 4 CSS px.
В настройках строка отцентрировалась в новой минимальной высоте.
Переполнения шапок при проверенной ширине нет. Числа и размеры действий
сохранены в `current.json` и `ladder.json` рядом со снимками.
Показанные варианты ждут решения владельца.

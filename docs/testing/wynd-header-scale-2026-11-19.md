# Wynd UI · шапки 48 и 60 px

19 ноября 2026. Предпросмотр по итогам [ревизии](wynd-header-audit-2026-11-19.md).
Для проверки взяты штатные маршруты из `2104d0d1` с синтетическими
данными и изолированными сессиями.

[Все семь кадров рядом](../../dev/screens/shots/wynd-header-scale-2026-11-19/index.html).

| Кадр | Основа в `docs/visual/screens.html` (`825921cc`) | Сейчас, CSS px | Предпросмотр, CSS px |
| --- | --- | ---: | ---: |
| [Улочка](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-street.png) | `e2-1` | 48 | 48 |
| [Настройки](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-settings.png) | `e7-46` | 45,64 | 48 |
| [Новая запись](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-compose.png) | `e4-2` | 52 | 48 |
| [Общий поиск](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-search.png) | `e2-19` | 60,19 | 60,19 |
| [Обсуждение](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-circle.png) | `e4-17` | 60 | 60 |
| [Круг, вкладка «Дни»](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-circle-tabs.png) | `e5-1` | 92,56 | 92,56 |
| [Альбом с датой](../../dev/screens/shots/wynd-header-scale-2026-11-19/ladder-album.png) | `e4-29` | 78,13 | 78,13 |

Настольная основа редактора — `docs/plans/55-desktop-frames.svelte.txt#pc6`
из `54ecac07`. Круг с вкладками использует верхний блок 48 px
(верхний отступ 8 px и строка 40 px). Строка вкладок занимает ещё 44,56 px.
Поиск с настоящим полем имеет естественную высоту 60,19 px.
В альбоме верхний блок тоже 48 px. Автор, время публикации и отдельная
дата записи занимают одну строку высотой 18,13 px плюс нижний отступ 12 px.
Полная высота составляет 78,13 px. Правое действие «К записи» взято из
действующего маршрута (`71d45daf`, план 61, В-2).

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
Альбом снят отдельным прогоном тех же размеров с настоящего маршрута
`/circles/[id]/posts/[postId]/album` из `2104d0d1`. Изменения лесенки
не затрагивают его `CircleBar`. Проверены стрелка назад, заголовок,
«К записи», дата с календарём и плитки. Замеры сохранены в `album.json`.
Показанные варианты ждут решения владельца.

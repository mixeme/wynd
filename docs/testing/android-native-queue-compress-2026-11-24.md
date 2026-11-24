# Android: нативная очередь 5.3 — фоновое сжатие видео (24.11.2026)

Этап 5 плана [50](../plans/50-android-wrapper.plan.md): после 5.1–5.2 native
получил **видео оригиналом** и сжимает его после закрытия окна. Сжатие фото и
звука пока остаётся в foreground; отдельного шага по ним нет — они быстрые и
уже сжимаются страницей.

## Что сделано

- **Офлайн-видео в compose.** Если приложение офлайн и мост умеет сжатие,
  страница не ждёт `compressVideo`, а кладёт оригинал в очередь и помечает
  `media_meta.native_compress` (`max_height`, `video_bitrate_kbps`). В браузере
  и при онлайне всё как раньше: сжатие в foreground.
- **Очередь.** `NativeQueue.Record` хранит флаг сжатия и высоту; `Part` после
  перекодирования меняет имя/размер/MIME, сбрасывает сессию и блоб; повторная
  попытка не перекодирует (`compressed`).
- **Сжатие.** `VideoCompressor` на Media3 Transformer 1.5.1: H.264/AAC,
  `Presentation.createForHeight(maxHeight)`, вывод в MP4 в каталоге записи.
  Оригинал заменяется только после успешного перекодирования; при ошибке в
  лог и уходит оригинал (очередь не блокируется).
- **Мост.** Операция `capabilities` отвечает `{compress:true}`; страница
  кеширует ответ. Старый APK без операции получает `false` и продолжает
  сжимать в foreground.
- **Media.** Постер и длительность по-прежнему берутся из оригинала; media
  на сервер уходит с новым именем `video.mp4`, размером и MIME.

## Ворота

- Android: `NativeQueueTest` — 9, `NativeQueueSenderTest` — 6, lint, сборка —
  зелёные. APK debug вырос с ~3,6 до ~11 МБ: Media3 Transformer тянет
  exoplayer/decoder/effect. Лицензии Apache-2.0, Google Play Services нет.
- Web: 134 файла, 649 тестов; `svelte-check` 0/0; prettier/eslint изменённых
  файлов чистые. Полный lint по-прежнему красный на чужом
  `src/lib/first-frame.svelte.test.ts`.

## Живой прогон на OPPO

OPPO CPH1931, Android 10, debug APK из текущего дерева. MP4 960×540, 25 fps,
375 кадров, 3 766 394 байта передан через `__wyndQueue` с
`native_compress.max_height=360`; стенд остановлен, приложение свёрнуто.

- Native: `TransformerInternal [AndroidXMedia3/1.5.1]`, декодер/энкодер
  `OMX.qcom.video.*.avc`, ~5 с; файл в очереди стал `video.mp4`, 958 664 байта,
  `compressed=true`; исходные 3,6 МБ заменены.
- После подъёма стенда запись ушла **при невидимой активности**: `state: sent`,
  каталог удалён; в ленте медиа `video.mp4`, 958 664 байта, `video/mp4`;
  скачанный блоб — валидный MP4 640×360, 375 кадров, 25 fps. Сжатие и отправка
  реальные, не только метаданные.
- **Границы:** живой прогон шёл через мост, а не через кнопку публикации в UI;
  путь compose → очередь → native покрыт веб-тестами и type-check. Настройка
  `video_bitrate_kbps` сохранена, но Media3 пока использует свой битрейт —
  применяется только ограничение высоты. Прогресса сжатия в интерфейсе нет;
  WorkManager без foreground-режима, длинный ролик может быть прерван и
  продолжен следующим прогоном. Авто-WorkManager после гибели процесса
  без открытия приложения остаётся открытым.

## Осталось

1. **5.4 — `.wynd`** с постоянным правом доступа.
2. **5.5 — кадр ролика в native** и снятие серверной съёмки кадра (C48).
3. Живой UI-прогон офлайн-видео из compose; прогресс/foreground для длинных
   роликов; битрейт Media3; Android 13+.
4. Если размер APK станет важен — заменить Media3 на прямой MediaCodec
   (дороже в коде, без +7 МБ).

## Повтор

```powershell
.\scripts\build-android.ps1
# стенд: cd web; npm run build; cd ..; go build -o dev/native-queue/wynd.exe ./cmd/wynd
# adb reverse tcp:7676 tcp:7676
```

CDP: `begin` с `media_meta[].native_compress`, `chunk` по 128 КиБ, `commit`;
файл очереди и каталог — `run-as mixeme.wynd.debug cat files/native-queue.json`.
Токен и base64 в отчёт не выводить.

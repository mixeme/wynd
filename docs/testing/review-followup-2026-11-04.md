# Доработка по ревью — 4 ноября 2026

План: [59](../plans/59-review-followup.plan.md). Исходный срез исправлений: `d06bb050`, чистое рабочее дерево. Отчёты [кода](../code-review-2026-11-04.md) и [безопасности](../security-audit-2026-11-04.md) остаются историческими; здесь записывается приёмка исправлений. Go-команды выполняются с `GOCACHE=dev/go-cache`.

## SEC-01

Транспорт переноса выделен в `internal/api/move_transport.go`. Разбор ссылки не допускает query/fragment и управляющие пути; URL-префикс сериализуется структурно. Каждое новое соединение разрешает DNS, отклоняет весь смешанный ответ с запрещённым IP и соединяется по проверенному IP. Сохраняется проверка TLS hostname. Системный HTTP proxy и редиректы отключены. Исключения LAN задаёт только оператор через `WYND_MOVE_ALLOWED_CIDRS`; по умолчанию список пуст. Встроенный транспорт нельзя перенастроить через API участника.

Проверки:

- `TestMoveRejectsLoopbackBeforeConnection`: прежний запрос к `/private%3F/move/...` и обычный loopback не достигают локального сервиса.
- `TestMoveAddressPolicy`: loopback/private/link-local/CGNAT/documentation/multicast, IPv4-mapped IPv6, NAT64/6to4 и зональные адреса; публичные IPv4/IPv6 и точечное LAN-исключение.
- `TestMoveDialPinsAddressesAndRejectsMixedDNS`: изменение DNS между соединениями и смешанный ответ; dial получает только проверенный IP, а не hostname.
- `TestMoveLinkPathBoundaries`, `TestMoveLANExceptionAndRedirect`, `TestMoveAllowedNetworks`: пути URL, разрешённый LAN, отказ перехода по redirect, разбор конфигурации.
- `TestIncomingMoveRejectsPrivateSourceWithoutDraft`: настоящий HTTP-обработчик отклоняет источник и не создаёт черновик.
- `go test ./internal/config ./internal/api -run 'TestMove|TestArchivePreservationMoveBetweenServers' -count=1` — успешно, включая переносы между серверами, перезапуск/докачку и A19. У тестовых локальных источников явно разрешён loopback.
- `go test ./internal/api -run '^TestIncomingMoveRejectsPrivateSourceWithoutDraft$' -count=1` — успешно.

Оригинальные отрицательные свидетельства — [протокол ревью](review-2026-11-04/README.md). До исправления loopback-сервис получал запрос; после — ни одного соединения. Проверка реального внешнего сервера не использовалась.

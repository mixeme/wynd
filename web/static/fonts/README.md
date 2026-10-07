# Golos Text

Variable-шрифт, разбитый по `unicode-range` так, как его отдаёт апстрим:

- `GolosText-Variable-cyrillic.woff2`
- `GolosText-Variable-cyrillic-ext.woff2`
- `GolosText-Variable-latin.woff2`
- `GolosText-Variable-latin-ext.woff2`

Этап 0 клиента ([client-reference.md](../../../docs/reference/client-reference.md)): `@font-face` в
`+layout.svelte`, без Google Fonts CDN — файлы лежат здесь и отдаются с того же origin.

Источник — Google Fonts (Golos Text, [SIL OFL](OFL.txt)). Полный текст лицензии — `OFL.txt` в этой папке. Файлы бинарные: `.gitattributes`
держит их вне конвертации переводов строк.

package chronicle

import (
	"errors"
	"time"
	"unicode/utf8"
)

// Text length caps. They keep a client from storing megabytes of text in
// one row; they are product constants, not admin settings (the admin
// setting governs attachment bytes, which never pass through these paths).
const (
	MaxTextBytes  = 32768 // post body, comment body, day title (UTF-8 bytes)
	MaxNameChars  = 100   // circle name, identity name
	MaxEmojiChars = 16
	// MaxClientIDChars — ключ идемпотентности очереди; клиент шлёт UUID
	// (36 знаков), запас на другие форматы. Без потолка ключ до 1 МиБ ложился
	// в колонку с уникальным индексом (аудит 2026-09-22).
	MaxClientIDChars = 64
	// MaxPostMedia — сколько вложений может нести одна запись. Клиент
	// столько и не предлагает; потолок нужен, чтобы запрос мимо него не
	// заводил тысячи строк post_media и blob_refs (аудит 2026-09-22).
	MaxPostMedia = 50
)

// ErrTooLong is returned when a text field exceeds its cap.
var ErrTooLong = errors.New("chronicle: text too long")

func checkByteLen(s string, max int) error {
	if len(s) > max {
		return ErrTooLong
	}
	return nil
}

func checkLen(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}
	return nil
}

const entryDateLayout = "2006-01-02"

// normalizeEntryDate accepts strict YYYY-MM-DD calendar dates (invalid days like 2026-02-30 are rejected).
func normalizeEntryDate(s string) (string, error) {
	if s == "" {
		return "", ErrInvalid
	}
	t, err := time.Parse(entryDateLayout, s)
	if err != nil {
		return "", ErrInvalid
	}
	out := t.Format(entryDateLayout)
	if out != s {
		return "", ErrInvalid
	}
	return out, nil
}

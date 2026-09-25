package chronicle

import (
	"errors"
	"unicode/utf8"
)

// Text length caps. They keep a client from storing megabytes of text in
// one row; they are product constants, not admin settings (the admin
// setting governs attachment bytes, which never pass through these paths).
const (
	MaxPostBodyChars    = 20000
	MaxCommentBodyChars = 2000
	MaxNameChars        = 100 // circle name, identity name
	MaxDayTitleChars    = 200
	MaxEmojiChars       = 16
)

// ErrTooLong is returned when a text field exceeds its cap.
var ErrTooLong = errors.New("chronicle: text too long")

func checkLen(s string, max int) error {
	if utf8.RuneCountInString(s) > max {
		return ErrTooLong
	}
	return nil
}

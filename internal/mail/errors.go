package mail

import (
	"errors"
	"fmt"
)

var (
	ErrNotConfigured = errors.New("mail: smtp not configured")
	ErrInvalid       = errors.New("mail: invalid")
	ErrSend          = errors.New("mail: send failed")
)

// Причины отказа отправки. Наружу уходит только эта метка: сырой ответ
// сервера почты попадал в ответ API и в панель как есть, вместе с чужими
// именами хостов и подробностями релея (аудит 2026-09-22).
const (
	ReasonDial             = "dial"
	ReasonTLS              = "tls"
	ReasonSTARTTLSRequired = "starttls_required"
	ReasonAuth             = "auth"
	ReasonProtocol         = "protocol"
)

// SendError carries a classified reason plus the raw cause for the log.
type SendError struct {
	Reason string
	Err    error
}

func (e *SendError) Error() string {
	if e.Err == nil {
		return "mail: send failed: " + e.Reason
	}
	return fmt.Sprintf("mail: send failed: %s: %v", e.Reason, e.Err)
}

// Unwrap отдаёт и ErrSend (для errors.Is у вызывающих), и исходную причину.
func (e *SendError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrSend}
	}
	return []error{ErrSend, e.Err}
}

func sendErr(reason string, err error) error {
	return &SendError{Reason: reason, Err: err}
}

func sendErrf(reason, format string, args ...any) error {
	return &SendError{Reason: reason, Err: fmt.Errorf(format, args...)}
}

// SendReason returns the classified reason of a send failure, or an empty
// string when err is not one.
func SendReason(err error) string {
	var se *SendError
	if errors.As(err, &se) {
		return se.Reason
	}
	if errors.Is(err, ErrSend) {
		return ReasonProtocol
	}
	return ""
}

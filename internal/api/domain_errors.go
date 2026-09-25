package api

import (
	"errors"
	"log"
	"net/http"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
)

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chronicle.ErrNotFound), errors.Is(err, blob.ErrNotFound), errors.Is(err, push.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	case errors.Is(err, chronicle.ErrForbidden), errors.Is(err, blob.ErrForbidden), errors.Is(err, push.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, chronicle.ErrTooLong):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too_long"})
	case errors.Is(err, chronicle.ErrInvalid), errors.Is(err, blob.ErrInvalid), errors.Is(err, push.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
	case errors.Is(err, blob.ErrQuotaExceeded):
		writeJSON(w, http.StatusInsufficientStorage, map[string]any{"error": "quota_exceeded"})
	case errors.Is(err, blob.ErrIncomplete):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "incomplete"})
	case errors.Is(err, blob.ErrExpired):
		writeJSON(w, http.StatusGone, map[string]string{"error": "expired"})
	case errors.Is(err, mail.ErrNotConfigured):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "smtp_not_configured"})
	case errors.Is(err, push.ErrNotConfigured):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "push_not_configured"})
	default:
		log.Printf("api: unmapped domain error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
	}
}

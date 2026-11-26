package api

import (
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
)

// Сессии устройств участника (план 72): список своих активных сессий и
// отзыв любой из них. Метки устройств и времена без IP и сырого UA.

type sessionItem struct {
	ID         string `json:"id"`
	Device     string `json:"device"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	ExpiresAt  string `json:"expires_at"`
	Current    bool   `json:"current"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	items, err := s.Auth.ListSessions(r.Context(), sess.AccountID, auth.SessionParticipant)
	if err != nil {
		writeError(w, err)
		return
	}
	// В списке Token — уже хеш токена из базы: открытый id это его префикс,
	// совпадающий с SessionID живого токена текущей сессии.
	currentID := auth.SessionID(sess.Token)
	out := make([]sessionItem, 0, len(items))
	for _, it := range items {
		id := it.Token[:16]
		out = append(out, sessionItem{
			ID:         id,
			Device:     it.Device,
			CreatedAt:  it.CreatedAt.UTC().Format(time.RFC3339),
			LastSeenAt: it.LastSeen.UTC().Format(time.RFC3339),
			ExpiresAt:  it.ExpiresAt.UTC().Format(time.RFC3339),
			Current:    id == currentID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, auth.ErrForbidden)
		return
	}
	id := r.PathValue("id")
	var err error
	if id == auth.SessionID(sess.Token) {
		// Своя текущая — выход: существующий отзыв по токену.
		err = s.Auth.RevokeSession(r.Context(), sess.Token)
	} else {
		err = s.Auth.RevokeSessionByID(r.Context(), sess.AccountID, auth.SessionParticipant, id)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

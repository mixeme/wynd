package api

import (
	"net/http"
	"time"
)

// Раскладка улочки (план 46, A2): закрепления и группы кругов учётки. Читаются
// вместе со списком кругов (GET /circles: pinned_at у круга, groups рядом).

type pinBody struct {
	// PinnedAt — когда закрепили на устройстве; пусто — сейчас. Нужен при
	// переносе закреплений с устройства: порядок закреплённых — по нему.
	PinnedAt *time.Time `json:"pinned_at"`
}

func (s *Server) handlePinCircle(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[pinBody](w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	at := now
	// Время с устройства — не из будущего: иначе круг стоял бы выше
	// закреплённых позже.
	if body.PinnedAt != nil && !body.PinnedAt.IsZero() && body.PinnedAt.Before(now) {
		at = body.PinnedAt.UTC()
	}
	if err := s.Chronicle.PinCircle(r.Context(), r.PathValue("circle_id"), sess.AccountID, at); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUnpinCircle(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.UnpinCircle(r.Context(), r.PathValue("circle_id"), sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type circleGroupBody struct {
	Name string `json:"name"`
}

func (s *Server) handlePutCircleGroup(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[circleGroupBody](w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.PutCircleInGroup(r.Context(), r.PathValue("circle_id"), sess.AccountID, body.Name, time.Now().UTC()); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteCircleGroup(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RemoveCircleFromGroup(r.Context(), r.PathValue("circle_id"), sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type renameGroupBody struct {
	Name    string `json:"name"`
	NewName string `json:"new_name"`
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[renameGroupBody](w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.RenameGroup(r.Context(), sess.AccountID, body.Name, body.NewName); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDropGroup(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[circleGroupBody](w, r)
	if !ok {
		return
	}
	if err := s.Chronicle.DropGroup(r.Context(), sess.AccountID, body.Name); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

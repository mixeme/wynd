package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Перенос круга по ссылке (план 53), новый сервер. Устройство отдаёт ссылку
// своему серверу; он спрашивает карточку у прежнего (из браузера туда нельзя —
// другой адрес) и после «Поднять круг здесь» забирает круг сам: телефон не
// нужен (7.20), обрыв не страшен — докачка с места обрыва, пока жива ссылка.

func peopleOrEmpty(s string) string {
	var names []string
	if json.Unmarshal([]byte(s), &names) != nil {
		return "[]"
	}
	return s
}

func incomingJSON(m *chronicle.IncomingMove, now time.Time) map[string]any {
	operationKey := m.OperationKey
	if operationKey == "" {
		operationKey = "move:" + m.ID
	}
	out := map[string]any{
		"operation_key": operationKey,
		"id":            m.ID,
		"status":        m.Status,
		"source_url":    m.SourceURL,
		"source_name":   m.SourceName,
		"expires_at":    m.ExpiresAt,
		"link_alive":    now.Before(m.ExpiresAt),
		"send_invites":  m.SendInvites,
		"pulled_bytes":  m.PulledBytes,
		// «Аня — это вы?» (A12): спрашивать, если reviver_absent.
		"owner_name":     m.OwnerName,
		"reviver_absent": m.ReviverAbsent,
		"people":         json.RawMessage(peopleOrEmpty(m.People)),
		"circle": map[string]any{
			"name": m.CircleName, "color": m.CircleColor,
			"posts": m.Posts, "members": m.Members, "bytes": m.Bytes,
		},
	}
	if m.Link != "" {
		out["link"] = m.Link
	}
	if m.FrozenAt != nil {
		out["frozen_at"] = *m.FrozenAt
	}
	if m.Error != "" {
		out["error"] = m.Error
	}
	if m.Status == chronicle.IncomingDone {
		out["result"] = map[string]any{
			"circle_id": m.CircleID, "posts": m.ResultPosts, "names": m.ResultNames, "bytes": m.ResultBytes + m.ResultTextBytes,
			"verified": m.Verified, "invited": m.Invited, "not_joined": m.NotJoined,
			"already_here": m.AlreadyHere,
		}
	}
	return out
}

// withRoom добавляет к ответу место для круга и просьбу о нём (7.21, 7.22,
// 7.23) — как у оживления из архива.
func (s *Server) withRoom(ctx context.Context, out map[string]any, accountID string, need int64) error {
	room, approved, last, err := s.reviveRoom(ctx, accountID)
	if err != nil {
		return err
	}
	if err := s.roomLimit(ctx, out, room, approved != nil); err != nil {
		return err
	}
	out["room_bytes"] = room
	out["fits"] = need <= room
	if last != nil {
		req := map[string]any{
			"status": last.Status, "requested_bytes": last.RequestedBytes,
			"circle_name": last.CircleName, "created_at": last.CreatedAt,
			"operation_key": last.OperationKey,
		}
		if last.ResolvedAt != nil {
			req["resolved_at"] = *last.ResolvedAt
		}
		req["id"] = last.ID
		out["request"] = req
	}
	return nil
}

// handlePutIncomingMove — вставили ссылку на 7.20: карточка круга с прежнего
// сервера, место здесь. Ссылка запоминается (7.22).
func (s *Server) handlePutIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		Link string `json:"link"`
	}](w, r)
	if !ok {
		return
	}
	link := strings.TrimSpace(body.Link)
	card, code := s.fetchMoveCard(r.Context(), link)
	if code != "" {
		status := http.StatusBadRequest
		if code == "source_unreachable" {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	if card.Circle == nil {
		// Перенос отменён или закончен: круга по этой ссылке больше нет.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "move_" + card.Status})
		return
	}
	if card.Circle.Bytes < 0 || card.Circle.Posts < 0 || card.Circle.Members < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "archive_invalid"})
		return
	}
	// Адрес прежнего сервера — из самой ссылки: по нему сервер и ходит.
	base, _, _ := parseMoveLink(link)
	in := chronicle.IncomingCard{
		SourceCircleID: card.Circle.ID,
		SourceURL:      base, SourceName: card.ServerName,
		CircleName: card.Circle.Name, CircleColor: card.Circle.Color,
		Posts: card.Circle.Posts, Members: card.Circle.Members, Bytes: card.Circle.Bytes,
		ExpiresAt: card.ExpiresAt,
	}
	if t, err := time.Parse(time.RFC3339, card.FrozenAt); err == nil {
		in.FrozenAt = &t
	}
	// Спросить «это вы?» — только когда карточка назвала владельца и почты,
	// и почты поднимающего среди них нет.
	if card.Circle.OwnerName != "" && len(card.Circle.Emails) > 0 {
		in.OwnerName = card.Circle.OwnerName
		mine, _ := s.Chronicle.AccountEmail(r.Context(), sess.AccountID)
		in.ReviverAbsent = !slices.ContainsFunc(card.Circle.Emails, func(e string) bool { return strings.EqualFold(e, mine) })
		if raw, err := json.Marshal(card.Circle.Names); err == nil && len(card.Circle.Names) > 0 {
			in.People = string(raw)
		}
	}
	previous, _ := s.Chronicle.LatestIncoming(r.Context(), sess.AccountID)
	m, err := s.Chronicle.PutIncomingDraft(r.Context(), sess.AccountID, link, in, time.Now())
	if errors.Is(err, chronicle.ErrMoveBusy) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "move_busy"})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if previous != nil && previous.ID != m.ID && !previous.Active() {
		s.removeIncomingFile(previous.ID)
	}
	out := incomingJSON(m, time.Now())
	if card.Status == "expired" {
		out["link_alive"] = false
	}
	if err := s.withRoom(r.Context(), out, sess.AccountID, m.Bytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleIncomingMove — последний входящий перенос этой учётки (7.20 при
// открытии, ход, итог 7.25 / 7.26) и место для него.
func (s *Server) handleIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	m, err := s.Chronicle.LatestIncoming(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if m == nil {
		out := map[string]any{"move": nil}
		if err := s.withRoom(r.Context(), out, sess.AccountID, 0); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	out := incomingJSON(m, time.Now())
	if err := s.withRoom(r.Context(), out, sess.AccountID, m.Bytes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"move": out})
}

// handleStartIncomingMove — «Поднять круг здесь» (7.20): дальше сервер
// работает сам.
func (s *Server) handleStartIncomingMove(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		SendInvites bool `json:"send_invites"`
		// NewMember — «Аня — это вы?» → «Нет» (7.29в при переносе, A12).
		NewMember *reviveNewMember `json:"new_member,omitempty"`
	}](w, r)
	if !ok {
		return
	}
	newMember := ""
	if body.NewMember != nil {
		if _, _, err := body.NewMember.parse(); err != nil {
			writeError(w, err)
			return
		}
		raw, _ := json.Marshal(body.NewMember)
		newMember = string(raw)
	}
	m, err := s.Chronicle.GetIncoming(r.Context(), r.PathValue("id"))
	if err != nil || m.AccountID != sess.AccountID {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	if !time.Now().Before(m.ExpiresAt) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "move_expired"})
		return
	}
	room, _, _, err := s.reviveRoom(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	if m.Bytes > room {
		writeJSON(w, http.StatusInsufficientStorage, map[string]string{"error": "no_room"})
		return
	}
	started, err := s.Chronicle.StartIncoming(r.Context(), m.ID, body.SendInvites, newMember, time.Now())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if started {
		s.runIncomingMove(m.ID)
	}
	m, err = s.Chronicle.GetIncoming(r.Context(), m.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incomingJSON(m, time.Now()))
}

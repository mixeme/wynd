package api

import (
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// maxInvitableEmails — сколько почт из архива можно сверить за раз: круг
// больше не бывает, а длинный список — это перебор почт сервера.
const maxInvitableEmails = 500

// handleReviveInvitable — скольким людям из архива уйдёт письмо (7.29): тем,
// чьей почты на этом сервере нет. Ноль — переключателя «Разослать
// приглашения» на экране нет: писать некому, все уже здесь (владелец,
// 2026-10-23: «Если письма не нужны, то надо было переключатель убрать»).
// Отдаётся только число, а не какие почты есть на сервере.
func (s *Server) handleReviveInvitable(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		Emails []string `json:"emails"`
	}](w, r)
	if !ok {
		return
	}
	if len(body.Emails) > maxInvitableEmails {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	own, err := s.Chronicle.AccountEmail(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	n := 0
	seen := map[string]bool{}
	for _, raw := range body.Emails {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" || seen[email] || strings.EqualFold(email, own) {
			continue
		}
		seen[email] = true
		var one int
		if err := s.Auth.DB().QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM accounts WHERE email = ? COLLATE NOCASE`, email).Scan(&one); err != nil {
			writeError(w, err)
			return
		}
		if one == 0 {
			n++
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"invitable": n})
}

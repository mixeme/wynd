package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// «Отделиться» (план 46, A9; кадры 6.41–6.45): новый круг на этом сервере из
// того, что человек видел, и личные приглашения тем, кого он позвал. Старый
// круг не меняется.

func (s *Server) handleSplitPreview(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	pre, err := s.Chronicle.SplitPreview(r.Context(), r.PathValue("circle_id"), sess.AccountID, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pre)
}

type splitBody struct {
	Name   string   `json:"name"`
	Invite []string `json:"invite"`
}

func (s *Server) handleSplit(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[splitBody](w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	res, err := s.Chronicle.SplitCircle(r.Context(), chronicle.SplitInput{
		CircleID: circleID, AccountID: sess.AccountID, Name: body.Name, Invite: body.Invite, Now: now,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Позванным — личное приглашение: строка «… зовёт вас» в списке кругов,
	// пуш и письмо. Не вышло — круг уже есть, позвать можно и потом.
	invited := []string{}
	for _, target := range res.Invited {
		if _, err := s.Auth.CreateMemberInvite(r.Context(), auth.CreateMemberInviteInput{
			CircleID: res.CircleID, TargetAccountID: target, CreatedByAccountID: sess.AccountID, Now: now,
		}); err != nil {
			log.Printf("split %s: invite %s: %v", res.CircleID, target, err)
			continue
		}
		invited = append(invited, target)
		s.notifyMemberInvited(res.CircleID, target)
	}
	s.sendSplitInviteEmails(res.CircleID, sess.AccountID, invited)
	writeJSON(w, http.StatusCreated, map[string]any{
		"circle_id": res.CircleID,
		"posts":     res.Posts,
		"invited":   len(invited),
	})
}

// sendSplitInviteEmails — письмо каждому позванному (A9, решение 3): ссылка
// ведёт на экран «позвали» (6.44).
func (s *Server) sendSplitInviteEmails(circleID, inviterAccountID string, targets []string) {
	if s.Mail == nil || len(targets) == 0 {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/circles/" + circleID + "/join"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		name, err := s.Chronicle.CircleName(ctx, circleID)
		if err != nil {
			log.Printf("split mail %s: name: %v", circleID, err)
			return
		}
		for _, target := range targets {
			info, err := s.Chronicle.SplitJoinFor(ctx, circleID, target, inviterAccountID)
			if err != nil || info == nil {
				log.Printf("split mail %s: %v", circleID, err)
				return
			}
			email, err := s.Chronicle.AccountEmail(ctx, target)
			if err != nil {
				log.Printf("split mail %s to %s: %v", circleID, target, err)
				continue
			}
			if err := s.Mail.SendSplitInvite(ctx, email, name, info.From, info.InviteLine, link); err != nil {
				log.Printf("split mail %s to %s: %v", circleID, target, err)
			}
		}
	}()
}

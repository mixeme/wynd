package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/push"
)

type notifyPrefsBody struct {
	Posts        *bool   `json:"posts"`
	CommentsMine *bool   `json:"comments_mine"`
	CommentsAll  *bool   `json:"comments_all"`
	Reactions    *bool   `json:"reactions"`
	Events       *bool   `json:"events"`
	MuteUntil    *string `json:"mute_until"`
}

// applyNotifyPrefsBody накладывает присланные поля на текущие настройки.
// Ошибка — только на mute_until: он хранится строкой, и неразобранное
// значение молча означало бы «не заглушено» (аудит 2026-09-22).
func applyNotifyPrefsBody(prefs auth.NotifyPrefs, body notifyPrefsBody) (auth.NotifyPrefs, error) {
	if body.Posts != nil {
		prefs.Posts = *body.Posts
	}
	if body.CommentsMine != nil {
		prefs.CommentsMine = *body.CommentsMine
	}
	if body.CommentsAll != nil {
		prefs.CommentsAll = *body.CommentsAll
	}
	if body.Reactions != nil {
		prefs.Reactions = *body.Reactions
	}
	if body.Events != nil {
		prefs.Events = *body.Events
	}
	if body.MuteUntil != nil {
		if *body.MuteUntil == "" {
			prefs.MuteUntil = nil
		} else {
			t, err := time.Parse(time.RFC3339, *body.MuteUntil)
			if err != nil {
				return prefs, chronicle.ErrInvalid
			}
			v := t.UTC().Format(time.RFC3339)
			prefs.MuteUntil = &v
		}
	}
	prefs.Mentions = true
	return prefs, nil
}

func (s *Server) handleGetAccountNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	prefs, err := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleSetAccountNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[notifyPrefsBody](w, r)
	if !ok {
		return
	}
	prefs, err := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	prefs, err = applyNotifyPrefsBody(prefs, body)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.SaveAccountNotifyPrefs(r.Context(), sess.AccountID, prefs); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleGetCircleNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	circleID := r.PathValue("circle_id")
	prefs, err := s.Auth.CircleNotifyPrefs(r.Context(), sess.AccountID, circleID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleSetCircleNotifyPrefs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	circleID := r.PathValue("circle_id")
	// Строка настроек заводится только для своего круга: иначе участник
	// насыпал строк по произвольным circle_id (аудит 2026-09-22).
	if err := s.Chronicle.RequireReader(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	body, ok := bindJSON[notifyPrefsBody](w, r)
	if !ok {
		return
	}
	base, err := s.Auth.AccountNotifyPrefs(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	prefs, err := s.Auth.CircleNotifyPrefs(r.Context(), sess.AccountID, circleID)
	if err != nil {
		writeError(w, err)
		return
	}
	prefs, err = applyNotifyPrefsBody(prefs, body)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.Auth.SaveCircleNotifyPrefs(r.Context(), sess.AccountID, circleID, prefs, base); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

// notifyBudget — потолок на весь цикл рассылки одного сигнала. Раньше это были
// 10 с на всех адресатов при последовательной доставке: один молчащий
// endpoint глушил уведомления остальным (аудит 2026-09-22). Срок на одну
// доставку — deliverTimeout в push.deliver.
const notifyBudget = 2 * time.Minute

// pushSignal — тот же сигнал, но заголовок уведомления это имя круга.
// Текст журнала по-прежнему не уходит в push-сервис.
func (s *Server) pushSignal(ctx context.Context, circleID, signalType string) push.Signal {
	sig := push.Signal{CircleID: circleID, Type: signalType, Count: 1}
	if s == nil || s.Chronicle == nil || circleID == "" {
		return sig
	}
	name, err := s.Chronicle.CircleName(ctx, circleID)
	if err != nil {
		log.Printf("push title %s: %v", circleID, err)
		return sig
	}
	sig.Title = name
	return sig
}

func (s *Server) notifyAccounts(circleID, actorAccountID, signalType string, accountIDs []string) {
	if s == nil || s.Push == nil || len(accountIDs) == 0 {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), notifyBudget)
		defer cancel()
		now := time.Now().UTC()
		for _, accountID := range accountIDs {
			if accountID == "" || accountID == actorAccountID {
				continue
			}
			prefs, err := s.Auth.CircleNotifyPrefs(ctx, accountID, circleID)
			if err != nil {
				log.Printf("notifyAccounts: prefs %s/%s: %v", accountID, circleID, err)
				continue
			}
			if !auth.NotifyPrefAllows(prefs, signalType, now) {
				continue
			}
			if err := s.Push.SendSignal(ctx, accountID, s.pushSignal(ctx, circleID, signalType)); err != nil {
				log.Printf("notifyAccounts: push %s/%s: %v", accountID, circleID, err)
			}
		}
	}()
}

func (s *Server) notifyMemberInvited(circleID, targetAccountID string) {
	if s == nil || s.Push == nil || targetAccountID == "" {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), notifyBudget)
		defer cancel()
		now := time.Now().UTC()
		prefs, err := s.Auth.AccountNotifyPrefs(ctx, targetAccountID)
		if err != nil {
			log.Printf("notifyMemberInvited: prefs %s: %v", targetAccountID, err)
			return
		}
		if !auth.NotifyPrefAllows(prefs, "invite", now) {
			return
		}
		if err := s.Push.SendSignal(ctx, targetAccountID, s.pushSignal(ctx, circleID, "invite")); err != nil {
			log.Printf("notifyMemberInvited: push %s: %v", targetAccountID, err)
		}
	}()
}

func (s *Server) notifyCircle(circleID, actorAccountID, signalType string) {
	if s == nil || s.Push == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), notifyBudget)
		defer cancel()
		now := time.Now().UTC()
		ids, err := s.Chronicle.CircleMemberAccountIDs(ctx, circleID)
		if err != nil {
			log.Printf("notifyCircle: members %s: %v", circleID, err)
			return
		}
		for _, accountID := range ids {
			if accountID == actorAccountID {
				continue
			}
			prefs, err := s.Auth.CircleNotifyPrefs(ctx, accountID, circleID)
			if err != nil {
				log.Printf("notifyCircle: prefs %s/%s: %v", accountID, circleID, err)
				continue
			}
			if !auth.NotifyPrefAllows(prefs, signalType, now) {
				continue
			}
			if err := s.Push.SendSignal(ctx, accountID, s.pushSignal(ctx, circleID, signalType)); err != nil {
				log.Printf("notifyCircle: push %s/%s: %v", accountID, circleID, err)
			}
		}
	}()
}

func (s *Server) notifyComment(circleID, actorAccountID, postID string) {
	if s == nil || s.Push == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), notifyBudget)
		defer cancel()
		now := time.Now().UTC()
		postAuthorID, err := s.Chronicle.PostAuthorAccountID(ctx, circleID, postID)
		if err != nil {
			log.Printf("notifyComment: author %s/%s: %v", circleID, postID, err)
			return
		}
		ids, err := s.Chronicle.CircleMemberAccountIDs(ctx, circleID)
		if err != nil {
			log.Printf("notifyComment: members %s: %v", circleID, err)
			return
		}
		for _, accountID := range ids {
			if accountID == actorAccountID {
				continue
			}
			prefs, err := s.Auth.CircleNotifyPrefs(ctx, accountID, circleID)
			if err != nil {
				log.Printf("notifyComment: prefs %s/%s: %v", accountID, circleID, err)
				continue
			}
			if !auth.NotifyCommentAllows(prefs, accountID == postAuthorID, now) {
				continue
			}
			if err := s.Push.SendSignal(ctx, accountID, s.pushSignal(ctx, circleID, "comment")); err != nil {
				log.Printf("notifyComment: push %s/%s: %v", accountID, circleID, err)
			}
		}
	}()
}

type pushSubscribeBody struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[pushSubscribeBody](w, r)
	if !ok {
		return
	}
	if err := s.Push.EnsureKeys(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	if err := s.Push.Subscribe(r.Context(), push.SubscribeInput{
		AccountID: sess.AccountID,
		Endpoint:  body.Endpoint,
		P256dh:    body.P256dh,
		Auth:      body.Auth,
		UserAgent: r.UserAgent(),
		Now:       time.Now().UTC(),
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[pushSubscribeBody](w, r)
	if !ok {
		return
	}
	if err := s.Push.Unsubscribe(r.Context(), sess.AccountID, body.Endpoint); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

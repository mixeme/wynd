package api

import (
	"context"
	"log"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// Письма и пуши участнику о решениях по оплате (2026-11-10): заявка
// утверждена или отклонена, подписка выдана без заявки. В фоне, как письма
// администратору: ответ админа не ждёт релея, сбой — строка в журнале.

// paySignal — служебный пуш с собственным текстом, как напоминание в jobs/pay.go.
func (s *Server) paySignal(ctx context.Context, accountID, title, body string) {
	if s.Push == nil || strings.TrimSpace(body) == "" {
		return
	}
	if err := s.Push.SendSignal(ctx, accountID, push.Signal{
		Type:  "subscription",
		Title: title,
		Body:  body,
	}); err != nil {
		log.Printf("pay push %s: %v", accountID, err)
	}
}

// payInstanceName — имя сервера для темы пуша и текстов писем.
func (s *Server) payInstanceName(ctx context.Context) string {
	info, err := s.Auth.Instance(ctx)
	if err != nil {
		return ""
	}
	return info.Name
}

// payTerm — срок для писем: дата или бессрочность.
func payTerm(raw string) (expires time.Time, unlimited bool) {
	t, err := xtime.Parse(raw)
	if err != nil {
		return time.Time{}, false
	}
	return t, !t.Before(auth.SubscriptionUnlimitedAt)
}

// notifyPayRequestApproved — заявка id утверждена, срок участника изменился.
func (s *Server) notifyPayRequestApproved(requestID string) {
	if s.Mail == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		req, err := s.Auth.PayRequestByID(ctx, requestID)
		if err != nil {
			log.Printf("pay approve mail %s: request: %v", requestID, err)
			return
		}
		if req.SubscriptionEnd == nil {
			return
		}
		expires, unlimited := payTerm(*req.SubscriptionEnd)
		name := s.payInstanceName(ctx)
		if err := s.Mail.SendPayRequestApproved(ctx, req.AccountEmail, name, expires, unlimited); err != nil {
			log.Printf("pay approve mail %s: %v", requestID, err)
		}
		if unlimited {
			s.paySignal(ctx, req.AccountID, name, "заявка утверждена — подписка бессрочная")
			return
		}
		s.paySignal(ctx, req.AccountID, name, "заявка утверждена — подписка до "+expires.UTC().Format("02.01.2006"))
	}()
}

// notifyPayRequestRejected — заявка id отклонена, срок не менялся.
func (s *Server) notifyPayRequestRejected(requestID string) {
	if s.Mail == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		req, err := s.Auth.PayRequestByID(ctx, requestID)
		if err != nil {
			log.Printf("pay reject mail %s: request: %v", requestID, err)
			return
		}
		name := s.payInstanceName(ctx)
		if err := s.Mail.SendPayRequestRejected(ctx, req.AccountEmail, name); err != nil {
			log.Printf("pay reject mail %s: %v", requestID, err)
		}
		s.paySignal(ctx, req.AccountID, name, "заявка об оплате отклонена")
	}()
}

// notifyPayGranted — подписка accountID продлена без заявки.
func (s *Server) notifyPayGranted(accountID string) {
	if s.Mail == nil {
		return
	}
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		acc, err := s.Auth.PayAccountByID(ctx, accountID)
		if err != nil {
			log.Printf("pay grant mail %s: account: %v", accountID, err)
			return
		}
		if acc.SubscriptionExpiresAt == nil {
			return
		}
		expires, unlimited := payTerm(*acc.SubscriptionExpiresAt)
		name := s.payInstanceName(ctx)
		if err := s.Mail.SendPayGranted(ctx, acc.Email, name, expires, unlimited); err != nil {
			log.Printf("pay grant mail %s: %v", accountID, err)
		}
		if unlimited {
			s.paySignal(ctx, accountID, name, "подписка стала бессрочной")
			return
		}
		s.paySignal(ctx, accountID, name, "подписка продлена до "+expires.UTC().Format("02.01.2006"))
	}()
}

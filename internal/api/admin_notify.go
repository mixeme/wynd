package api

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

// Письма администратору о том, что ждёт его решения (2026-10-21): просьба о
// месте и заявка об оплате. В фоне, как письма участникам: человек не ждёт
// релея, сбой — строка в журнале. Без почты администратора писем нет.

// notifyAdminQuota — просьба о месте id создана и ждёт ответа.
func (s *Server) notifyAdminQuota(id string) {
	if s.Mail == nil {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/admin#requests"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		req, err := s.Blobs.QuotaRequestByID(ctx, id)
		if err != nil || req == nil {
			log.Printf("admin quota mail %s: request: %v", id, err)
			return
		}
		name := req.CircleName
		var before sql.NullInt64
		if req.CircleID != "" {
			if name, err = s.Chronicle.CircleName(ctx, req.CircleID); err != nil {
				log.Printf("admin quota mail %s: circle: %v", id, err)
				return
			}
			if before, err = s.Blobs.CircleQuotaBytes(ctx, req.CircleID); err != nil {
				log.Printf("admin quota mail %s: quota: %v", id, err)
				return
			}
		}
		if err := s.Mail.SendAdminQuotaRequest(ctx, req.RequesterEmail, name, req.RequestedBytes, before, req.CircleID == "", link); err != nil {
			log.Printf("admin quota mail %s: %v", id, err)
		}
	}()
}

// notifyAdminPay — заявка об оплате id создана и ждёт ответа.
func (s *Server) notifyAdminPay(id string) {
	if s.Mail == nil {
		return
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/admin/pay/requests/" + id
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		req, err := s.Auth.PayRequestByID(ctx, id)
		if err != nil {
			log.Printf("admin pay mail %s: request: %v", id, err)
			return
		}
		comment := ""
		if req.Comment != nil {
			comment = *req.Comment
		}
		if err := s.Mail.SendAdminPayRequest(ctx, req.AccountEmail, comment, link); err != nil {
			log.Printf("admin pay mail %s: %v", id, err)
		}
	}()
}

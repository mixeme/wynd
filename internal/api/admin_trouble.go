package api

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/check"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// Письмо администратору о поломках сервера (план 46, A10, ч. 2): не чаще раза
// в сутки, только когда что-то не так. Беды — те, что сервер видит сам, без
// браузера: предел сервера, диск, сертификат, резервная копия, рутина.
// Новые учётки, сборка архивов и переносы кругов — не поломки.

// troubleMailEvery — как часто можно писать, пока беда не прошла. Как у
// суточной рутины: часовой цикл не сдвигает письмо на час каждый день.
const troubleMailEvery = 20 * time.Hour

// MaybeSendServerTrouble вызывается часовым циклом после суточной рутины.
func (s *Server) MaybeSendServerTrouble(ctx context.Context, now time.Time) {
	if s.Mail == nil || s.Blobs == nil {
		return
	}
	if to, err := s.Mail.AdminEmail(ctx); err != nil || to == "" {
		return
	}
	db := s.Auth.DB()
	var last sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT admin_trouble_sent_at FROM instance_settings WHERE id = 1`).Scan(&last); err != nil {
		log.Printf("trouble mail: read last: %v", err)
		return
	}
	if last.Valid && last.String != "" {
		if t, err := xtime.Parse(last.String); err == nil && now.Sub(t) < troubleMailEvery {
			return
		}
	}
	troubles := s.serverTroubles(ctx, now)
	publicURL := strings.TrimRight(s.PublicURL(), "/")
	sent, err := s.Mail.SendAdminServerTrouble(ctx, publicHostname(publicURL), publicURL+"/admin/check", troubles, now)
	if err != nil {
		log.Printf("trouble mail: %v", err)
		return
	}
	if !sent {
		return
	}
	if _, err := db.ExecContext(ctx, `UPDATE instance_settings SET admin_trouble_sent_at = ? WHERE id = 1`, xtime.Format(now)); err != nil {
		log.Printf("trouble mail: save sent: %v", err)
	}
	log.Printf("trouble mail: %d troubles", len(troubles))
}

func (s *Server) serverTroubles(ctx context.Context, now time.Time) []check.Trouble {
	in := check.TroubleInput{Now: now}
	in.UsedBytes, _ = s.Blobs.UsedBytes(ctx)
	in.LimitBytes, _ = s.Blobs.InstanceQuotaBytes(ctx)
	if s.DataDir != "" {
		if free, total, err := check.DiskUsage(s.DataDir); err == nil {
			in.DiskFree, in.DiskTotal = free, total
		}
	}
	in.LastRoutineAt, in.LastBackupAt = s.loadInstanceTimestamps(ctx)
	publicURL := s.PublicURL()
	if !s.Loopback() && strings.HasPrefix(publicURL, "https://") {
		probeCtx, cancel := context.WithTimeout(ctx, adminCheckTimeout)
		in.TLS = check.ProbeTLS(probeCtx, publicURL)
		cancel()
	}
	return check.Troubles(in)
}

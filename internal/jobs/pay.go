package jobs

import (
	"context"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/xtime"
)

// PayJobCounts reports pay maintenance actions.
type PayJobCounts struct {
	ScreenshotsDeleted int
	RemindersSent      int
}

// RunPayJobs cleans expired pay screenshots and sends subscription reminders.
func RunPayJobs(ctx context.Context, authSvc *auth.Service, mailSvc *mail.Service, pushSvc *push.Service, blobsDir string, now time.Time) (PayJobCounts, error) {
	if authSvc == nil {
		return PayJobCounts{}, nil
	}
	now = now.UTC()
	var counts PayJobCounts
	deleted, err := authSvc.CleanupExpiredPayScreenshots(ctx, blobsDir, now)
	if err != nil {
		return counts, err
	}
	counts.ScreenshotsDeleted = deleted

	info, err := authSvc.Instance(ctx)
	if err != nil {
		return counts, err
	}
	candidates, err := authSvc.ListPayReminderCandidates(ctx, now)
	if err != nil {
		return counts, err
	}
	for _, c := range candidates {
		expires, err := xtime.Parse(c.ExpiresAt)
		if err != nil {
			continue
		}
		sent := false
		if mailSvc != nil {
			if err := mailSvc.SendSubscriptionReminder(ctx, c.Email, info.Name, expires, c.DaysLeft); err == nil {
				sent = true
			}
		}
		if pushSvc != nil {
			if err := pushSvc.SendSignal(ctx, c.AccountID, push.Signal{
				Type:  "subscription",
				Title: info.Name,
				Body:  "подписка до " + expires.Format("02.01.2006"),
			}); err == nil {
				sent = true
			}
		}
		if !sent {
			continue
		}
		if err := authSvc.MarkPayReminderSent(ctx, c.AccountID, c.ExpiresAt); err != nil {
			return counts, err
		}
		counts.RemindersSent++
	}
	return counts, nil
}

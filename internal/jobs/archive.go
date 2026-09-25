package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
)

// ArchiveJobCounts reports archive maintenance results.
type ArchiveJobCounts struct {
	PurgedCircles int
	RemindersSent int
}

// archiveReminderSender отправляет одно письмо-напоминание; nil — почты нет.
type archiveReminderSender func(ctx context.Context, email, cutoffDate string, deadline time.Time, downloadURL string) error

// RunArchiveJobs purges expired archive cycles and sends reminder emails.
func RunArchiveJobs(ctx context.Context, db *sql.DB, ch *chronicle.Chronicle, blobs *blob.Store, mailSvc *mail.Service, publicURL string, now time.Time) (ArchiveJobCounts, error) {
	var send archiveReminderSender
	if mailSvc != nil {
		send = mailSvc.SendArchiveReminder
	}
	return runArchiveJobs(ctx, db, ch, blobs, send, publicURL, now)
}

// runArchiveJobs — тело RunArchiveJobs с подменяемой отправкой.
//
// Круги независимы: сбой purge или письма одного круга пишется в ошибку и не
// останавливает остальные. Письма уходят всем адресатам круга; напоминание
// считается отправленным, если ушло хотя бы одно. Раньше первый же отказ
// прерывал рассылку без отметки, и через час те, кому письмо уже ушло,
// получали его снова — и так каждый час до срока (план 42, ARC-5).
func runArchiveJobs(ctx context.Context, db *sql.DB, ch *chronicle.Chronicle, blobs *blob.Store, send archiveReminderSender, publicURL string, now time.Time) (ArchiveJobCounts, error) {
	if db == nil || ch == nil {
		return ArchiveJobCounts{}, fmt.Errorf("jobs: nil dependency")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var counts ArchiveJobCounts
	var errs []error

	due, err := ch.CirclesDueForArchivePurge(ctx, now)
	if err != nil {
		errs = append(errs, err)
	}
	for _, circleID := range due {
		cycle, err := ch.GetArchiveCycle(ctx, circleID)
		if err != nil || !cycle.Active {
			continue
		}
		blobIDs, err := ch.PurgeBeforeCutoff(ctx, circleID, cycle.CutoffDate, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("purge circle %s: %w", circleID, err))
			continue
		}
		if blobs != nil && len(blobIDs) > 0 {
			if err := blobs.ReleaseBlobs(ctx, blobIDs); err != nil {
				errs = append(errs, fmt.Errorf("release blobs of circle %s: %w", circleID, err))
			}
		}
		counts.PurgedCircles++
	}

	remind, err := ch.CirclesDueForArchiveReminder(ctx, now)
	if err != nil {
		errs = append(errs, err)
	}
	base := strings.TrimRight(publicURL, "/")
	for _, circleID := range remind {
		cycle, err := ch.GetArchiveCycle(ctx, circleID)
		if err != nil || !cycle.Active {
			continue
		}
		emails, err := ch.CircleMemberEmails(ctx, circleID)
		if err != nil {
			errs = append(errs, fmt.Errorf("archive reminder circle %s: %w", circleID, err))
			continue
		}
		download := base + "/api/v1/circles/" + circleID + "/archive/download"
		if send != nil {
			sent := 0
			for _, email := range emails {
				if err := send(ctx, email, cycle.CutoffDate, cycle.Deadline, download); err != nil {
					errs = append(errs, fmt.Errorf("archive reminder circle %s to %s: %w", circleID, email, err))
					continue
				}
				sent++
			}
			if sent == 0 && len(emails) > 0 {
				continue
			}
		}
		if err := ch.MarkArchiveReminderSent(ctx, circleID, now); err != nil {
			errs = append(errs, err)
			continue
		}
		counts.RemindersSent++
	}
	return counts, errors.Join(errs...)
}

package jobs

import (
	"context"
	"database/sql"
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

// RunArchiveJobs purges expired archive cycles and sends reminder emails.
func RunArchiveJobs(ctx context.Context, db *sql.DB, ch *chronicle.Chronicle, blobs *blob.Store, mailSvc *mail.Service, publicURL string, now time.Time) (ArchiveJobCounts, error) {
	if db == nil || ch == nil {
		return ArchiveJobCounts{}, fmt.Errorf("jobs: nil dependency")
	}
	now = now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var counts ArchiveJobCounts

	due, err := ch.CirclesDueForArchivePurge(ctx, now)
	if err != nil {
		return counts, err
	}
	for _, circleID := range due {
		cycle, err := ch.GetArchiveCycle(ctx, circleID)
		if err != nil || !cycle.Active {
			continue
		}
		blobIDs, err := ch.PurgeBeforeCutoff(ctx, circleID, cycle.CutoffDate, now)
		if err != nil {
			return counts, fmt.Errorf("purge circle %s: %w", circleID, err)
		}
		if blobs != nil && len(blobIDs) > 0 {
			if err := blobs.ReleaseBlobs(ctx, blobIDs); err != nil {
				return counts, err
			}
		}
		counts.PurgedCircles++
	}

	remind, err := ch.CirclesDueForArchiveReminder(ctx, now)
	if err != nil {
		return counts, err
	}
	base := strings.TrimRight(publicURL, "/")
	for _, circleID := range remind {
		cycle, err := ch.GetArchiveCycle(ctx, circleID)
		if err != nil || !cycle.Active {
			continue
		}
		emails, err := ch.CircleMemberEmails(ctx, circleID)
		if err != nil {
			return counts, err
		}
		download := base + "/api/v1/circles/" + circleID + "/archive/download"
		if mailSvc != nil {
			for _, email := range emails {
				_ = mailSvc.SendArchiveReminder(ctx, email, cycle.CutoffDate, cycle.Deadline, download)
			}
		}
		if err := ch.MarkArchiveReminderSent(ctx, circleID, now); err != nil {
			return counts, err
		}
		counts.RemindersSent++
	}
	return counts, nil
}

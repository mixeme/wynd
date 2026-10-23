package jobs

import (
	"context"
	"database/sql"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// RunArchiveJobsWithSender открывает тестам подмену отправки напоминаний.
func RunArchiveJobsWithSender(ctx context.Context, db *sql.DB, ch *chronicle.Chronicle, send func(ctx context.Context, email, circle, cutoffDate string, deadline time.Time, link string) error, publicURL string, now time.Time) (ArchiveJobCounts, error) {
	return runArchiveJobs(ctx, db, ch, nil, send, publicURL, now)
}

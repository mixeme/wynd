package chronicle_test

import (
	"database/sql"
	"errors"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"testing"
)

// Отзыв и расход одобрения не могут оба завершиться успешно.
func TestRevivePlaceWithdrawalRace(t *testing.T) {
	for _, first := range []string{"withdraw", "revive", "concurrent"} {
		t.Run(first, func(t *testing.T) {
			e := newTestEnv(t)
			e.seedAccount("owner")
			circle := e.createCircle("owner", "Кот", chronicle.UnlimitedWindow())
			data, err := e.ch.FullArchiveData(e.ctx, circle.ID)
			if err != nil {
				t.Fatal(err)
			}
			tables, accounts, blobs := archiveOverJSON(t, data)
			_, err = e.ch.DB().ExecContext(e.ctx, `INSERT INTO quota_requests
				(id, circle_name, requested_by_account_id, requested_bytes, status, created_at)
				VALUES ('place', 'Семья', 'owner', 123456, 'approved', '2026-11-02T12:00:00Z')`)
			if err != nil {
				t.Fatal(err)
			}
			var result chronicle.ReviveResult
			var reviveErr, withdrawErr error
			var withdrawn int64
			revive := func() {
				result, reviveErr = e.ch.ReviveCircle(e.ctx, chronicle.ReviveInput{
					FormatVersion: 2, SchemaVersion: 12, Tables: tables, Accounts: accounts, BlobMap: blobs,
					ReviverID: "owner", PlaceRequestID: "place", Now: e.at(2),
				})
			}
			withdraw := func() {
				var changed sql.Result
				changed, withdrawErr = e.ch.DB().ExecContext(e.ctx, `UPDATE quota_requests SET withdrawn_at = '2026-11-02T13:00:00Z'
					WHERE id = 'place' AND used_at IS NULL`)
				if withdrawErr == nil {
					withdrawn, withdrawErr = changed.RowsAffected()
				}
			}
			switch first {
			case "withdraw":
				withdraw()
				revive()
			case "revive":
				revive()
				withdraw()
			default:
				finished := make(chan struct{})
				go func() { defer close(finished); revive() }()
				withdraw()
				<-finished
			}
			if withdrawErr != nil {
				t.Fatal(withdrawErr)
			}
			var circles int
			if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT COUNT(*) FROM circles`).Scan(&circles); err != nil {
				t.Fatal(err)
			}
			if withdrawn == 1 {
				if !errors.Is(reviveErr, chronicle.ErrInvalid) || circles != 1 {
					t.Fatalf("revived withdrawn request: %v; circles=%d", reviveErr, circles)
				}
			} else {
				if reviveErr != nil || circles != 2 {
					t.Fatalf("revival: %v; circles=%d", reviveErr, circles)
				}
				var quota int64
				var used sql.NullString
				if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT quota_bytes FROM circles WHERE id = ?`, result.CircleID).Scan(&quota); err != nil {
					t.Fatal(err)
				}
				if err := e.ch.DB().QueryRowContext(e.ctx, `SELECT used_at FROM quota_requests WHERE id = 'place'`).Scan(&used); err != nil {
					t.Fatal(err)
				}
				if quota != 123456 || !used.Valid {
					t.Fatalf("quota=%d; used=%v", quota, used)
				}
			}
		})
	}
}

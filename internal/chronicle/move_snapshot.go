package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// MoveSnapshot holds immutable source rows until the ZIP index is published.
// The index then replaces these rows; neither form stores another media copy.
type MoveSnapshot struct {
	ID            string
	At            time.Time
	SchemaVersion int
	WyndVersion   string
	Card          json.RawMessage
	Data          *FullArchiveData
	Layout        []byte
	Size          int64
	Posts, Names  int
	MediaBytes    int64
}

func readMoveSnapshot(ctx context.Context, q archiveReader, id string) (*MoveSnapshot, error) {
	var out MoveSnapshot
	var at string
	var data []byte
	err := q.QueryRowContext(ctx, `SELECT id,snapshot_at,schema_version,wynd_version,data,layout,archive_size,posts,names,media_bytes,card FROM move_snapshots WHERE id=?`, id).
		Scan(&out.ID, &at, &out.SchemaVersion, &out.WyndVersion, &data, &out.Layout, &out.Size, &out.Posts, &out.Names, &out.MediaBytes, &out.Card)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out.At = parseTimeOrZero(at)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &out.Data); err != nil {
			return nil, err
		}
	}
	return &out, nil
}

// MoveSnapshot loads an already frozen source, including after a restart.
func (c *Chronicle) MoveSnapshot(ctx context.Context, id string) (*MoveSnapshot, error) {
	return readMoveSnapshot(ctx, c.db, id)
}

// CaptureMoveSnapshot freezes all physical journal rows and pins their files in
// the same write transaction. Existing snapshots are always reused, never reread.
func (c *Chronicle) CaptureMoveSnapshot(ctx context.Context, moveID, wyndVersion string) (*MoveSnapshot, error) {
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	m, err := scanCircleMove(tx.QueryRowContext(ctx, `SELECT `+circleMoveCols+` FROM circle_moves WHERE id=?`, moveID))
	if err != nil {
		return nil, err
	}
	if !m.LinkAlive(time.Now()) {
		return nil, ErrMoveExpired
	}
	if m.SnapshotID != "" {
		return readMoveSnapshot(ctx, tx, m.SnapshotID)
	}
	data, err := c.fullArchiveDataTx(ctx, tx, m.CircleID)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	out := &MoveSnapshot{ID: id, At: data.SnapshotAt, Data: data, WyndVersion: wyndVersion, Posts: data.Posts}
	if err := tx.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&out.SchemaVersion); err != nil {
		return nil, err
	}
	for _, t := range data.Tables {
		switch t.Name {
		case "identities":
			out.Names = len(t.Rows)
		case "blobs":
			for i := range t.Rows {
				out.MediaBytes += asInt64(rowMap(t, i)["size_bytes"])
			}
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	owner, emails, names, err := c.circleMoveFolk(ctx, tx, m.CircleID)
	if err != nil {
		return nil, err
	}
	var text int64
	var members int
	for _, table := range data.Tables {
		switch table.Name {
		case "circles":
			if len(table.Rows) > 0 {
				text = asInt64(rowMap(table, 0)["text_bytes"])
			}
		case "memberships":
			for i := range table.Rows {
				if rowMap(table, i)["status"] != "gone" {
					members++
				}
			}
		}
	}
	out.Card, err = json.Marshal(map[string]any{"id": m.CircleID, "name": data.CircleName, "color": data.Color, "posts": out.Posts, "members": members, "bytes": out.MediaBytes + text, "owner_name": owner, "emails": emails, "names": names})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO move_snapshots(id,circle_id,snapshot_at,schema_version,wynd_version,data,posts,names,media_bytes,card) VALUES (?,?,?,?,?,?,?,?,?,?)`, id, m.CircleID, formatTime(out.At), out.SchemaVersion, wyndVersion, raw, out.Posts, out.Names, out.MediaBytes, out.Card); err != nil {
		return nil, err
	}
	for _, blobID := range data.BlobIDs {
		result, err := tx.ExecContext(ctx, `INSERT INTO move_snapshot_blobs(snapshot_id,blob_id) SELECT ?,id FROM blobs WHERE id=? AND status='complete'`, id, blobID)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, ErrNotFound
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE circle_moves SET snapshot_id=? WHERE id=?`, id, moveID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

// PublishMoveLayout commits one exact ZIP representation. Concurrent builders
// reuse the first published index and its length, even across link reissues.
// MoveLiteral is an opaque interval of the immutable ZIP metadata.
type MoveLiteral struct {
	Part int
	Data []byte
}

func (c *Chronicle) PublishMoveLayout(ctx context.Context, snapshotID string, layout []byte, size int64, literals ...MoveLiteral) error {
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(layout) > 0 {
		result, err := tx.ExecContext(ctx, `UPDATE move_snapshots SET layout=?,data=NULL,archive_size=? WHERE id=? AND layout IS NULL`, layout, size, snapshotID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			for _, literal := range literals {
				if _, err := tx.ExecContext(ctx, `INSERT INTO move_snapshot_literals(snapshot_id,part,data) VALUES (?,?,?)`, snapshotID, literal.Part, literal.Data); err != nil {
					return err
				}
			}
		}
	}
	var actual int64
	var posts, names int
	var media int64
	err = tx.QueryRowContext(ctx, `SELECT archive_size,posts,names,media_bytes FROM move_snapshots WHERE id=? AND layout IS NOT NULL`, snapshotID).Scan(&actual, &posts, &names, &media)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE circle_moves SET archive_status='ready',archive_bytes=?,archive_posts=?,archive_names=?,archive_media_bytes=?
		WHERE snapshot_id=? AND status IN ('waiting','pulling')`, actual, posts, names, media, snapshotID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ReadMoveLiteral reads only the requested bytes, not the entire text snapshot.
func (c *Chronicle) ReadMoveLiteral(ctx context.Context, snapshotID string, part int, offset int64, p []byte) (int, error) {
	if offset < 0 {
		return 0, ErrInvalid
	}
	if len(p) == 0 {
		return 0, nil
	}
	var data []byte
	err := c.db.QueryRowContext(ctx, `SELECT substr(data,?+1,?) FROM move_snapshot_literals WHERE snapshot_id=? AND part=?`, offset, len(p), snapshotID, part).Scan(&data)
	if err != nil {
		return 0, err
	}
	n := copy(p, data)
	if n != len(p) {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

// ReleaseMoveSnapshot detaches one ended link; another live/reissued link keeps
// the same snapshot and pins. Returned blobs can be garbage collected afterwards.
func (c *Chronicle) ReleaseMoveSnapshot(ctx context.Context, moveID string) ([]string, string, error) {
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	var legacy sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT legacy_archive_id FROM circle_moves WHERE id=?`, moveID).Scan(&legacy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE circle_moves SET snapshot_id=NULL,legacy_archive_id=NULL,archive_status='none',archive_bytes=0 WHERE id=?`, moveID); err != nil {
		return nil, "", err
	}
	ids, err := pruneMoveSnapshots(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return ids, legacy.String, nil
}

func pruneMoveSnapshots(ctx context.Context, tx *sql.Tx) ([]string, error) {
	predicate := `NOT EXISTS (SELECT 1 FROM circle_moves m WHERE m.snapshot_id=move_snapshots.id AND m.status IN ('waiting','pulling'))`
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT blob_id FROM move_snapshot_blobs WHERE snapshot_id IN (SELECT id FROM move_snapshots WHERE `+predicate+`)`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM move_snapshots WHERE `+predicate)
	return ids, err
}

// PruneMoveSnapshots cleans up a crash between ending a move and releasing pins.
func (c *Chronicle) PruneMoveSnapshots(ctx context.Context) ([]string, error) {
	tx, err := c.beginWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	ids, err := pruneMoveSnapshots(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

// LegacyMoveArchiveInUse protects pre-upgrade files while a new link uses them.
func (c *Chronicle) LegacyMoveArchiveInUse(ctx context.Context, id string) (bool, error) {
	var n int
	err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM circle_moves WHERE (legacy_archive_id=? OR (id=? AND snapshot_id IS NULL)) AND archive_status='ready' AND status IN ('waiting','pulling')`, id, id).Scan(&n)
	return n > 0, err
}

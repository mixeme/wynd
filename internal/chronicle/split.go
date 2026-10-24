package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// «Отделиться» (план 46, A9; кадры 6.41–6.45). Любой участник собирает новый
// круг из того, что видел сам: записи, комментарии и реакции всех авторов в
// своих отрезках видимости. Старый круг не меняется и ничего о новом не
// пишет. В новом круге — он владелец и те, кого позвал: позванный входит сам,
// по приглашению, и видит прошлое только там, где видел его в старом круге.
// Файлы на этом же сервере общие: оба круга ссылаются на один блоб, удаляется
// он, когда не ссылается никто (blob.ReferencedPredicate).
//
// Сборка — та же, что оживление из полного архива: снимок круга (как
// FullArchiveData), отбор строк по видимости и вставка с новыми id.

// EventCircleSplit — служебная строка нового круга: где кончается прошлое.
const EventCircleSplit = "circle_split"

// SplitPerson — кого можно позвать (6.43): люди круга, кроме самого человека.
type SplitPerson struct {
	AccountID    string `json:"account_id"`
	Name         string `json:"name"`
	AvatarBlobID string `json:"avatar_blob_id,omitempty"`
}

// SplitPreview — что перейдёт в новый круг (6.42) и кого позвать (6.43).
type SplitPreview struct {
	Posts  int           `json:"posts"`
	Since  *time.Time    `json:"since,omitempty"`
	People []SplitPerson `json:"people"`
}

// SplitInput — «Отделиться»: имя нового круга и кого позвать.
type SplitInput struct {
	CircleID  string
	AccountID string
	Name      string
	Invite    []string
	Now       time.Time
}

// SplitResult — новый круг и кого позвали: приглашения заводит auth.
type SplitResult struct {
	CircleID string
	Posts    int
	Invited  []string
}

// splitWindow — отрезок, который видел тот, кто отделяется.
type splitWindow struct{ start, end time.Time }

func (c *Chronicle) splitWindows(ctx context.Context, membershipID string, now time.Time) ([]splitWindow, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT started_at, ended_at FROM membership_spans
		WHERE membership_id = ? AND can_read = 1 ORDER BY started_at
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []splitWindow
	for rows.Next() {
		var started string
		var ended sql.NullString
		if err := rows.Scan(&started, &ended); err != nil {
			return nil, err
		}
		w := splitWindow{end: now}
		if w.start, err = parseTime(started); err != nil {
			return nil, err
		}
		if ended.Valid {
			if w.end, err = parseTime(ended.String); err != nil {
				return nil, err
			}
			if w.end.After(now) {
				w.end = now
			}
		}
		if w.start.Before(w.end) {
			out = append(out, w)
		}
	}
	return out, rows.Err()
}

// splitActiveMember — отделиться может тот, кто сейчас в круге.
func (c *Chronicle) splitActiveMember(ctx context.Context, circleID, accountID string) (Membership, error) {
	mem, err := c.membership(ctx, c.db, circleID, accountID)
	if err != nil {
		return Membership{}, err
	}
	if mem.Status != StatusActive {
		return Membership{}, ErrForbidden
	}
	return mem, nil
}

// SplitPreview — сколько записей перейдёт, с какого дня, и кого позвать.
func (c *Chronicle) SplitPreview(ctx context.Context, circleID, accountID string, now time.Time) (SplitPreview, error) {
	now = utcOrNow(now)
	var out SplitPreview
	mem, err := c.splitActiveMember(ctx, circleID, accountID)
	if err != nil {
		return out, err
	}
	windows, err := c.splitWindows(ctx, mem.ID, now)
	if err != nil {
		return out, err
	}
	if len(windows) > 0 {
		since := windows[0].start
		out.Since = &since
	}
	if err := c.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM posts p
		WHERE p.circle_id = ? AND p.deleted = 0 AND p.created_at < ? AND `+sqlVisibleAt("p.created_at"),
		circleID, formatTime(now), circleID, accountID).Scan(&out.Posts); err != nil {
		return out, err
	}
	members, err := c.ListMembers(ctx, circleID, accountID)
	if err != nil {
		return out, err
	}
	out.People = []SplitPerson{}
	for _, m := range members {
		if m.Status != StatusActive || m.AccountID == accountID {
			continue
		}
		out.People = append(out.People, SplitPerson{AccountID: m.AccountID, Name: m.Name, AvatarBlobID: m.AvatarBlobID})
	}
	return out, nil
}

// SplitCircle собирает новый круг из того, что видел in.AccountID.
func (c *Chronicle) SplitCircle(ctx context.Context, in SplitInput) (SplitResult, error) {
	var res SplitResult
	now := utcOrNow(in.Now)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return res, ErrInvalid
	}
	if err := checkLen(name, MaxNameChars); err != nil {
		return res, err
	}
	mem, err := c.splitActiveMember(ctx, in.CircleID, in.AccountID)
	if err != nil {
		return res, err
	}
	windows, err := c.splitWindows(ctx, mem.ID, now)
	if err != nil {
		return res, err
	}
	// Позвать можно только тех, кто сейчас в круге.
	keepMembers := map[string]bool{mem.ID: true}
	for _, acc := range in.Invite {
		if acc == in.AccountID || slices.Contains(res.Invited, acc) {
			continue
		}
		m, err := c.membership(ctx, c.db, in.CircleID, acc)
		if err != nil || m.Status != StatusActive {
			return res, ErrInvalid
		}
		keepMembers[m.ID] = true
		res.Invited = append(res.Invited, acc)
	}
	data, err := c.FullArchiveData(ctx, in.CircleID)
	if err != nil {
		return res, err
	}
	tables, blobs, err := splitTables(data, windows, keepMembers, mem.ID, name, now)
	if err != nil {
		return res, err
	}
	accounts := map[string]string{}
	acc := tables["accounts"]
	for i := range acc.Rows {
		r := rowMap(acc, i)
		id, _ := r["id"].(string)
		email, _ := r["email"].(string)
		accounts[id] = email
	}
	blobMap := map[string]string{}
	for _, id := range blobs {
		blobMap[id] = id
	}
	makerMembership := ""
	revived, err := c.ReviveCircle(ctx, ReviveInput{
		FormatVersion: 2,
		SchemaVersion: ReviveSchemaVersions[len(ReviveSchemaVersions)-1],
		Tables:        tables,
		Accounts:      accounts,
		BlobMap:       blobMap,
		ReviverID:     in.AccountID,
		Now:           now,
		SplitFrom:     data.CircleName,
		SameServer:    true,
		afterInsert: func(ctx context.Context, tx *sql.Tx, circleID string) error {
			// Тот, кто отделился, — в новом круге с этой минуты: открытый
			// отрезок после своего прошлого.
			if err := tx.QueryRowContext(ctx, `SELECT id FROM memberships WHERE circle_id = ? AND account_id = ?`,
				circleID, in.AccountID).Scan(&makerMembership); err != nil {
				return err
			}
			spanID, err := newID()
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO membership_spans (id, membership_id, started_at, can_read, can_write)
				VALUES (?, ?, ?, 1, 1)
			`, spanID, makerMembership, formatTime(now))
			return err
		},
	})
	if err != nil {
		return res, err
	}
	res.CircleID = revived.CircleID
	res.Posts = revived.Posts
	return res, nil
}

// splitTables — строки снимка, отобранные по видимости отделяющегося.
func splitTables(data *FullArchiveData, windows []splitWindow, keepMembers map[string]bool,
	makerMembership, name string, now time.Time) (map[string]FullArchiveTable, []string, error) {

	visible := func(v any, inclusive bool) bool {
		s, _ := v.(string)
		t, err := parseTime(s)
		if err != nil {
			return false
		}
		for _, w := range windows {
			if t.Before(w.start) {
				continue
			}
			if inclusive && t.After(w.end) || !inclusive && !t.Before(w.end) {
				continue
			}
			return true
		}
		return false
	}
	src := map[string]FullArchiveTable{}
	for _, t := range data.Tables {
		src[t.Name] = t
	}
	out := map[string]FullArchiveTable{}
	// filter оставляет строки, для которых keep вернул true; keep может их
	// править.
	filter := func(table string, keep func(r map[string]any) bool) []map[string]any {
		t := src[table]
		var kept []map[string]any
		for i := range t.Rows {
			r := rowMap(t, i)
			if keep(r) {
				kept = append(kept, r)
			}
		}
		return kept
	}
	put := func(table string, rows []map[string]any) {
		t := FullArchiveTable{Name: table, Columns: src[table].Columns}
		for _, r := range rows {
			row := make([]any, len(t.Columns))
			for j, col := range t.Columns {
				row[j] = r[col]
			}
			t.Rows = append(t.Rows, row)
		}
		out[table] = t
	}
	str := func(v any) string { s, _ := v.(string); return s }
	alive := func(r map[string]any) bool { return asInt64(r["deleted"]) == 0 }

	seqs := map[int64]bool{}
	posts := map[string]bool{}
	dates := map[string]bool{}
	postRows := filter("posts", func(r map[string]any) bool {
		if !alive(r) || !visible(r["created_at"], false) {
			return false
		}
		posts[str(r["id"])] = true
		seqs[asInt64(r["event_seq"])] = true
		dates[str(r["entry_date"])] = true
		return true
	})
	comments := map[string]bool{}
	commentRows := filter("comments", func(r map[string]any) bool {
		if !alive(r) || !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		comments[str(r["id"])] = true
		seqs[asInt64(r["event_seq"])] = true
		return true
	})
	reactionRows := filter("reactions", func(r map[string]any) bool {
		if !alive(r) || !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		seqs[asInt64(r["event_seq"])] = true
		return true
	})
	titleRows := filter("day_titles", func(r map[string]any) bool {
		if !alive(r) || !visible(r["created_at"], false) {
			return false
		}
		seqs[asInt64(r["event_seq"])] = true
		dates[str(r["entry_date"])] = true
		return true
	})
	coverRows := filter("day_covers", func(r map[string]any) bool {
		if !alive(r) || !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		seqs[asInt64(r["event_seq"])] = true
		dates[str(r["entry_date"])] = true
		return true
	})
	dayRows := filter("days", func(r map[string]any) bool {
		if !dates[str(r["entry_date"])] {
			return false
		}
		// Имя и обложка дня — только если их строки перешли.
		if r["title_event_seq"] != nil && !seqs[asInt64(r["title_event_seq"])] {
			r["title"], r["title_event_seq"] = nil, nil
		}
		if r["cover_event_seq"] != nil && !seqs[asInt64(r["cover_event_seq"])] || r["cover_post_id"] != nil && !posts[str(r["cover_post_id"])] {
			r["cover_post_id"], r["cover_blob_id"], r["cover_event_seq"] = nil, nil, nil
		}
		return true
	})
	// Служебные строки — те, что он видел (конец отрезка включительно, как
	// CanReadServiceEvent); строки записей — по записям.
	eventRows := filter("events", func(r map[string]any) bool {
		if seqs[asInt64(r["seq"])] {
			return true
		}
		return asInt64(r["is_service"]) == 1 && visible(r["created_at"], true)
	})
	circleRows := filter("circles", func(r map[string]any) bool {
		r["name"] = name
		// «Здесь начинается круг» — там, где начинается перешедшее прошлое.
		r["created_at"] = formatTime(now)
		if len(windows) > 0 {
			r["created_at"] = formatTime(windows[0].start)
		}
		return true
	})
	memberRows := filter("memberships", func(r map[string]any) bool {
		id := str(r["id"])
		if !keepMembers[id] {
			return false
		}
		if id != makerMembership {
			// Позванный войдёт сам (6.44): до того — без доступа.
			r["status"] = string(StatusGone)
			r["can_settings"] = int64(0)
		}
		return true
	})
	// Отрезки позванных — пересечение с тем, что видел отделяющийся: прошлое
	// видят только те, кто видел его в старом круге.
	var spanRows []map[string]any
	for _, r := range filter("membership_spans", func(r map[string]any) bool { return keepMembers[str(r["membership_id"])] }) {
		start, err := parseTime(str(r["started_at"]))
		if err != nil {
			return nil, nil, err
		}
		end := now
		if e, ok := r["ended_at"].(string); ok {
			if end, err = parseTime(e); err != nil {
				return nil, nil, err
			}
		}
		for i, w := range windows {
			s, e := maxTime(start, w.start), minTime(end, w.end)
			if !s.Before(e) {
				continue
			}
			piece := map[string]any{}
			for k, v := range r {
				piece[k] = v
			}
			piece["id"] = fmt.Sprintf("%s#%d", str(r["id"]), i)
			piece["started_at"] = formatTime(s)
			piece["ended_at"] = formatTime(e)
			spanRows = append(spanRows, piece)
		}
	}

	put("circles", circleRows)
	put("identities", filter("identities", func(map[string]any) bool { return true }))
	put("identity_names", filter("identity_names", func(map[string]any) bool { return true }))
	put("memberships", memberRows)
	put("membership_spans", spanRows)
	put("events", eventRows)
	put("posts", postRows)
	put("post_media", filter("post_media", func(r map[string]any) bool { return posts[str(r["post_id"])] }))
	put("comments", commentRows)
	put("comment_media", filter("comment_media", func(r map[string]any) bool { return comments[str(r["comment_id"])] }))
	put("reactions", reactionRows)
	put("days", dayRows)
	put("day_titles", titleRows)
	put("day_covers", coverRows)
	put("accounts", filter("accounts", func(map[string]any) bool { return true }))

	// Файлы — те, на которые ссылаются перешедшие строки: в новом круге они
	// те же блобы.
	seen := map[string]bool{}
	var blobs []string
	for _, t := range out {
		for i := range t.Rows {
			for col, v := range rowMap(t, i) {
				if s, ok := v.(string); ok && s != "" && isBlobColumn(col) && !seen[s] {
					seen[s] = true
					blobs = append(blobs, s)
				}
			}
		}
	}
	return out, blobs, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// SplitJoinInfo — экран «позвали» (6.44): откуда отделён круг, кто зовёт,
// под каким именем войдёт позванный и с какого дня его прошлое.
type SplitJoinInfo struct {
	From       string     `json:"split_from"`
	InviteLine string     `json:"invite_line,omitempty"`
	YouName    string     `json:"you_name,omitempty"`
	PastFrom   *time.Time `json:"past_from,omitempty"`
}

// SplitJoinFor — nil, если круг не отделён (обычное приглашение).
func (c *Chronicle) SplitJoinFor(ctx context.Context, circleID, accountID, inviterAccountID string) (*SplitJoinInfo, error) {
	var payload string
	err := c.db.QueryRowContext(ctx, `
		SELECT payload FROM events WHERE circle_id = ? AND event_type = ? ORDER BY seq LIMIT 1
	`, circleID, EventCircleSplit).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p struct {
		From string `json:"from"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return nil, err
	}
	info := &SplitJoinInfo{From: p.From}
	if inviterAccountID != "" {
		if m, err := c.membership(ctx, c.db, circleID, inviterAccountID); err == nil {
			name, err := c.identityName(ctx, c.db, m.IdentityID)
			if err != nil {
				return nil, err
			}
			info.InviteLine = SplitInviteLine(c.identityGender(ctx, c.db, m.IdentityID), name, p.From)
		}
	}
	m, err := c.membership(ctx, c.db, circleID, accountID)
	if err == ErrNotFound {
		return info, nil
	}
	if err != nil {
		return nil, err
	}
	if info.YouName, err = c.identityName(ctx, c.db, m.IdentityID); err != nil {
		return nil, err
	}
	var first sql.NullString
	if err := c.db.QueryRowContext(ctx, `
		SELECT MIN(started_at) FROM membership_spans WHERE membership_id = ? AND can_read = 1
	`, m.ID).Scan(&first); err != nil {
		return nil, err
	}
	if first.Valid {
		t, err := parseTime(first.String)
		if err != nil {
			return nil, err
		}
		info.PastFrom = &t
	}
	return info, nil
}

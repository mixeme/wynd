package chronicle

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Оживление круга из полного архива (план 52, A1 этап 2). Устройство читает
// файл и присылает строки таблиц как есть (data/<таблица>.json) и
// соответствие «блоб архива → блоб этого сервера». Здесь круг вставляется
// заново: все идентификаторы новые — круг может ожить рядом с живым исходным,
// и на том же сервере; люди — по почте; владелец — тот, кто оживил.

// ReviveSchemaVersions — какие схемы архива сервер умеет оживлять (разметка
// архива 2, новая нумерация миграций). Новая схема с другими колонками —
// сюда и в переводчик строк.
// Схемы 2–3 (раскладка улочки, A2), 4 (почта администратора) и 5 (письмо о
// поломках) таблиц круга не меняли. Схема 6 добавила post_media.video_duration_ms:
// reviveRow берёт только столбцы, что есть в обеих схемах, архив без него — NULL.
// Схема 7 (итог переноса, incoming_moves.already_here) таблиц круга не меняла.
// Схемы 8 и 9 (владелец и имена в переносе, A12) — тоже только incoming_moves.
var ReviveSchemaVersions = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

// ReviveLegacySchemaVersions — схемы архивов разметки 1: номера прежней
// нумерации (до 0.34.0). Схема 36 — та же, что 1.
var ReviveLegacySchemaVersions = []int{34, 35, 36}

// ReviveSchemaKnown — оживляет ли сервер архив такой разметки и схемы.
func ReviveSchemaKnown(formatVersion, schema int) bool {
	switch formatVersion {
	case 1:
		return slices.Contains(ReviveLegacySchemaVersions, schema)
	case 2:
		return slices.Contains(ReviveSchemaVersions, schema)
	}
	return false
}

// reviveOrder — порядок вставки: по зависимостям.
var reviveOrder = []string{
	"circles", "identities", "identity_names", "memberships", "membership_spans",
	"events", "posts", "post_media", "comments", "comment_media", "reactions",
	"days", "day_titles", "day_covers",
}

// ReviveInput — что прислало устройство.
type ReviveInput struct {
	// FormatVersion — разметка архива (manifest.format_version): от неё
	// зависит, по какой нумерации SchemaVersion.
	FormatVersion int
	SchemaVersion int
	Tables        map[string]FullArchiveTable
	// Accounts — почта по id учётки архива (data/accounts.json).
	Accounts map[string]string
	// BlobMap — блоб архива → блоб этого сервера (уже загружен).
	BlobMap   map[string]string
	ReviverID string
	Now       time.Time
	// MovedFrom — адрес прежнего сервера, если круг приехал переносом (план
	// 53): в ленте тогда «Круг перенесён с …» (7.25), а не «оживил из архива».
	MovedFrom string
	// NewMember — почты того, кто оживляет, в архиве нет, и на «Аня — это
	// вы?» он ответил «Нет» (7.29б → 7.29в, A12): входит своим именем,
	// новым участником и владельцем, видит круг с оживления; лицо прежнего
	// владельца остаётся за его почтой. Пусто — «Да, это я»: лицо и прошлое
	// прежнего владельца переходят к нему (так было и до A12).
	NewMember *ReviveNewMember
}

// ReviveNewMember — кто оживляет, если он не прежний владелец (7.29в).
type ReviveNewMember struct {
	Name   string
	Gender Gender
}

// ReviveResult — что получилось, для итога 7.33 / 7.34.
type ReviveResult struct {
	CircleID string
	Posts    int
	Names    int
	// Bytes — файлы круга: по ним сверяется архив (7.33).
	Bytes int64
	// TextBytes — текст круга, как его считает квота (C42): в итог для
	// экрана, чтобы «Круг на месте» не показывал 0 Б у круга без файлов.
	TextBytes int64
	// NotJoined — учётки, заведённые по почте из архива: на этом сервере
	// они ещё не были («Ещё не вошли», 6.6).
	NotJoined []string
	// AlreadyHere — сколько людей круга уже есть на этом сервере (кроме того,
	// кто оживил): письмо им не нужно, круг сам встаёт в их список.
	AlreadyHere int
}

// ReviveCircle вставляет круг из строк архива одной транзакцией.
func (c *Chronicle) ReviveCircle(ctx context.Context, in ReviveInput) (ReviveResult, error) {
	var res ReviveResult
	if !ReviveSchemaKnown(in.FormatVersion, in.SchemaVersion) {
		return res, fmt.Errorf("%w: archive format %d schema %d", ErrInvalid, in.FormatVersion, in.SchemaVersion)
	}
	circles := in.Tables["circles"]
	if len(circles.Rows) != 1 {
		return res, fmt.Errorf("%w: archive has %d circles", ErrInvalid, len(circles.Rows))
	}
	now := utcOrNow(in.Now)
	ids := map[string]string{}
	newID := func(old string) (string, error) {
		if v, ok := ids[old]; ok {
			return v, nil
		}
		v, err := newIDFn()
		if err != nil {
			return "", err
		}
		ids[old] = v
		return v, nil
	}
	// Блобы: только присланные устройством и уже лежащие здесь.
	for old, nw := range in.BlobMap {
		ids[old] = nw
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// Учётки — по почте. Есть на сервере — она; нет — заводится без входа.
	owner, _ := rowMap(circles, 0)["owner_account_id"].(string)
	reviverEmail := ""
	if err := tx.QueryRowContext(ctx, `SELECT email FROM accounts WHERE id = ?`, in.ReviverID).Scan(&reviverEmail); err != nil {
		return res, err
	}
	reviverInArchive := false
	for _, email := range in.Accounts {
		if strings.EqualFold(email, reviverEmail) {
			reviverInArchive = true
		}
	}
	newMember := in.NewMember
	if _, ownerHasEmail := in.Accounts[owner]; reviverInArchive || !ownerHasEmail {
		// Почта оживляющего в архиве — он и так в своём имени; почты прежнего
		// владельца нет — его лицо не за кем оставить, оно переходит к нему.
		newMember = nil
	}
	if newMember != nil {
		newMember.Name = strings.TrimSpace(newMember.Name)
		if newMember.Name == "" {
			return res, ErrInvalid
		}
		if err := checkLen(newMember.Name, MaxNameChars); err != nil {
			return res, err
		}
	}
	accounts := map[string]string{}
	for oldID, email := range in.Accounts {
		if !reviverInArchive && newMember == nil && oldID == owner {
			// Тот, кто оживил, — владелец и лицо прежнего владельца (план 52).
			accounts[oldID] = in.ReviverID
			continue
		}
		var id string
		err := tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE email = ? COLLATE NOCASE`, email).Scan(&id)
		if err == sql.ErrNoRows {
			if id, err = newIDFn(); err != nil {
				return res, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO accounts (id, email, created_at) VALUES (?, ?, ?)`, id, email, formatTime(now)); err != nil {
				return res, err
			}
			res.NotJoined = append(res.NotJoined, id)
		} else if err != nil {
			return res, err
		} else if id != in.ReviverID {
			res.AlreadyHere++
		}
		accounts[oldID] = id
	}
	if _, ok := accounts[owner]; !ok && newMember == nil {
		accounts[owner] = in.ReviverID
	}

	// Новые id — заранее, всем строкам: событие ссылается на запись, которая
	// вставится позже.
	for _, name := range reviveOrder {
		t := in.Tables[name]
		col := slices.Index(t.Columns, "id")
		if col < 0 {
			continue
		}
		for _, r := range t.Rows {
			if old, ok := r[col].(string); ok {
				if _, err := newID(old); err != nil {
					return res, err
				}
			}
		}
	}

	seqs := map[int64]int64{}
	for _, name := range reviveOrder {
		t, ok := in.Tables[name]
		if !ok {
			continue
		}
		cols, err := c.tableColumns(ctx, tx, name)
		if err != nil {
			return res, err
		}
		for i := range t.Rows {
			row := rowMap(t, i)
			if err := c.reviveRow(ctx, tx, name, row, cols, ids, newID, accounts, seqs, in, now, &res); err != nil {
				return res, fmt.Errorf("revive %s: %w", name, err)
			}
		}
	}
	res.CircleID = ids[rowMap(circles, 0)["id"].(string)]
	res.Posts = len(in.Tables["posts"].Rows)
	res.Names = len(in.Tables["identities"].Rows)

	// Служебная строка: журнал продолжается с того места, где замер.
	var reviverIdentity, reviverName string
	if newMember != nil {
		// Новым человеком: своё лицо и отрезок с оживления; прежний владелец
		// остаётся участником без права менять настройки, как после передачи.
		if _, reviverIdentity, err = insertMemberTx(ctx, tx, res.CircleID, in.ReviverID, newMember.Name, newMember.Gender, true, now); err != nil {
			return res, err
		}
		if old := accounts[owner]; old != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE memberships SET can_settings = 0, updated_at = ? WHERE circle_id = ? AND account_id = ?`,
				formatTime(now), res.CircleID, old); err != nil {
				return res, err
			}
		}
	} else if err := tx.QueryRowContext(ctx, `SELECT identity_id FROM memberships WHERE circle_id = ? AND account_id = ?`, res.CircleID, in.ReviverID).Scan(&reviverIdentity); err != nil {
		return res, fmt.Errorf("revive: reviver has no identity: %w", err)
	}
	if reviverName, err = c.identityName(ctx, tx, reviverIdentity); err != nil {
		return res, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memberships SET status = 'active', can_settings = 1, updated_at = ? WHERE circle_id = ? AND account_id = ?`,
		formatTime(now), res.CircleID, in.ReviverID); err != nil {
		return res, err
	}
	// Кто приехал с кругом — встретить экраном 1.10 при первом входе.
	kind := "revived"
	if in.MovedFrom != "" {
		kind = "moved"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circle_arrivals (circle_id, account_id, kind, created_at)
		SELECT circle_id, account_id, ?, ? FROM memberships
		WHERE circle_id = ? AND account_id != ? AND status = 'active'
	`, kind, formatTime(now), res.CircleID, in.ReviverID); err != nil {
		return res, err
	}
	event, summary := "circle_revived", summaryCircleRevived(c.identityGender(ctx, tx, reviverIdentity), reviverName)
	if in.MovedFrom != "" {
		event, summary = "circle_moved_in", summaryCircleMovedIn(in.MovedFrom)
	}
	if _, err := c.appendEvent(ctx, tx, appendEventInput{
		circleID: res.CircleID, eventType: event, isService: true,
		actorIdentityID: reviverIdentity, actorName: reviverName,
		summary: summary,
		now:     now,
	}); err != nil {
		return res, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(size_bytes), 0) FROM blobs WHERE id IN (`+
		placeholders(len(in.BlobMap))+`)`, mapValues(in.BlobMap)...).Scan(&res.Bytes); err != nil {
		return res, err
	}
	// Текст уже сочли триггеры базы.
	if err := tx.QueryRowContext(ctx, `SELECT text_bytes FROM circles WHERE id = ?`, res.CircleID).Scan(&res.TextBytes); err != nil {
		return res, err
	}
	return res, tx.Commit()
}

var newIDFn = newID

var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func rowMap(t FullArchiveTable, i int) map[string]any {
	m := make(map[string]any, len(t.Columns))
	for j, col := range t.Columns {
		if j < len(t.Rows[i]) {
			m[col] = t.Rows[i][j]
		}
	}
	return m
}

func (c *Chronicle) tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// reviveRow переводит одну строку архива и вставляет её.
func (c *Chronicle) reviveRow(ctx context.Context, tx *sql.Tx, table string, row map[string]any, cols map[string]bool,
	ids map[string]string, newID func(string) (string, error), accounts map[string]string,
	seqs map[int64]int64, in ReviveInput, now time.Time, res *ReviveResult) error {

	// Собственный id строки — новый (у days, day_titles, day_covers его нет).
	if v, ok := row["id"].(string); ok && table != "events" {
		nv, err := newID(v)
		if err != nil {
			return err
		}
		row["id"] = nv
	}
	var oldSeq int64
	if table == "events" {
		oldSeq = asInt64(row["seq"])
		delete(row, "seq")
		nv, err := newID(row["id"].(string))
		if err != nil {
			return err
		}
		row["id"] = nv
	}
	for col, v := range row {
		switch {
		case col == "id":
		case col == "account_id" || col == "owner_account_id":
			if s, ok := v.(string); ok && s != "" {
				row[col] = accounts[s]
			}
		case strings.HasSuffix(col, "event_seq"):
			if v != nil {
				ns, ok := seqs[asInt64(v)]
				if !ok {
					return fmt.Errorf("%s.%s: unknown event %v", table, col, v)
				}
				row[col] = ns
			}
		case col == "payload":
			// В данных события — id записей и лиц: их тоже на новые.
			if s, ok := v.(string); ok {
				row[col] = uuidRe.ReplaceAllStringFunc(s, func(old string) string {
					if nw, ok := ids[old]; ok {
						return nw
					}
					return old
				})
			}
		default:
			if s, ok := v.(string); ok {
				if nv, ok := ids[s]; ok {
					row[col] = nv
				} else if isBlobColumn(col) && s != "" {
					return fmt.Errorf("%s.%s: file %s was not uploaded", table, col, s)
				}
			}
		}
	}
	if table == "circles" {
		// Живёт заново: без цикла архивации, не замерший, место — по
		// правилам этого сервера, текст сочтут триггеры.
		for _, col := range []string{"archive_cutoff_date", "archive_deadline", "archive_reminder_before_sec",
			"cutoff_locked_at", "archive_reminder_sent_at", "archive_cycle_started_at", "frozen_at", "quota_bytes"} {
			row[col] = nil
		}
		row["quota_custom"] = 0
		row["text_bytes"] = 0
		row["owner_account_id"] = in.ReviverID
		row["updated_at"] = formatTime(now)
	}
	if table == "identities" {
		if s, ok := row["account_id"].(string); ok && s == "" {
			row["account_id"] = nil
		}
	}
	var names []string
	var args []any
	for col, v := range row {
		if !cols[col] {
			continue
		}
		names = append(names, col)
		args = append(args, v)
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+strings.Join(names, ", ")+`) VALUES (`+placeholders(len(names))+`)`, args...)
	if err != nil {
		return err
	}
	if table == "events" {
		ns, err := r.LastInsertId()
		if err != nil {
			return err
		}
		seqs[oldSeq] = ns
	}
	return nil
}

func isBlobColumn(col string) bool {
	return col == "blob_id" || strings.HasSuffix(col, "_blob_id")
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func placeholders(n int) string {
	if n == 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func mapValues(m map[string]string) []any {
	out := make([]any, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ErasedName — подпись вместо стёртого имени (кадры 7.14, 7.17: «записи
// останутся без имени»).
const ErasedName = "без имени"

// EraseAccountNamesInTx стирает имя, фото и пол учётки во всех кругах сервера
// (план 46, A3; wynd.html, «Журнал»). Сказанное остаётся: записи,
// комментарии, реакции, дни и строки хроники — без подписи. Имя лежит и в
// копиях: подпись записи, комментария и реакции, имя действующего у события и
// готовая строка хроники. Их переписываем; сам факт события не меняется.
// Вызывать до того, как идентичности отвяжутся от учётки.
func (c *Chronicle) EraseAccountNamesInTx(ctx context.Context, tx *sql.Tx, accountID string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM identities WHERE account_id = ?`, accountID)
	if err != nil {
		return fmt.Errorf("erase: list identities: %w", err)
	}
	erased := map[string]bool{}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		erased[id] = true
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	// Строки хроники переписываются по именам на момент события, поэтому —
	// раньше, чем сотрутся сами имена.
	if err := c.rewriteErasedSummaries(ctx, tx, erased); err != nil {
		return err
	}
	stamp := formatTime(now)
	for start := 0; start < len(ids); start += avatarBatch {
		marks, args := inClause(ids[start:min(start+avatarBatch, len(ids))])
		withLabel := append([]any{ErasedName}, args...)
		for _, step := range []struct {
			query string
			args  []any
		}{
			// Фото уходит вместе с именем: блоб аватара больше ничем не
			// держится, и ежедневная уборка удалит файл.
			{`UPDATE identity_names SET name = '', avatar_blob_id = NULL, erased_at = COALESCE(erased_at, ?)
				WHERE identity_id IN (` + marks + `)`, append([]any{stamp}, args...)},
			{`UPDATE identities SET gender = NULL WHERE id IN (` + marks + `)`, args},
			{`UPDATE posts SET author_name = ? WHERE identity_id IN (` + marks + `)`, withLabel},
			{`UPDATE comments SET author_name = ? WHERE identity_id IN (` + marks + `)`, withLabel},
			{`UPDATE reactions SET author_name = ? WHERE identity_id IN (` + marks + `)`, withLabel},
			{`UPDATE events SET actor_name = ? WHERE actor_identity_id IN (` + marks + `)`, withLabel},
			// Поиск по файлам комментариев: триггеры подпись записи и
			// комментария переносят в индекс сами, а эти строки — нет.
			{`UPDATE content_fts SET author_name = ? WHERE kind IN ('cfile', 'caudio') AND comment_id IN (
				SELECT cm.id FROM comment_media cm JOIN comments c ON c.id = cm.comment_id
				WHERE c.identity_id IN (` + marks + `))`, withLabel},
		} {
			if _, err := tx.ExecContext(ctx, step.query, step.args...); err != nil {
				return fmt.Errorf("erase names: %w", err)
			}
		}
	}
	return nil
}

type erasableEvent struct {
	seq       int64
	eventType string
	actorID   string
	targetID  string
	payload   string
	summary   string
	createdAt string
}

// rewriteErasedSummaries пересобирает строки хроники, где названо стёртое
// имя: у событий, где стёртый действовал, и у тех, где его назвали
// (передача владения, право на настройки).
func (c *Chronicle) rewriteErasedSummaries(ctx context.Context, tx *sql.Tx, erased map[string]bool) error {
	ids := make([]string, 0, len(erased))
	for id := range erased {
		ids = append(ids, id)
	}
	var events []erasableEvent
	for start := 0; start < len(ids); start += avatarBatch {
		marks, args := inClause(ids[start:min(start+avatarBatch, len(ids))])
		rows, err := tx.QueryContext(ctx, `
			SELECT seq, event_type, COALESCE(actor_identity_id, ''), COALESCE(target_id, ''),
				payload, summary, created_at
			FROM events
			WHERE actor_identity_id IN (`+marks+`) OR target_id IN (`+marks+`)
		`, append(args, args...)...)
		if err != nil {
			return fmt.Errorf("erase: list events: %w", err)
		}
		for rows.Next() {
			var ev erasableEvent
			if err := rows.Scan(&ev.seq, &ev.eventType, &ev.actorID, &ev.targetID,
				&ev.payload, &ev.summary, &ev.createdAt); err != nil {
				rows.Close()
				return err
			}
			events = append(events, ev)
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	for _, ev := range events {
		summary, payload, ok := c.erasedSummary(ctx, tx, ev, erased)
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE events SET summary = ?, payload = ? WHERE seq = ?
		`, summary, payload, ev.seq); err != nil {
			return fmt.Errorf("erase: rewrite event: %w", err)
		}
	}
	return nil
}

// erasedSummary — строка события без стёртого имени. ok=false — строку не
// трогаем: имени в ней нет, или запись уже удалена и строка пуста.
func (c *Chronicle) erasedSummary(ctx context.Context, tx *sql.Tx, ev erasableEvent, erased map[string]bool) (string, string, bool) {
	if ev.summary == "" {
		return "", "", false
	}
	var p map[string]any
	_ = json.Unmarshal([]byte(ev.payload), &p)
	str := func(key string) string {
		v, _ := p[key].(string)
		return v
	}
	// Имя и род участника на момент события; стёртый — без имени и без рода.
	who := func(identityID string) (Gender, string) {
		if erased[identityID] {
			return GenderNone, ErasedName
		}
		return c.identityGender(ctx, tx, identityID), c.identityNameAt(ctx, tx, identityID, ev.createdAt)
	}
	actorErased := erased[ev.actorID]
	g, name := who(ev.actorID)
	payload := ev.payload
	switch ev.eventType {
	case "member.joined":
		return summaryMemberJoined(GenderNone, ErasedName), payload, actorErased
	case "member.left", "member.left_with_access":
		return summaryMemberLeft(GenderNone, ErasedName), payload, actorErased
	case "identity.avatar_set":
		return summaryAvatarSet(ErasedName), payload, actorErased
	case "identity.avatar_cleared":
		return summaryAvatarCleared(ErasedName), payload, actorErased
	case "identity.renamed":
		// В строке и в событии оба имени — и прежнее, и новое.
		return summaryIdentityRenamed(ErasedName, ErasedName), "{}", actorErased
	case "owner.transferred":
		_, to := who(ev.targetID)
		return summaryOwnerTransferred(name, to), payload, true
	case "member.settings_granted":
		_, target := who(ev.targetID)
		return summarySettingsGranted(g, name, target), payload, true
	case "member.settings_revoked":
		_, target := who(ev.targetID)
		return summarySettingsRevoked(g, name, target), payload, true
	}
	if !actorErased {
		return "", "", false
	}
	switch ev.eventType {
	case "post.created":
		return summaryPostCreated(g, name), payload, true
	case "post.edited":
		return summaryPostEdited(g, name), payload, true
	case "comment.created":
		return summaryCommentCreated(g, name), payload, true
	case "comment.edited":
		return summaryCommentEdited(g, name), payload, true
	case "reaction.set":
		return summaryReactionSet(g, name, str("emoji")), payload, true
	case "day.titled":
		return summaryDayTitled(g, name, str("title"), str("entry_date")), payload, true
	case "day.cover_set":
		return summaryDayCoverSet(g, name, str("entry_date")), payload, true
	case "day.title_cleared":
		return summaryDayTitleCleared(g, name, str("entry_date")), payload, true
	case "day.cover_cleared":
		return summaryDayCoverCleared(g, name, str("entry_date")), payload, true
	}
	return "", "", false
}

// identityNameAt — имя участника, действовавшее в момент at; если его нет,
// нынешнее.
func (c *Chronicle) identityNameAt(ctx context.Context, q querier, identityID, at string) string {
	var name string
	err := q.QueryRowContext(ctx, `
		SELECT name FROM identity_names
		WHERE identity_id = ? AND erased_at IS NULL AND effective_at <= ?
		ORDER BY effective_at DESC LIMIT 1
	`, identityID, at).Scan(&name)
	if err == nil {
		return name
	}
	name, _ = c.identityName(ctx, q, identityID)
	return name
}

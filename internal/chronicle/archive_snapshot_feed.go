package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// snapshotRecord переводит имена колонок SQLite в поля доменной структуры.
// Читаемая часть не перечитывает живую БД после машинного снимка.
func snapshotRecord[T any](row map[string]any) (T, error) {
	values := make(map[string]any, len(row))
	for column, v := range row {
		name := strings.ReplaceAll(column, "_", "")
		if v != nil && (name == "deleted" || name == "iscover" || name == "voice") {
			if b, ok := v.(bool); ok {
				v = b
			} else {
				v = asInt64(v) != 0
			}
		}
		if name == "audiopeaks" {
			if raw, ok := v.(string); ok {
				v = splitPeaks(raw)
			}
		}
		values[name] = v
	}
	var out T
	raw, err := json.Marshal(values)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}

func snapshotEditWindow(row map[string]any) EditWindow {
	window, _ := editWindowFromSQL(sql.NullInt64{Int64: asInt64(row["edit_window_sec"]), Valid: row["edit_window_sec"] != nil})
	return window
}

func snapshotMedia(row map[string]any, blobs map[string]map[string]any) (PostMedia, error) {
	copy := make(map[string]any, len(row)+3)
	for k, v := range row {
		copy[k] = v
	}
	id, _ := row["blob_id"].(string)
	for _, col := range []string{"size_bytes", "mime_type", "original_filename"} {
		copy[col] = blobs[id][col]
	}
	media, err := snapshotRecord[PostMedia](copy)
	if err != nil {
		return media, err
	}
	if row["crop_x"] != nil && row["crop_y"] != nil && row["crop_w"] != nil && row["crop_h"] != nil {
		asFloat := func(v any) float64 {
			if f, ok := v.(float64); ok {
				return f
			}
			return float64(asInt64(v))
		}
		media.Crop = &CoverCrop{X: asFloat(row["crop_x"]), Y: asFloat(row["crop_y"]), W: asFloat(row["crop_w"]), H: asFloat(row["crop_h"])}
	}
	return media, nil
}

// ArchiveFeed строит читаемую ленту из того же снимка, что машинные таблицы.
func (c *Chronicle) ArchiveFeed(ctx context.Context, data *FullArchiveData) ([]FeedPost, map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	tables := map[string]FullArchiveTable{}
	for _, table := range data.Tables {
		tables[table.Name] = table
	}
	rows := func(name string) []map[string]any {
		table := tables[name]
		out := make([]map[string]any, 0, len(table.Rows))
		for i := range table.Rows {
			out = append(out, rowMap(table, i))
		}
		return out
	}
	blobs := map[string]map[string]any{}
	for _, row := range rows("blobs") {
		id, _ := row["id"].(string)
		blobs[id] = row
	}
	media := map[string][]PostMedia{}
	commentMedia := map[string][]PostMedia{}
	for _, table := range []string{"post_media", "comment_media"} {
		for _, row := range rows(table) {
			m, err := snapshotMedia(row, blobs)
			if err != nil {
				return nil, nil, err
			}
			if table == "post_media" {
				id, _ := row["post_id"].(string)
				media[id] = append(media[id], m)
			} else {
				id, _ := row["comment_id"].(string)
				commentMedia[id] = append(commentMedia[id], m)
			}
		}
	}
	for _, groups := range []map[string][]PostMedia{media, commentMedia} {
		for id := range groups {
			sort.SliceStable(groups[id], func(i, j int) bool { return groups[id][i].SortOrder < groups[id][j].SortOrder })
		}
	}
	comments := map[string][]Comment{}
	for _, row := range rows("comments") {
		comment, err := snapshotRecord[Comment](row)
		if err != nil {
			return nil, nil, err
		}
		if comment.Deleted {
			continue
		}
		comment.EditWindow = snapshotEditWindow(row)
		comment.Media = commentMedia[comment.ID]
		comments[comment.PostID] = append(comments[comment.PostID], comment)
	}
	for id := range comments {
		sort.SliceStable(comments[id], func(i, j int) bool { return comments[id][i].CreatedAt.Before(comments[id][j].CreatedAt) })
	}
	reactions := map[string][]Reaction{}
	for _, row := range rows("reactions") {
		reaction, err := snapshotRecord[Reaction](row)
		if err != nil {
			return nil, nil, err
		}
		if reaction.Deleted {
			continue
		}
		reaction.EditWindow = snapshotEditWindow(row)
		reactions[reaction.PostID] = append(reactions[reaction.PostID], reaction)
	}
	for id := range reactions {
		sort.SliceStable(reactions[id], func(i, j int) bool { return reactions[id][i].CreatedAt.Before(reactions[id][j].CreatedAt) })
	}
	var out []FeedPost
	for _, row := range rows("posts") {
		post, err := snapshotRecord[Post](row)
		if err != nil {
			return nil, nil, err
		}
		if post.Deleted {
			continue
		}
		post.EditWindow = snapshotEditWindow(row)
		out = append(out, FeedPost{Post: post, Media: media[post.ID], Comments: comments[post.ID], Reactions: reactions[post.ID]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Post.CreatedAt.After(out[j].Post.CreatedAt) })
	days := map[string]string{}
	for _, row := range rows("days") {
		if title, ok := row["title"].(string); ok {
			date, _ := row["entry_date"].(string)
			days[date] = title
		}
	}
	return out, days, nil
}

// ArchiveAvatars выбирает текущие неудалённые аватары внутри снимка.
func ArchiveAvatars(data *FullArchiveData) map[string]string {
	avatars := map[string]string{}
	latest := map[string]time.Time{}
	latestID := map[string]string{}
	for _, table := range data.Tables {
		if table.Name != "identity_names" {
			continue
		}
		for i := range table.Rows {
			row := rowMap(table, i)
			if row["erased_at"] != nil {
				continue
			}
			identity, _ := row["identity_id"].(string)
			id, _ := row["id"].(string)
			text, _ := row["effective_at"].(string)
			at, err := parseTime(text)
			if err != nil {
				continue
			}
			if old, ok := latest[identity]; ok && (old.After(at) || old.Equal(at) && latestID[identity] > id) {
				continue
			}
			latest[identity] = at
			latestID[identity] = id
			delete(avatars, identity)
			if blob, ok := row["avatar_blob_id"].(string); ok && blob != "" {
				avatars[identity] = blob
			}
		}
	}
	return avatars
}

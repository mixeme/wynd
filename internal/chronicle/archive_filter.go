package chronicle

import (
	"context"
	"fmt"
	"strings"
)

// OwnerArchiveData — копия, которую вправе скачать нынешний владелец.
// Для переноса и отделения остаётся сырой FullArchiveData: у них другие правила.
func (c *Chronicle) OwnerArchiveData(ctx context.Context, circleID string) (*FullArchiveData, error) {
	data, err := c.FullArchiveData(ctx, circleID)
	if err != nil {
		return nil, err
	}
	if data.ArchiveScope.All {
		return data, nil
	}
	return filterArchiveData(data)
}

func filterArchiveData(data *FullArchiveData) (*FullArchiveData, error) {
	src := map[string]FullArchiveTable{}
	out := map[string]FullArchiveTable{}
	for _, t := range data.Tables {
		src[t.Name] = t
		out[t.Name] = t
	}
	str := func(v any) string { s, _ := v.(string); return s }
	visible := func(v any, service bool) bool {
		at, err := parseTime(str(v))
		return err == nil && data.ArchiveScope.contains(at, service)
	}
	seqs := map[int64]bool{}
	entities := map[string]bool{}
	dates := map[string]bool{}
	filter := func(name string, keep func(map[string]any) bool, journal bool) {
		t := src[name]
		t.Rows = nil
		for i := range src[name].Rows {
			r := rowMap(src[name], i)
			if !keep(r) {
				continue
			}
			row := make([]any, len(t.Columns))
			for j, col := range t.Columns {
				row[j] = r[col]
			}
			t.Rows = append(t.Rows, row)
		}
		if journal && len(t.Rows) != len(src[name].Rows) {
			data.PartialJournal = true
		}
		out[name] = t
	}
	remember := func(r map[string]any) {
		if id := str(r["id"]); id != "" {
			entities[id] = true
		}
		if date := str(r["entry_date"]); date != "" {
			dates[date] = true
		}
		for col, v := range r {
			if strings.HasSuffix(col, "event_seq") && v != nil {
				seqs[asInt64(v)] = true
			}
		}
	}
	posts := map[string]bool{}
	comments := map[string]bool{}
	filter("posts", func(r map[string]any) bool {
		if !visible(r["created_at"], false) {
			return false
		}
		posts[str(r["id"])] = true
		remember(r)
		return true
	}, true)
	filter("post_media", func(r map[string]any) bool { return posts[str(r["post_id"])] }, false)
	filter("comments", func(r map[string]any) bool {
		if !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		comments[str(r["id"])] = true
		remember(r)
		return true
	}, true)
	filter("comment_media", func(r map[string]any) bool { return comments[str(r["comment_id"])] }, false)
	filter("reactions", func(r map[string]any) bool {
		if !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		remember(r)
		return true
	}, true)
	filter("day_titles", func(r map[string]any) bool {
		if !visible(r["created_at"], false) {
			return false
		}
		remember(r)
		return true
	}, true)
	filter("day_covers", func(r map[string]any) bool {
		if !posts[str(r["post_id"])] || !visible(r["created_at"], false) {
			return false
		}
		remember(r)
		return true
	}, true)
	// Проекции восстанавливаются по оставшимся строкам: скрытое позднее
	// название или обложка не стирает ранее доступное сказанное о дне.
	latestDays := func(name string, keep func(map[string]any) bool) map[string]map[string]any {
		latest := map[string]map[string]any{}
		for i := range out[name].Rows {
			r := rowMap(out[name], i)
			if !keep(r) {
				continue
			}
			date := str(r["entry_date"])
			old := latest[date]
			at, before := parseTimeOrZero(str(r["created_at"])), parseTimeOrZero(str(old["created_at"]))
			if old == nil || at.After(before) || at.Equal(before) && asInt64(r["event_seq"]) > asInt64(old["event_seq"]) {
				latest[date] = r
			}
		}
		return latest
	}
	deletedPosts := map[string]bool{}
	for i := range out["posts"].Rows {
		r := rowMap(out["posts"], i)
		deletedPosts[str(r["id"])] = asInt64(r["deleted"]) != 0
	}
	titles := latestDays("day_titles", func(map[string]any) bool { return true })
	covers := latestDays("day_covers", func(r map[string]any) bool { return !deletedPosts[str(r["post_id"])] })
	filter("days", func(r map[string]any) bool {
		if !dates[str(r["entry_date"])] {
			return false
		}
		date := str(r["entry_date"])
		r["title"], r["title_event_seq"] = nil, nil
		if title := titles[date]; str(title["title"]) != "" {
			r["title"], r["title_event_seq"] = title["title"], title["event_seq"]
		}
		r["cover_post_id"], r["cover_blob_id"], r["cover_event_seq"] = nil, nil, nil
		if cover := covers[date]; str(cover["blob_id"]) != "" {
			r["cover_post_id"], r["cover_blob_id"], r["cover_event_seq"] = cover["post_id"], cover["blob_id"], cover["event_seq"]
		}
		return true
	}, false)
	filter("events", func(r map[string]any) bool {
		if seqs[asInt64(r["seq"])] {
			return true
		}
		typeName := str(r["event_type"])
		target := str(r["target_id"])
		if strings.HasPrefix(typeName, "post.") || strings.HasPrefix(typeName, "comment.") || strings.HasPrefix(typeName, "reaction.") {
			if !entities[target] {
				return false
			}
		}
		if strings.HasPrefix(typeName, "day.") || strings.HasPrefix(typeName, "day_") {
			// Каждое сказанное о дне имеет строку day_titles/day_covers.
			// Событие скрытой обложки не должно раскрывать post_id и blob_id.
			return false
		}
		return visible(r["created_at"], asInt64(r["is_service"]) == 1)
	}, true)
	// Только файлы, на которые ещё ссылаются строки получившейся копии.
	needed := map[string]bool{}
	for name, t := range out {
		if name == "blobs" {
			continue
		}
		for i := range t.Rows {
			for col, v := range rowMap(t, i) {
				if isBlobColumn(col) && str(v) != "" {
					needed[str(v)] = true
				}
			}
		}
	}
	filter("blobs", func(r map[string]any) bool { return needed[str(r["id"])] }, false)
	var ids []string
	for _, id := range data.BlobIDs {
		if needed[id] {
			ids = append(ids, id)
		}
	}
	data.BlobIDs = ids
	data.Posts = 0
	for i := range out["posts"].Rows {
		if asInt64(rowMap(out["posts"], i)["deleted"]) == 0 {
			data.Posts++
		}
	}
	data.Tables = nil
	for _, original := range fullArchiveQueries {
		if t, ok := out[original.name]; ok {
			data.Tables = append(data.Tables, t)
		}
	}
	data.Tables = append(data.Tables, out["blobs"])
	for i := range data.Tables {
		if data.Tables[i].Name == "circles" {
			for j, col := range data.Tables[i].Columns {
				if col == "text_bytes" && len(data.Tables[i].Rows) > 0 {
					data.Tables[i].Rows[0][j] = archiveTextBytes(data)
				}
			}
		}
	}
	return data, nil
}

func archiveTextBytes(data *FullArchiveData) (bytes int64) {
	for _, t := range data.Tables {
		for i := range t.Rows {
			r := rowMap(t, i)
			length := func(col string) int64 {
				if r[col] == nil {
					return 0
				}
				return int64(len(fmt.Sprint(r[col])))
			}
			switch t.Name {
			case "posts":
				bytes += 550 + 3*length("body") + length("author_name")
			case "comments":
				bytes += 560 + 3*length("body") + length("author_name")
			case "reactions":
				bytes += 450 + length("emoji") + length("author_name")
			case "events":
				bytes += 400 + length("payload") + length("summary") + length("actor_name")
			case "day_titles":
				bytes += 250 + 3*length("title")
			}
		}
	}
	return bytes
}

// OwnerArchiveEstimate оценивает тот же состав, который попадёт в файл.
func (c *Chronicle) OwnerArchiveEstimate(ctx context.Context, circleID string) (bytes int64, posts, members int, full bool, err error) {
	scope, err := c.ArchiveScope(ctx, circleID)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if scope.All {
		bytes, posts, _, err = c.CircleArchiveEstimate(ctx, circleID)
		if err != nil {
			return 0, 0, 0, false, err
		}
		err = c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identities WHERE circle_id = ?`, circleID).Scan(&members)
		return bytes, posts, members, true, err
	}
	data, err := c.OwnerArchiveData(ctx, circleID)
	if err != nil {
		return 0, 0, 0, false, err
	}
	bytes = archiveTextBytes(data)
	for _, t := range data.Tables {
		for i := range t.Rows {
			r := rowMap(t, i)
			switch t.Name {
			case "blobs":
				bytes += asInt64(r["size_bytes"])
			case "identities":
				members++
			}
		}
	}
	return bytes, data.Posts, members, !data.PartialJournal, nil
}

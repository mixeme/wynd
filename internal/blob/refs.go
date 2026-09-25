package blob

import (
	"context"
	"database/sql"
	"strings"
)

// Querier is satisfied by *sql.DB and *sql.Tx.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Кто ссылается на блоб. Истина — сами ссылающиеся таблицы, а не blob_refs:
// та лишь дублирует их и отстаёт (план 42, раздел B «Ссылки на блобы»).
// Раньше предикат был написан трижды — в рутине, в GC и в оплате — и ни одна
// копия не была полной: рутина не знала про аватары и стирала их через сутки
// после загрузки (BLB-1), GC и оплата не знали про скриншоты оплаты и дни.
var referencingTables = []struct {
	table string
	where string
	// payScreenshot: строка pay_requests держит блоб внешним ключом даже
	// после удаления файла, поэтому для строки blobs она считается всегда,
	// а для файла на диске — только пока blob_deleted = 0.
	payScreenshot bool
}{
	{table: "blob_refs", where: "blob_id = %s"},
	{table: "post_media", where: "blob_id = %s"},
	{table: "day_covers", where: "blob_id = %s AND deleted = 0"},
	{table: "days", where: "cover_blob_id = %s"},
	{table: "identity_names", where: "avatar_blob_id = %s AND erased_at IS NULL"},
	{table: "pay_requests", where: "blob_id = %s", payScreenshot: true},
}

// referencedPredicate builds a SQL boolean over column col (e.g. "b.id").
// fileOnly отвечает на вопрос «нужен ли ещё файл на диске», иначе — «можно ли
// удалить строку blobs».
func referencedPredicate(col string, fileOnly bool) string {
	parts := make([]string, 0, len(referencingTables))
	for _, t := range referencingTables {
		where := strings.ReplaceAll(t.where, "%s", col)
		if t.payScreenshot && fileOnly {
			where += " AND blob_deleted = 0"
		}
		parts = append(parts, "EXISTS (SELECT 1 FROM "+t.table+" WHERE "+where+")")
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// ReferencedPredicate returns the SQL predicate "на блоб кто-то ссылается" for
// a blob id column, for callers that select many blobs at once.
func ReferencedPredicate(col string) string {
	return referencedPredicate(col, false)
}

// IsReferenced reports whether the blobs row may not be deleted yet.
func IsReferenced(ctx context.Context, q Querier, blobID string) (bool, error) {
	return queryPredicate(ctx, q, referencedPredicate("?", false), blobID)
}

// IsFileNeeded reports whether the file on disk is still needed. Отличается от
// IsReferenced одним случаем: скриншот оплаты, файл которого уже удалён
// админом, продолжает держать строку blobs, но не файл.
func IsFileNeeded(ctx context.Context, q Querier, blobID string) (bool, error) {
	return queryPredicate(ctx, q, referencedPredicate("?", true), blobID)
}

func queryPredicate(ctx context.Context, q Querier, pred, blobID string) (bool, error) {
	args := make([]any, strings.Count(pred, "?"))
	for i := range args {
		args[i] = blobID
	}
	var referenced int
	if err := q.QueryRowContext(ctx, `SELECT `+pred, args...).Scan(&referenced); err != nil {
		return false, err
	}
	return referenced == 1, nil
}

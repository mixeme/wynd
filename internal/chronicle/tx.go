package chronicle

import (
	"context"
	"database/sql"
)

// beginWrite opens a write transaction.
//
// Немедленная блокировка включена на уровне подключения (`_txlock=immediate`
// в DSN): все транзакции хроники были deferred, а при повышении read→write
// SQLite отдаёт SQLITE_BUSY сразу, и busy_timeout не помогает — ждать уже
// поздно (QLT-2). Хелпер оставлен один на пакет, чтобы пишущие пути
// открывались единообразно.
func (c *Chronicle) beginWrite(ctx context.Context) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, nil)
}

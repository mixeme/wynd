-- 0020: докуда участник видел «Отклики» (3.12–3.13).
--
-- Отдельно от read_cursors: там записи ленты, здесь комментарии, реакции,
-- названия и обложки дней. Число на вкладке и точка на улочке — то, что
-- новее этой отметки.
--
-- Всё, что случилось до обновления, считается просмотренным: иначе после
-- выхода версии каждый круг показал бы все отклики за всю историю как новые.

CREATE TABLE response_cursors (
    account_id TEXT NOT NULL,
    circle_id TEXT NOT NULL REFERENCES circles(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (account_id, circle_id)
);

INSERT INTO response_cursors (account_id, circle_id, seq, updated_at)
SELECT m.account_id, m.circle_id,
       COALESCE((SELECT MAX(e.seq) FROM events e WHERE e.circle_id = m.circle_id), 0),
       strftime('%Y-%m-%dT%H:%M:%f', 'now') || '000000Z'
FROM memberships m;

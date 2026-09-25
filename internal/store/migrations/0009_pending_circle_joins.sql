-- Deferred circle join after invite verify (name collected on /circles/{id}/join).

CREATE TABLE pending_circle_joins (
  account_id TEXT NOT NULL,
  circle_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (account_id, circle_id),
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
  FOREIGN KEY (circle_id) REFERENCES circles(id) ON DELETE CASCADE
);

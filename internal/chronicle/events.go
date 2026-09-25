package chronicle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type appendEventInput struct {
	circleID        string
	eventType       string
	isService       bool
	actorIdentityID string
	actorName       string
	targetID        string
	payload         any
	summary         string
	now             time.Time
}

func (c *Chronicle) appendEvent(ctx context.Context, tx *sql.Tx, in appendEventInput) (Event, error) {
	eventID, err := newID()
	if err != nil {
		return Event{}, err
	}
	payload := "{}"
	if in.payload != nil {
		b, err := json.Marshal(in.payload)
		if err != nil {
			return Event{}, fmt.Errorf("marshal payload: %w", err)
		}
		payload = string(b)
	}
	created := formatTime(in.now)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO events (
			id, circle_id, event_type, is_service,
			actor_identity_id, actor_name, target_id, payload, summary, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, eventID, in.circleID, in.eventType, boolToInt(in.isService),
		nullString(in.actorIdentityID), in.actorName, nullString(in.targetID),
		payload, in.summary, created)
	if err != nil {
		return Event{}, fmt.Errorf("insert event: %w", err)
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("event seq: %w", err)
	}
	return Event{
		Seq:             seq,
		ID:              eventID,
		CircleID:        in.circleID,
		Type:            in.eventType,
		IsService:       in.isService,
		ActorIdentityID: in.actorIdentityID,
		ActorName:       in.actorName,
		TargetID:        in.targetID,
		Payload:         payload,
		Summary:         in.summary,
		CreatedAt:       in.now.UTC(),
	}, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

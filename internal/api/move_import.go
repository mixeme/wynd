package api

import (
	"context"
	"encoding/json"
	"errors"
	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"net/url"
	"time"
)

// reviveIncoming поднимает круг из забранного файла: каждый файл — в
// хранилище от имени владельца со сверкой суммы, строки — в ReviveCircle;
// затем «принято и сверено» прежнему серверу.
func (s *Server) reviveIncoming(ctx context.Context, m *chronicle.IncomingMove) error {
	if m.CircleID == "" {
		out, err := s.reviveMoveFile(ctx, m)
		if err != nil {
			return err
		}
		if err := s.Chronicle.SetIncomingRevived(ctx, m.ID, out.CircleID, out.Posts, out.Names, out.MediaBytes, out.Verified, out.Invited, out.NotJoined, out.AlreadyHere); err != nil {
			return err
		}
		m.ResultPosts, m.ResultNames, m.ResultBytes = out.Posts, out.Names, out.MediaBytes
	}
	s.acceptMove(ctx, m.Link, m.ResultPosts, m.ResultNames, m.ResultBytes)
	return s.Chronicle.FinishIncoming(ctx, m.ID)
}

func (s *Server) reviveMoveFile(ctx context.Context, m *chronicle.IncomingMove) (reviveOutcome, error) {
	s.moveImportOnce.Do(func() { s.moveImportGate = make(chan struct{}, 1) })
	select {
	case s.moveImportGate <- struct{}{}:
		defer func() { <-s.moveImportGate }()
	case <-ctx.Done():
		return reviveOutcome{}, ctx.Err()
	}
	zr, err := archive.OpenFullArchiveWithLimits(s.incomingPath(m.ID), archive.ReadLimits{MediaBytes: uint64(s.moveMaxBytes())})
	if err != nil {
		return reviveOutcome{}, &moveFailure{"archive_invalid"}
	}
	defer zr.Close()
	raw, err := zr.Tables()
	if err != nil {
		return reviveOutcome{}, &moveFailure{"archive_invalid"}
	}
	tables := map[string]chronicle.FullArchiveTable{}
	for name, t := range raw {
		tables[name] = chronicle.FullArchiveTable{Name: name, Columns: t.Columns, Rows: t.Rows}
	}
	if err := s.Chronicle.ValidateReviveTables(ctx, zr.Manifest.FormatVersion, zr.Manifest.SchemaVersion, tables); err != nil {
		return reviveOutcome{}, &moveFailure{"archive_invalid"}
	}
	blobMap, err := s.importMoveBlobs(ctx, m.ID, m.AccountID, zr, tables["blobs"])
	if err != nil {
		return reviveOutcome{}, err
	}
	host := m.SourceURL
	if u, err := url.Parse(m.SourceURL); err == nil && u.Host != "" {
		host = u.Host
	}
	var nm *reviveNewMember
	if m.NewMember != "" {
		nm = &reviveNewMember{}
		if err := json.Unmarshal([]byte(m.NewMember), nm); err != nil {
			nm = nil
		}
	}
	newMember, greeting, err := nm.parse()
	if err != nil {
		newMember, greeting = nil, ""
	}
	return s.reviveFromTables(ctx, reviveRequest{
		NewMember:     newMember,
		Greeting:      greeting,
		GreetingDay:   entryDayOf(nm),
		AccountID:     m.AccountID,
		FormatVersion: zr.Manifest.FormatVersion,
		SchemaVersion: zr.Manifest.SchemaVersion,
		Posts:         zr.Manifest.Posts,
		Tables:        tables,
		BlobMap:       blobMap,
		SendInvites:   m.SendInvites,
		MovedFrom:     host,
	})
}

// importMoveBlobs кладёт файлы круга в хранилище. Файл, который у этого
// человека уже лежит с той же суммой, второй раз не кладётся.
func (s *Server) importMoveBlobs(ctx context.Context, moveID, accountID string, zr *archive.FullReader, t chronicle.FullArchiveTable) (map[string]string, error) {
	col := func(name string) int {
		for i, c := range t.Columns {
			if c == name {
				return i
			}
		}
		return -1
	}
	id, sum, size, mime, fname := col("id"), col("sha256"), col("size_bytes"), col("mime_type"), col("original_filename")
	if len(t.Rows) > 0 && (id < 0 || sum < 0 || size < 0) {
		return nil, &moveFailure{"archive_invalid"}
	}
	str := func(row []any, i int) string {
		if i < 0 {
			return ""
		}
		v, _ := row[i].(string)
		return v
	}
	var sums []string
	for _, row := range t.Rows {
		sums = append(sums, str(row, sum))
	}
	have, err := s.Blobs.OwnBlobsBySHA(ctx, accountID, sums)
	if err != nil {
		return nil, err
	}
	var needed int64
	seen := map[string]bool{}
	for _, row := range t.Rows {
		sha := str(row, sum)
		if _, exists := have[sha]; !exists && !seen[sha] {
			needed += jsonInt64(row[size]) // validated nonnegative, nonoverflowing total
			seen[sha] = true
		}
	}
	if err := s.reserveMoveDisk(ctx, moveID, needed); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, row := range t.Rows {
		old := str(row, id)
		if nw, ok := have[str(row, sum)]; ok {
			out[old] = nw
			continue
		}
		rc, err := zr.OpenMedia(old)
		if err != nil {
			return nil, &moveFailure{"archive_invalid"}
		}
		b, err := s.Blobs.ImportBlob(ctx, blob.ImportInput{
			AccountID: accountID, SHA256: str(row, sum), SizeBytes: jsonInt64(row[size]),
			MimeType: str(row, mime), OriginalFilename: str(row, fname), Now: time.Now(),
		}, moveContextReader{ctx: ctx, reader: rc})
		rc.Close()
		if errors.Is(err, blob.ErrInvalid) {
			return nil, &moveFailure{"archive_invalid"}
		}
		if err != nil {
			return nil, err
		}
		out[old] = b.ID
		have[b.SHA256] = b.ID
	}
	return out, nil
}

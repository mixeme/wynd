package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

// Оживление круга из полного архива (план 52). Устройство расшифровало и
// прочитало файл, загрузило файлы круга обычными загрузками частями и
// присылает строки таблиц и соответствие «блоб архива → блоб сервера».

// maxReviveBody — строки таблиц круга одним JSON: на большом круге это
// мегабайты, обычные 1 МБ запросов сюда не годятся. Файлы идут загрузками.
const maxReviveBody = 256 << 20

type reviveTable struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type reviveBody struct {
	Manifest struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
		SchemaVersion int    `json:"schema_version"`
		Posts         int    `json:"posts"`
	} `json:"manifest"`
	Tables      map[string]reviveTable `json:"tables"`
	BlobMap     map[string]string      `json:"blob_map"`
	SendInvites bool                   `json:"send_invites"`
	// NewMember — «Аня — это вы?» → «Нет» (7.29в, A12): имя, пол и
	// приветствие того, кто оживляет. Нет поля — «Да, это я».
	NewMember *reviveNewMember `json:"new_member,omitempty"`
}

type reviveNewMember struct {
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Body   string `json:"body"`
}

// parse — имя и пол для круга; приветствие — отдельно, оно пишется после
// строки «оживил круг».
func (m *reviveNewMember) parse() (*chronicle.ReviveNewMember, string, error) {
	if m == nil {
		return nil, "", nil
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		return nil, "", chronicle.ErrInvalid
	}
	gender, err := chronicle.ParseGender(m.Gender)
	if err != nil {
		return nil, "", err
	}
	return &chronicle.ReviveNewMember{Name: name, Gender: gender}, strings.TrimSpace(m.Body), nil
}

// handleReviveBlobs — какие файлы архива у этого человека уже есть на
// сервере (та же сумма и длина): их второй раз не грузят. Только свои —
// поиск по чужим выдавал бы, лежит ли на сервере такая фотография.
func (s *Server) handleReviveBlobs(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		SHA256 []string `json:"sha256"`
	}](w, r)
	if !ok {
		return
	}
	found, err := s.Blobs.OwnBlobsBySHA(r.Context(), sess.AccountID, body.SHA256)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blobs": found})
}

// reviveRoom — сколько места этот сервер даёт новому кругу этой учётки (7.29:
// «для круга есть 3 ГБ»): квота круга по умолчанию или одобренная просьба о
// месте для него (7.31), но не больше, чем свободно на сервере. approved —
// одобренная просьба, если её место и считается.
func (s *Server) reviveRoom(ctx context.Context, accountID string) (room int64, approved *blob.QuotaRequest, last *blob.QuotaRequest, err error) {
	if room, err = s.Blobs.RoomBytes(ctx); err != nil {
		return 0, nil, nil, err
	}
	def, err := s.Blobs.DefaultCircleQuotaBytes(ctx)
	if err != nil {
		return 0, nil, nil, err
	}
	if last, err = s.Blobs.LastPlaceRequest(ctx, accountID); err != nil {
		return 0, nil, nil, err
	}
	switch {
	case last != nil && last.Status == "approved":
		approved = last
		room = min(room, last.RequestedBytes)
	case def.Valid:
		room = min(room, def.Int64)
	}
	return room, approved, last, nil
}

// roomLimit — чем ограничено место для круга (владелец, 2026-10-17: «если на
// сервере доступно только 100 МБ, откуда нужный объём возьмёт админ?»):
// "quota" — квотой круга по умолчанию, "approved" — одобренной просьбой,
// хотя диск свободнее; "disk" — самим диском. disk_bytes — свободно на сервере.
func (s *Server) roomLimit(ctx context.Context, out map[string]any, room int64, approved bool) error {
	disk, err := s.Blobs.RoomBytes(ctx)
	if err != nil {
		return err
	}
	out["disk_bytes"] = disk
	if room < disk && approved {
		out["limit"] = "approved"
	} else if room < disk {
		out["limit"] = "quota"
	} else {
		out["limit"] = "disk"
	}
	return nil
}

// handleReviveRoom — помещается ли круг из архива сюда (7.29, 7.30) и что с
// просьбой о месте для него (7.31, 7.32). Экран спрашивает при открытии.
func (s *Server) handleReviveRoom(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	need, _ := strconv.ParseInt(r.URL.Query().Get("bytes"), 10, 64)
	room, approved, last, err := s.reviveRoom(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := map[string]any{"room_bytes": room, "fits": need <= room}
	if err := s.roomLimit(r.Context(), out, room, approved != nil); err != nil {
		writeError(w, err)
		return
	}
	if last != nil {
		req := map[string]any{
			"status": last.Status, "requested_bytes": last.RequestedBytes,
			"circle_name": last.CircleName, "created_at": last.CreatedAt,
		}
		if last.ResolvedAt != nil {
			req["resolved_at"] = *last.ResolvedAt
		}
		out["request"] = req
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRevivePlaceRequest — «Попросить место» на 7.30: просьба о месте для
// круга, которого пока нет; встаёт в ту же очередь панели.
func (s *Server) handleRevivePlaceRequest(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[struct {
		Bytes      int64  `json:"bytes"`
		CircleName string `json:"circle_name"`
	}](w, r)
	if !ok {
		return
	}
	id, err := s.Blobs.CreatePlaceRequest(r.Context(), sess.AccountID, body.CircleName, body.Bytes)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyAdminQuota(id)
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "pending"})
}

// handleReviveCircle — «Оживить круг здесь» (7.29): круг вставляется заново,
// тот, кто оживил, — владелец; итог — для 7.33 / 7.34.
func (s *Server) handleReviveCircle(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	var body reviveBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Manifest.Format != archive.FullFormat || !archive.FullFormatKnown(body.Manifest.FormatVersion) {
		writeDomainError(w, chronicle.ErrInvalid)
		return
	}
	newMember, greeting, err := body.NewMember.parse()
	if err != nil {
		writeError(w, err)
		return
	}
	tables := map[string]chronicle.FullArchiveTable{}
	for name, t := range body.Tables {
		tables[name] = chronicle.FullArchiveTable{Name: name, Columns: t.Columns, Rows: t.Rows}
	}
	out, err := s.reviveFromTables(r.Context(), reviveRequest{
		NewMember:     newMember,
		Greeting:      greeting,
		AccountID:     sess.AccountID,
		FormatVersion: body.Manifest.FormatVersion,
		SchemaVersion: body.Manifest.SchemaVersion,
		Posts:         body.Manifest.Posts,
		Tables:        tables,
		BlobMap:       body.BlobMap,
		SendInvites:   body.SendInvites,
	})
	var re *reviveError
	if errors.As(err, &re) {
		writeJSON(w, re.status, map[string]string{"error": re.code, "detail": re.detail})
		return
	}
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"circle_id":  out.CircleID,
		"posts":      out.Posts,
		"names":      out.Names,
		"bytes":      out.Bytes,
		"verified":   out.Verified,
		"invited":    out.Invited,
		"not_joined": out.NotJoined,
		// Уже на этом сервере: письмо им не нужно, круг у них в списке.
		"already_here": out.AlreadyHere,
	})
}

// reviveRequest — круг из строк архива: от устройства (оживление) или от
// самого сервера (перенос, план 53).
type reviveRequest struct {
	AccountID     string
	FormatVersion int
	SchemaVersion int
	Posts         int
	Tables        map[string]chronicle.FullArchiveTable
	BlobMap       map[string]string
	SendInvites   bool
	MovedFrom     string
	// NewMember и Greeting — оживляет не прежний владелец (A12, 7.29в).
	NewMember *chronicle.ReviveNewMember
	Greeting  string
}

// reviveOutcome — итог для 7.25 / 7.33 / 7.26 / 7.34.
type reviveOutcome struct {
	CircleID string
	Posts    int
	Names    int
	// Bytes — для экрана: файлы и текст, как считает квота (C42).
	Bytes int64
	// MediaBytes — только файлы: их сверяет прежний сервер при переносе.
	MediaBytes  int64
	Verified    bool
	Invited     int
	NotJoined   int
	AlreadyHere int
}

// reviveError — отказ с кодом для экрана: файла нет, сумма не та, места нет.
type reviveError struct {
	status       int
	code, detail string
}

func (e *reviveError) Error() string { return "revive: " + e.code + " " + e.detail }

// reviveFromTables проверяет файлы и место и вставляет круг.
func (s *Server) reviveFromTables(ctx context.Context, in reviveRequest) (reviveOutcome, error) {
	var out reviveOutcome
	tables := in.Tables
	accounts := map[string]string{}
	if t, ok := tables["accounts"]; ok {
		id, email := slices.Index(t.Columns, "id"), slices.Index(t.Columns, "email")
		for _, row := range t.Rows {
			if id < 0 || email < 0 {
				break
			}
			a, _ := row[id].(string)
			e, _ := row[email].(string)
			if a != "" && strings.Contains(e, "@") {
				accounts[a] = e
			}
		}
	}
	// Каждый файл архива — на сервере, своей суммой и длиной (7.33: «сверено
	// с файлом архива, ничего не потерялось»).
	var archiveBytes int64
	if t, ok := tables["blobs"]; ok {
		id, sum, size := slices.Index(t.Columns, "id"), slices.Index(t.Columns, "sha256"), slices.Index(t.Columns, "size_bytes")
		for _, row := range t.Rows {
			old, _ := row[id].(string)
			nw, ok := in.BlobMap[old]
			if !ok {
				return out, &reviveError{http.StatusBadRequest, "blob_missing", old}
			}
			gotSum, gotSize, owner, err := s.Blobs.BlobDigest(ctx, nw)
			if err != nil || owner != in.AccountID {
				return out, &reviveError{http.StatusBadRequest, "blob_missing", old}
			}
			wantSum, _ := row[sum].(string)
			if gotSum != wantSum || gotSize != jsonInt64(row[size]) {
				return out, &reviveError{http.StatusBadRequest, "blob_mismatch", old}
			}
			archiveBytes += gotSize
		}
	}
	// Помещается ли (7.30). Файлы уже загружены — место на сервере ими занято;
	// предел здесь — нового круга: одобренная просьба или квота по умолчанию.
	_, approved, _, err := s.reviveRoom(ctx, in.AccountID)
	if err != nil {
		return out, err
	}
	limit := int64(-1)
	if approved != nil {
		limit = approved.RequestedBytes
	} else if def, err := s.Blobs.DefaultCircleQuotaBytes(ctx); err == nil && def.Valid {
		limit = def.Int64
	}
	if limit >= 0 && archiveBytes > limit {
		return out, &reviveError{http.StatusInsufficientStorage, "no_room", ""}
	}
	res, err := s.Chronicle.ReviveCircle(ctx, chronicle.ReviveInput{
		FormatVersion: in.FormatVersion,
		SchemaVersion: in.SchemaVersion,
		Tables:        tables,
		Accounts:      accounts,
		BlobMap:       in.BlobMap,
		ReviverID:     in.AccountID,
		Now:           time.Now(),
		MovedFrom:     in.MovedFrom,
		NewMember:     in.NewMember,
	})
	if err != nil {
		if errors.Is(err, chronicle.ErrInvalid) {
			log.Printf("revive: %v", err)
		}
		return out, err
	}
	if approved != nil {
		// Место дали под этот круг: оно и есть его квота.
		if err := s.Blobs.SetCircleQuotaAdmin(ctx, res.CircleID, true, &approved.RequestedBytes); err != nil {
			log.Printf("revive %s: quota: %v", res.CircleID, err)
		}
		if err := s.Blobs.UsePlaceRequest(ctx, approved.ID); err != nil {
			log.Printf("revive %s: place request: %v", res.CircleID, err)
		}
	}
	if in.SendInvites {
		out.Invited = s.inviteNotJoined(res.CircleID, in.AccountID, res.NotJoined)
	}
	// Приветствие нового человека — после строки «оживил круг», как при
	// вступлении (7.29в: «Скажите что-нибудь кругу»). Не вышло — круг уже
	// ожил, это не повод говорить об отказе.
	if in.NewMember != nil && in.Greeting != "" {
		now := time.Now().UTC()
		if _, err := s.Chronicle.CreatePost(ctx, chronicle.PostInput{
			CircleID: res.CircleID, AccountID: in.AccountID, Body: in.Greeting,
			EntryDate: now.Format("2006-01-02"), Now: now,
		}); err != nil {
			log.Printf("revive %s: greeting: %v", res.CircleID, err)
		}
	}
	// На экран — сколько круг занимает по квоте, с текстом (C42); сверка
	// с архивом — по файлам, их размер архив знает точно.
	out.CircleID, out.Posts, out.Names, out.Bytes = res.CircleID, res.Posts, res.Names, res.Bytes+res.TextBytes
	out.MediaBytes = res.Bytes
	out.Verified = res.Posts == in.Posts && res.Bytes == archiveBytes
	out.NotJoined = len(res.NotJoined)
	out.AlreadyHere = res.AlreadyHere
	return out, nil
}

// jsonInt64 — число из строки JSON-таблицы: float64 после encoding/json,
// json.Number или целое — если таблицу собрал сам сервер.
func jsonInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}

// inviteNotJoined — письмо каждому, кто приехал с кругом, но на этом сервере
// ещё не был (7.29, 6.6): войти той же почтой — и он в своём имени.
func (s *Server) inviteNotJoined(circleID, ownerAccountID string, accountIDs []string) int {
	if s.Mail == nil || len(accountIDs) == 0 {
		return 0
	}
	link := strings.TrimRight(s.PublicURL(), "/") + "/"
	s.notifyWG.Add(1)
	go func() {
		defer s.notifyWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		name, err := s.Chronicle.CircleName(ctx, circleID)
		if err != nil {
			log.Printf("revive invites %s: %v", circleID, err)
			return
		}
		owner, _ := s.Chronicle.OwnerName(ctx, circleID)
		for _, id := range accountIDs {
			email, err := s.Chronicle.AccountEmail(ctx, id)
			if err != nil {
				continue
			}
			if err := s.Mail.SendCircleWaiting(ctx, email, name, owner, link); err != nil {
				log.Printf("revive invite %s to %s: %v", circleID, email, err)
			}
		}
	}()
	return len(accountIDs)
}

// withReviveBody — участник с оплатой и телом до maxReviveBody.
func (s *Server) withReviveBody(next http.HandlerFunc) http.HandlerFunc {
	return s.RequireParticipantStream(func(w http.ResponseWriter, r *http.Request) {
		if err := s.requirePaidSession(r); err != nil {
			writeError(w, err)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxReviveBody)
		}
		next(w, r)
	})
}

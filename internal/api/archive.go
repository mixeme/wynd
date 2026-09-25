package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/archive"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
)

func (s *Server) archiveCycleJSON(ctx context.Context, circleID, accountID string) (map[string]any, error) {
	cycle, err := s.Chronicle.GetArchiveCycle(ctx, circleID)
	if err != nil {
		return nil, err
	}
	if !cycle.Active {
		return nil, nil
	}
	mediaBytes, err := s.Chronicle.EstimateArchiveMediaBytes(ctx, circleID, accountID, cycle.CutoffDate)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"active":                 true,
		"cutoff_date":            cycle.CutoffDate,
		"deadline":               cycle.Deadline.UTC().Format(time.RFC3339),
		"reminder_before_sec":    cycle.ReminderBeforeSec,
		"cutoff_locked":          cycle.CutoffLockedAt != nil,
		"personal_archive_bytes": mediaBytes,
		"download_url":           "/api/v1/circles/" + circleID + "/archive/download",
	}
	return out, nil
}

func (s *Server) handleCircleDetail(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	var found *chronicle.CircleSummary
	for i := range circles {
		if circles[i].ID == circleID {
			found = &circles[i]
			break
		}
	}
	if found == nil {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	banner, err := s.archiveCycleJSON(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	mem, err := s.Chronicle.MembershipForAccount(r.Context(), circleID, sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	owner, err := s.Chronicle.CircleOwnerAccount(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	editWindow, err := s.Chronicle.CircleEditWindow(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	color, err := s.Chronicle.CircleColor(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	identityName, err := s.Chronicle.ResolveIdentityName(r.Context(), mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	avatarBlobID, err := s.Chronicle.ResolveIdentityAvatar(r.Context(), mem.IdentityID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	inviteSettings, err := s.Chronicle.GetInviteSettings(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"id":                    found.ID,
		"name":                  found.Name,
		"color":                 color,
		"status":                string(found.Status),
		"edit_window_sec":       editWindow.Seconds,
		"is_owner":              owner == sess.AccountID,
		"can_settings":          mem.CanSettings,
		"identity_id":           mem.IdentityID,
		"identity_name":         identityName,
		"invite_who":            inviteSettings.InviteWho,
		"invite_kind_default":   inviteSettings.InviteKindDefault,
	}
	if avatarBlobID != "" {
		out["avatar_blob_id"] = avatarBlobID
	}
	if banner != nil {
		out["archive_cycle"] = banner
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCircleQuota(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	if err := s.Chronicle.RequireOwner(r.Context(), circleID, sess.AccountID); err != nil {
		writeDomainError(w, err)
		return
	}
	used, err := s.Blobs.CircleUsedBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	quota, err := s.Blobs.CircleQuotaBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	volume, err := s.Chronicle.MediaVolumeChart(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	median, err := s.Chronicle.MedianPostBytes(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := map[string]any{
		"used_bytes": used,
		"volume":     volume,
	}
	if quota.Valid {
		out["quota_bytes"] = quota.Int64
	}
	if median > 0 {
		out["median_post_bytes"] = median
	}
	if cutoff := r.URL.Query().Get("cutoff_date"); cutoff != "" {
		freed, err := s.Chronicle.FreedBytesBeforeCutoff(r.Context(), circleID, cutoff)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out["freed_at_cutoff_bytes"] = freed
	}
	writeJSON(w, http.StatusOK, out)
}

type startArchiveBody struct {
	CutoffDate        string `json:"cutoff_date"`
	Deadline          string `json:"deadline"`
	ReminderBeforeSec int64  `json:"reminder_before_sec"`
}

func (s *Server) handleStartArchiveCycle(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body startArchiveBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	deadline, err := time.Parse(time.RFC3339, body.Deadline)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	now := time.Now().UTC()
	if err := s.Chronicle.StartArchiveCycle(r.Context(), circleID, sess.AccountID, body.CutoffDate, deadline, body.ReminderBeforeSec, now); err != nil {
		writeDomainError(w, err)
		return
	}
	s.sendArchiveCycleStartEmails(r.Context(), circleID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type moveCutoffBody struct {
	CutoffDate string `json:"cutoff_date"`
}

func (s *Server) handleMoveCutoff(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body moveCutoffBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	err := s.Chronicle.MoveCutoff(r.Context(), circleID, sess.AccountID, body.CutoffDate, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type moveDeadlineBody struct {
	Deadline string `json:"deadline"`
}

func (s *Server) handleMoveDeadline(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	var body moveDeadlineBody
	if err := readJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	deadline, err := time.Parse(time.RFC3339, body.Deadline)
	if err != nil {
		writeError(w, chronicle.ErrInvalid)
		return
	}
	err = s.Chronicle.MoveDeadline(r.Context(), circleID, sess.AccountID, deadline, time.Now().UTC())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleArchiveDownload(w http.ResponseWriter, r *http.Request) {
	circleID := r.PathValue("circle_id")
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, chronicle.ErrForbidden)
		return
	}
	cycle, err := s.Chronicle.GetArchiveCycle(r.Context(), circleID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if !cycle.Active {
		writeDomainError(w, chronicle.ErrNotFound)
		return
	}
	now := time.Now().UTC()
	if !cycle.Deadline.IsZero() && now.After(cycle.Deadline) {
		writeDomainError(w, chronicle.ErrForbidden)
		return
	}
	layout := archive.LayoutFeed
	if v := strings.TrimSpace(r.URL.Query().Get("layout")); v == "posts" {
		layout = archive.LayoutPosts
	}
	posts, err := s.Chronicle.ArchiveSnapshot(r.Context(), circleID, sess.AccountID, cycle.CutoffDate)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	circles, err := s.Chronicle.ListAccountCircles(r.Context(), sess.AccountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	circleName := circleID
	for _, c := range circles {
		if c.ID == circleID {
			circleName = c.Name
			break
		}
	}
	avatars, err := s.Chronicle.IdentityAvatarBlobIDs(r.Context(), chronicle.IdentityIDsFromFeed(posts))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	data, err := archive.BuildPersonalArchive(r.Context(), archive.BuildInput{
		CircleName: circleName,
		CutoffDate: cycle.CutoffDate,
		Layout:     layout,
		Posts:      posts,
		Blobs:      s.Blobs,
		Avatars:    avatars,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.Chronicle.LockCutoff(r.Context(), circleID, now); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"wynd-archive-"+circleID+".zip\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func (s *Server) sendArchiveCycleStartEmails(ctx context.Context, circleID string) {
	if s.Mail == nil {
		return
	}
	bg, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cycle, err := s.Chronicle.GetArchiveCycle(bg, circleID)
	if err != nil || !cycle.Active {
		return
	}
	emails, err := s.Chronicle.CircleMemberEmails(bg, circleID)
	if err != nil {
		return
	}
	download := strings.TrimRight(s.PublicURL, "/") + "/api/v1/circles/" + circleID + "/archive/download"
	for _, email := range emails {
		_ = s.Mail.SendArchiveCycleStart(bg, email, cycle.CutoffDate, cycle.Deadline, download)
	}
}

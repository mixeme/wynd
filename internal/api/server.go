package api

import (
	"net"
	"net/http"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/search"
)

type Server struct {
	Auth           *auth.Service
	Chronicle      *chronicle.Chronicle
	Blobs          *blob.Store
	Search         *search.Service
	Mail           *mail.Service
	Push           *push.Service
	BootstrapToken string
	DataDir        string
	PublicURL      string
	ListenAddr     string
	Loopback       bool
	// TrustedProxies are peers whose X-Forwarded-For is believed. Empty means
	// loopback only (a reverse proxy on the same host).
	TrustedProxies []*net.IPNet
	Mux            *http.ServeMux
}

func NewServer(authSvc *auth.Service, ch *chronicle.Chronicle, blobs *blob.Store, mailSvc *mail.Service, pushSvc *push.Service, bootstrapToken, dataDir, publicURL, listenAddr string, loopback bool) *Server {
	s := &Server{
		Auth:           authSvc,
		Chronicle:      ch,
		Blobs:          blobs,
		Search:         search.New(ch),
		Mail:           mailSvc,
		Push:           pushSvc,
		BootstrapToken: bootstrapToken,
		DataDir:        dataDir,
		PublicURL:      publicURL,
		ListenAddr:     listenAddr,
		Loopback:       loopback,
		Mux:            http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	public := limitBody
	s.Mux.HandleFunc("GET /api/v1/instance", s.handleInstance)
	s.Mux.HandleFunc("POST /api/v1/auth/register", public(s.handleRegister))
	s.Mux.HandleFunc("POST /api/v1/auth/code", public(s.handleRequestCode))
	s.Mux.HandleFunc("POST /api/v1/auth/verify", public(s.handleVerify))
	s.Mux.HandleFunc("GET /api/v1/invites/{token}", public(s.handlePeekInvite))
	s.Mux.HandleFunc("POST /api/v1/invites/{token}/accept", public(s.handleAcceptInvite))
	s.Mux.HandleFunc("POST /api/v1/admin/bootstrap", public(s.handleBootstrap))
	s.Mux.HandleFunc("POST /api/v1/admin/login", public(s.handleAdminLogin))
	admin := s.requireAdmin
	s.Mux.HandleFunc("POST /api/v1/admin/logout", admin(s.handleAdminLogout))
	s.Mux.HandleFunc("POST /api/v1/admin/invites", admin(s.handleCreateServerInvite))
	s.Mux.HandleFunc("GET /api/v1/admin/invites", admin(s.handleAdminListInvites))
	s.Mux.HandleFunc("DELETE /api/v1/admin/invites/{id}", admin(s.handleAdminRevokeInvite))
	s.Mux.HandleFunc("GET /api/v1/admin/storage", admin(s.handleAdminStorage))
	s.Mux.HandleFunc("PUT /api/v1/admin/storage/quota", admin(s.handleAdminSetStorageQuota))
	s.Mux.HandleFunc("PUT /api/v1/admin/storage/default_quota", admin(s.handleAdminSetDefaultQuota))
	s.Mux.HandleFunc("PUT /api/v1/admin/circles/{id}/quota", admin(s.handleAdminSetCircleQuota))
	s.Mux.HandleFunc("GET /api/v1/admin/compression", admin(s.handleAdminCompression))
	s.Mux.HandleFunc("PUT /api/v1/admin/compression", admin(s.handleAdminSetCompression))
	s.Mux.HandleFunc("GET /api/v1/admin/access", admin(s.handleAdminAccess))
	s.Mux.HandleFunc("PUT /api/v1/admin/access", admin(s.handleAdminSetAccess))
	s.Mux.HandleFunc("GET /api/v1/admin/accounts", admin(s.handleAdminAccounts))
	s.Mux.HandleFunc("GET /api/v1/admin/accounts/{id}", admin(s.handleAdminGetAccount))
	s.Mux.HandleFunc("DELETE /api/v1/admin/accounts/{id}", admin(s.handleAdminDeleteAccount))
	s.Mux.HandleFunc("POST /api/v1/admin/accounts/{id}/block", admin(s.handleAdminBlockAccount))
	s.Mux.HandleFunc("POST /api/v1/admin/accounts/{id}/unblock", admin(s.handleAdminUnblockAccount))
	s.Mux.HandleFunc("GET /api/v1/admin/smtp", admin(s.handleAdminSMTP))
	s.Mux.HandleFunc("PUT /api/v1/admin/smtp", admin(s.handleAdminSetSMTP))
	s.Mux.HandleFunc("POST /api/v1/admin/smtp/test", admin(s.handleAdminSMTPTest))
	s.Mux.HandleFunc("GET /api/v1/admin/push/vapid", admin(s.handleAdminVAPID))
	s.Mux.HandleFunc("POST /api/v1/admin/push/test", admin(s.handleAdminPushTest))
	s.Mux.HandleFunc("POST /api/v1/admin/check", admin(s.handleAdminCheck))
	s.Mux.HandleFunc("POST /api/v1/admin/routine", admin(s.handleAdminRunRoutine))
	s.Mux.HandleFunc("GET /api/v1/admin/proxy/{kind}", admin(s.handleAdminProxySnippet))
	s.Mux.HandleFunc("GET /api/v1/admin/quota_requests", admin(s.handleAdminQuotaRequests))
	s.Mux.HandleFunc("POST /api/v1/admin/quota_requests/{id}/approve", admin(s.handleAdminApproveQuotaRequest))
	s.Mux.HandleFunc("POST /api/v1/admin/quota_requests/{id}/reject", admin(s.handleAdminRejectQuotaRequest))

	participant := s.RequireParticipant
	s.Mux.HandleFunc("POST /api/v1/invites/{token}/join", participant(s.handleJoinInvite))
	s.Mux.HandleFunc("POST /api/v1/auth/logout", participant(s.handleLogout))
	s.Mux.HandleFunc("GET /api/v1/sync", participant(s.handleSync))
	s.Mux.HandleFunc("GET /api/v1/circles", participant(s.handleListCircles))
	s.Mux.HandleFunc("POST /api/v1/circles", participant(s.handleCreateCircle))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/invites", participant(s.handleCreateCircleInvite))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/leave", participant(s.handleLeaveCircle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/read_cursor", participant(s.handleSetReadCursor))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/feed", participant(s.handleFeed))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/grid", participant(s.handleGrid))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/map", participant(s.handleMap))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/days", participant(s.handleDays))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/days/{date}", participant(s.handleDayDetail))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/search", participant(s.handleCircleSearch))
	s.Mux.HandleFunc("GET /api/v1/search", participant(s.handleGlobalSearch))

	s.Mux.HandleFunc("POST /api/v1/uploads", participant(s.handleCreateUpload))
	s.Mux.HandleFunc("HEAD /api/v1/uploads/{session_id}", participant(s.handleUploadStatus))
	s.Mux.HandleFunc("PUT /api/v1/uploads/{session_id}", s.RequireParticipantStream(s.handleUploadChunk))
	s.Mux.HandleFunc("POST /api/v1/uploads/{session_id}/complete", participant(s.handleCompleteUpload))
	s.Mux.HandleFunc("GET /api/v1/blobs/{blob_id}", participant(s.handleServeBlob))

	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/posts", participant(s.handleCreatePost))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}/posts/{post_id}", participant(s.handleEditPost))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}", participant(s.handleDeletePost))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/posts/{post_id}/comments", participant(s.handleCreateComment))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}/posts/{post_id}/comments/{comment_id}", participant(s.handleEditComment))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}/comments/{comment_id}", participant(s.handleDeleteComment))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/posts/{post_id}/reactions", participant(s.handleSetReaction))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}/reactions", participant(s.handleDeleteReaction))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/days/{date}/title", participant(s.handleSetDayTitle))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/days/{date}/title", participant(s.handleClearDayTitle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/days/{date}/cover", participant(s.handleSetDayCover))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/days/{date}/cover", participant(s.handleClearDayCover))

	s.Mux.HandleFunc("GET /api/v1/notify_prefs", participant(s.handleGetAccountNotifyPrefs))
	s.Mux.HandleFunc("PUT /api/v1/notify_prefs", participant(s.handleSetAccountNotifyPrefs))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/notify_prefs", participant(s.handleGetCircleNotifyPrefs))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/notify_prefs", participant(s.handleSetCircleNotifyPrefs))
	s.Mux.HandleFunc("POST /api/v1/push/subscribe", participant(s.handlePushSubscribe))
	s.Mux.HandleFunc("DELETE /api/v1/push/subscribe", participant(s.handlePushUnsubscribe))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/quota_requests", participant(s.handleCreateQuotaRequest))

	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}", participant(s.handleCircleDetail))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}", participant(s.handlePatchCircle))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}", participant(s.handleDeleteCircle))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/members", participant(s.handleCircleMembers))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/identity", participant(s.handleUpdateIdentity))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/members/{account_id}", participant(s.handleSetMember))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/identity", participant(s.handleIdentityHistory))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/transfer", participant(s.handleTransferOwnership))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/exclude", participant(s.handleExcludeMember))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/quota", participant(s.handleCircleQuota))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/archive", participant(s.handleStartArchiveCycle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/archive/cutoff", participant(s.handleMoveCutoff))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/archive/deadline", participant(s.handleMoveDeadline))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/archive/download", participant(s.handleArchiveDownload))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.Mux.ServeHTTP(w, r)
}

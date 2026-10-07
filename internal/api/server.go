package api

import (
	"context"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/poster"
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
	ListenAddr     string
	// publicURL и loopback меняются из обработчика (bootstrap, панель) и
	// читаются другими запросами, поэтому закрыты мьютексом и atomic:
	// экспортированные поля здесь были гонкой данных (QLT-4).
	publicURL   string
	publicURLMu sync.RWMutex
	loopback    atomic.Bool
	// TrustedProxies are peers whose X-Forwarded-For is believed. Empty means
	// loopback only (a reverse proxy on the same host).
	TrustedProxies []*net.IPNet
	Mux            *http.ServeMux
	notifyWG       sync.WaitGroup
	probes         *probeLimiter
	// probeInstance — случайная метка процесса в GET /probe: браузер
	// возвращает её в проверке, и «Домен» узнаёт, что имя ведёт сюда,
	// даже когда исходящий адрес — адрес контейнера или NAT.
	probeInstance string
	// archiveBuilds — учётки, у которых сейчас собирается архив: ZIP целиком
	// в памяти, и N параллельных запросов одного участника держали N копий
	// среза (аудит 2026-09-22). Одна сборка на учётку, повтор — 429.
	archiveBuilds sync.Map
	// Posters снимает кадр ролику, пришедшему без него (план 49, временно —
	// до нативной обёртки). nil или без ffmpeg — ничего не делает.
	Posters *poster.Maker
	// ArchiveFonts — вшитая сборка клиента (web/dist): из неё архив берёт
	// Golos Text и кладёт внутрь ZIP. nil — архив на системном шрифте.
	ArchiveFonts fs.FS
}

// NewServer wires the services into one HTTP handler and registers every route on Mux.
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
		publicURL:      publicURL,
		ListenAddr:     listenAddr,
		Mux:            http.NewServeMux(),
		probes:         newProbeLimiter(),
		probeInstance:  newProbeInstance(),
	}
	s.loopback.Store(loopback)
	s.routes()
	return s
}

func (s *Server) routes() {
	public := limitBody
	s.Mux.HandleFunc("GET /api/v1/instance", s.handleInstance)
	s.Mux.HandleFunc("GET /api/v1/probe", s.handleProbe)
	s.Mux.HandleFunc("GET /api/v1/probe/sse", s.handleProbeSSE)
	s.Mux.HandleFunc("GET /api/v1/probe/stream", s.limitProbe(s.handleProbeStream))
	s.Mux.HandleFunc("PUT /api/v1/probe/body", s.limitProbe(s.handleProbeBody))
	s.Mux.HandleFunc("POST /api/v1/auth/register", public(s.handleRegister))
	s.Mux.HandleFunc("POST /api/v1/auth/code", public(s.handleRequestCode))
	s.Mux.HandleFunc("POST /api/v1/auth/verify", public(s.handleVerify))
	s.Mux.HandleFunc("GET /api/v1/invites/{token}", public(s.handlePeekInvite))
	s.Mux.HandleFunc("POST /api/v1/invites/{token}/accept", public(s.handleAcceptInvite))
	s.Mux.HandleFunc("POST /api/v1/admin/bootstrap", public(s.handleBootstrap))
	s.Mux.HandleFunc("POST /api/v1/admin/bootstrap/smtp-test", public(s.handleBootstrapSMTPTest))
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
	s.Mux.HandleFunc("PUT /api/v1/admin/password", admin(s.handleAdminSetPassword))
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
	s.Mux.HandleFunc("GET /api/v1/admin/proxy/{kind}", admin(s.handleAdminProxySnippet))
	s.Mux.HandleFunc("GET /api/v1/admin/quota_requests", admin(s.handleAdminQuotaRequests))
	s.Mux.HandleFunc("POST /api/v1/admin/quota_requests/{id}/approve", admin(s.handleAdminApproveQuotaRequest))
	s.Mux.HandleFunc("POST /api/v1/admin/quota_requests/{id}/reject", admin(s.handleAdminRejectQuotaRequest))
	s.Mux.HandleFunc("GET /api/v1/admin/pay", admin(s.handleAdminPayHub))
	s.Mux.HandleFunc("PUT /api/v1/admin/pay", admin(s.handleAdminSetPayRequisites))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/donate", admin(s.handleAdminPayDonate))
	s.Mux.HandleFunc("PUT /api/v1/admin/pay/donate", admin(s.handleAdminSetPayDonate))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/subscription", admin(s.handleAdminPaySubscription))
	s.Mux.HandleFunc("PUT /api/v1/admin/pay/subscription", admin(s.handleAdminSetPaySubscription))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/requests", admin(s.handleAdminPayRequests))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/requests/{id}", admin(s.handleAdminPayRequestByID))
	s.Mux.HandleFunc("POST /api/v1/admin/pay/requests/{id}/approve", admin(s.handleAdminApprovePayRequest))
	s.Mux.HandleFunc("POST /api/v1/admin/pay/requests/{id}/reject", admin(s.handleAdminRejectPayRequest))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/blob/{blob_id}", admin(s.handleAdminPayBlob))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/accounts", admin(s.handleAdminPayAccounts))
	s.Mux.HandleFunc("GET /api/v1/admin/pay/accounts/{id}", admin(s.handleAdminPayAccountByID))
	s.Mux.HandleFunc("PUT /api/v1/admin/pay/accounts/{id}", admin(s.handleAdminGrantPayAccount))

	participant := s.RequireParticipant
	paid := s.RequirePaidParticipant
	s.Mux.HandleFunc("POST /api/v1/invites/{token}/join", paid(s.handleJoinInvite))
	s.Mux.HandleFunc("POST /api/v1/invites/{token}/claim", paid(s.handleClaimInvite))
	s.Mux.HandleFunc("POST /api/v1/auth/logout", participant(s.handleLogout))
	s.Mux.HandleFunc("GET /api/v1/sync", paid(s.handleSync))
	s.Mux.HandleFunc("GET /api/v1/circles", paid(s.handleListCircles))
	s.Mux.HandleFunc("POST /api/v1/circles", paid(s.handleCreateCircle))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/invites", paid(s.handleCreateCircleInvite))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/invites", paid(s.handleListCircleInvites))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/invites/{id}", paid(s.handleRevokeCircleInvite))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/invite-candidates", paid(s.handleListInviteCandidates))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/member-invites", paid(s.handleCreateMemberInvite))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/join-preview", paid(s.handleJoinPreview))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/join", paid(s.handleJoinPendingCircle))
	s.Mux.HandleFunc("GET /api/v1/pending-circle-joins", paid(s.handleListPendingCircleJoins))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/leave", paid(s.handleLeaveCircle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/read_cursor", paid(s.handleSetReadCursor))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/responses", paid(s.handleResponses))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/responses/read", paid(s.handleSetResponsesRead))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/feed", paid(s.handleFeed))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/grid", paid(s.handleGrid))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/map", paid(s.handleMap))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/days", paid(s.handleDays))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/days/{date}", paid(s.handleDayDetail))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/search/authors", paid(s.handleCircleSearchAuthors))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/search", paid(s.handleCircleSearch))
	s.Mux.HandleFunc("GET /api/v1/search", paid(s.handleGlobalSearch))

	// Загрузка — под participant, не paid: истёкшему участнику нужен блоб
	// для скриншота оплаты, иначе включение подписки запирало всех до ручного
	// продления (аудит 2026-09-22). Владение и квоту проверяет blob.Store,
	// привязать блоб к кругу без оплаты всё равно нельзя.
	s.Mux.HandleFunc("POST /api/v1/uploads", participant(s.handleCreateUpload))
	s.Mux.HandleFunc("HEAD /api/v1/uploads/{session_id}", participant(s.handleUploadStatus))
	s.Mux.HandleFunc("PUT /api/v1/uploads/{session_id}", s.RequireParticipantStream(s.handleUploadChunk))
	s.Mux.HandleFunc("POST /api/v1/uploads/{session_id}/complete", participant(s.handleCompleteUpload))
	s.Mux.HandleFunc("GET /api/v1/blobs/{blob_id}", paid(s.handleServeBlob))

	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/posts", paid(s.handleCreatePost))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}/posts/{post_id}", paid(s.handleEditPost))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}", paid(s.handleDeletePost))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/posts/{post_id}/comments", paid(s.handleCreateComment))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}/posts/{post_id}/comments/{comment_id}", paid(s.handleEditComment))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}/comments/{comment_id}", paid(s.handleDeleteComment))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/posts/{post_id}/reactions", paid(s.handleSetReaction))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/posts/{post_id}/reactions", paid(s.handleDeleteReaction))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/days/{date}/title", paid(s.handleSetDayTitle))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/days/{date}/title", paid(s.handleClearDayTitle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/days/{date}/cover", paid(s.handleSetDayCover))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}/days/{date}/cover", paid(s.handleClearDayCover))

	s.Mux.HandleFunc("GET /api/v1/notify_prefs", paid(s.handleGetAccountNotifyPrefs))
	s.Mux.HandleFunc("PUT /api/v1/notify_prefs", paid(s.handleSetAccountNotifyPrefs))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/notify_prefs", paid(s.handleGetCircleNotifyPrefs))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/notify_prefs", paid(s.handleSetCircleNotifyPrefs))
	s.Mux.HandleFunc("POST /api/v1/push/subscribe", paid(s.handlePushSubscribe))
	s.Mux.HandleFunc("DELETE /api/v1/push/subscribe", paid(s.handlePushUnsubscribe))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/quota_requests", paid(s.handleCreateQuotaRequest))
	s.Mux.HandleFunc("GET /api/v1/pay/status", participant(s.handlePayStatus))
	s.Mux.HandleFunc("POST /api/v1/pay/requests", participant(s.handleCreatePayRequest))
	s.Mux.HandleFunc("POST /api/v1/pay/banner/dismiss", participant(s.handleDismissPayBanner))

	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}", paid(s.handleCircleDetail))
	s.Mux.HandleFunc("PATCH /api/v1/circles/{circle_id}", paid(s.handlePatchCircle))
	s.Mux.HandleFunc("DELETE /api/v1/circles/{circle_id}", paid(s.handleDeleteCircle))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/members", paid(s.handleCircleMembers))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/identity", paid(s.handleUpdateIdentity))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/place", paid(s.handleSetSharePlace))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/members/{account_id}", paid(s.handleSetMember))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/identity", paid(s.handleIdentityHistory))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/transfer", paid(s.handleTransferOwnership))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/exclude", paid(s.handleExcludeMember))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/quota", paid(s.handleCircleQuota))
	s.Mux.HandleFunc("POST /api/v1/circles/{circle_id}/archive", paid(s.handleStartArchiveCycle))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/archive/cutoff", paid(s.handleMoveCutoff))
	s.Mux.HandleFunc("PUT /api/v1/circles/{circle_id}/archive/deadline", paid(s.handleMoveDeadline))
	s.Mux.HandleFunc("GET /api/v1/circles/{circle_id}/archive/download", paid(s.handleArchiveDownload))
}

// ServeHTTP sets nosniff on every response and dispatches to Mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.Mux.ServeHTTP(w, r)
}

// WaitNotify blocks until in-flight notifyCircle goroutines finish or ctx expires.
func (s *Server) WaitNotify(ctx context.Context) {
	if s == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.notifyWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Print("shutdown: pending push notifications abandoned")
	}
}

// PublicURL returns the instance public URL. Он меняется из обработчика
// bootstrap и панели доступа, поэтому читается только через геттер (QLT-4).
func (s *Server) PublicURL() string {
	s.publicURLMu.RLock()
	defer s.publicURLMu.RUnlock()
	return s.publicURL
}

// Loopback reports whether the public URL points at this machine.
func (s *Server) Loopback() bool {
	return s.loopback.Load()
}

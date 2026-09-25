package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/api"
	"gitea.mixdep.ru/mix/wynd/internal/auth"
	"gitea.mixdep.ru/mix/wynd/internal/blob"
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"gitea.mixdep.ru/mix/wynd/internal/config"
	"gitea.mixdep.ru/mix/wynd/internal/jobs"
	"gitea.mixdep.ru/mix/wynd/internal/mail"
	"gitea.mixdep.ru/mix/wynd/internal/push"
	"gitea.mixdep.ru/mix/wynd/internal/store"
	"gitea.mixdep.ru/mix/wynd/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "backup" {
		runBackup(os.Args[2:])
		return
	}
	runServer()
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	token, err := config.BootstrapToken(cfg.DataDir)
	if err != nil {
		log.Fatalf("bootstrap token: %v", err)
	}

	st, err := store.Open(filepath.Join(cfg.DataDir, "wynd.db"))
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("store close: %v", err)
		}
	}()

	version, err := st.Version(context.Background())
	if err != nil {
		log.Fatalf("store version: %v", err)
	}
	log.Printf("schema version: %d", version)

	ch, err := chronicle.New(st)
	if err != nil {
		log.Fatalf("chronicle: %v", err)
	}

	blobsDir := filepath.Join(cfg.DataDir, "blobs")
	blobStore, err := blob.New(st, blobsDir)
	if err != nil {
		log.Fatalf("blob: %v", err)
	}

	loopback := config.IsLoopback(cfg.PublicURL)
	codeLog := auth.LogCodes{Logger: log.Default()}
	if loopback {
		codeLog.File = filepath.Join(cfg.DataDir, "dev-auth-codes.log")
	}
	mailSvc, err := mail.New(st, loopback, codeLog)
	if err != nil {
		log.Fatalf("mail: %v", err)
	}
	pushSvc, err := push.New(st)
	if err != nil {
		log.Fatalf("push: %v", err)
	}
	if err := pushSvc.EnsureKeys(context.Background()); err != nil {
		log.Fatalf("vapid: %v", err)
	}

	authSvc, err := auth.New(st, ch, mailSvc, loopback)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}
	inst, err := authSvc.Instance(context.Background())
	if err != nil {
		log.Fatalf("instance: %v", err)
	}

	maybeRunRoutine(authSvc.DB(), blobsDir, ch, blobStore, mailSvc, cfg.PublicURL)

	maintDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-maintDone:
				return
			case <-ticker.C:
				maybeRunRoutine(authSvc.DB(), blobsDir, ch, blobStore, mailSvc, cfg.PublicURL)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", readyHandler(st))
	apiSrv := api.NewServer(authSvc, ch, blobStore, mailSvc, pushSvc, token, cfg.DataDir, cfg.PublicURL, cfg.Listen, loopback)
	trusted, err := api.ParseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}
	apiSrv.TrustedProxies = trusted
	mux.Handle("/api/", apiSrv)

	buildFS, err := fs.Sub(web.Build, "dist")
	if err != nil {
		log.Fatalf("embed web dist: %v", err)
	}
	mux.Handle("/", web.SPA(buildFS))

	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: mux,
		// Header and idle timeouts stop slow-loris clients from pinning
		// connections. No ReadTimeout: it would cut long uploads and SSE.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("data dir: %s", cfg.DataDir)
	log.Printf("listening on %s", cfg.Listen)
	if !inst.Bootstrapped {
		log.Printf("bootstrap URL: %s/admin/bootstrap?token=%s", strings.TrimRight(cfg.PublicURL, "/"), token)
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	close(maintDone)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Print("shutting down")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

func maybeRunRoutine(db *sql.DB, blobsDir string, ch *chronicle.Chronicle, blobs *blob.Store, mailSvc *mail.Service, publicURL string) {
	ctx := context.Background()
	now := time.Now().UTC()
	maybeRunDailyRoutine(ctx, db, blobsDir, now)
	runArchiveJobs(ctx, db, ch, blobs, mailSvc, publicURL, now)
}

func maybeRunDailyRoutine(ctx context.Context, db *sql.DB, blobsDir string, now time.Time) {
	var last sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT last_routine_at FROM instance_settings WHERE id = 1`).Scan(&last); err != nil {
		log.Printf("daily routine: read last_routine_at: %v", err)
		return
	}
	if last.Valid && last.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, last.String); err == nil {
			if now.Sub(t) < 20*time.Hour {
				return
			}
		}
	}
	counts, err := jobs.RunDailyRoutine(ctx, db, blobsDir, now)
	if err != nil {
		log.Printf("daily routine: %v", err)
		return
	}
	log.Printf("daily routine: %+v", counts)
}

func runArchiveJobs(ctx context.Context, db *sql.DB, ch *chronicle.Chronicle, blobs *blob.Store, mailSvc *mail.Service, publicURL string, now time.Time) {
	archiveCounts, err := jobs.RunArchiveJobs(ctx, db, ch, blobs, mailSvc, publicURL, now)
	if err != nil {
		log.Printf("archive jobs: %v", err)
		return
	}
	if archiveCounts.PurgedCircles > 0 || archiveCounts.RemindersSent > 0 {
		log.Printf("archive jobs: %+v", archiveCounts)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func readyHandler(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := st.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}

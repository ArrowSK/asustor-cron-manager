package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ArrowSK/asustor-cron-manager/internal/auth"
	cronpkg "github.com/ArrowSK/asustor-cron-manager/internal/cron"
	"github.com/ArrowSK/asustor-cron-manager/internal/history"
	updatepkg "github.com/ArrowSK/asustor-cron-manager/internal/update"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	Cron    *cronpkg.Store
	Auth    *auth.Manager
	History *history.Store
	Updater *updatepkg.Manager
	Version string
	Logger  *log.Logger

	runMu   sync.Mutex
	running map[string]bool
}

func New(c *cronpkg.Store, a *auth.Manager, h *history.Store, u *updatepkg.Manager, version string, logger *log.Logger) *Server {
	return &Server{Cron: c, Auth: a, History: h, Updater: u, Version: version, Logger: logger, running: map[string]bool{}}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/setup", s.handleSetup)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/jobs", s.authenticated(s.handleJobs))
	mux.HandleFunc("/api/jobs/save", s.authenticated(s.handleSaveJob))
	mux.HandleFunc("/api/jobs/toggle", s.authenticated(s.handleToggleJob))
	mux.HandleFunc("/api/jobs/delete", s.authenticated(s.handleDeleteJob))
	mux.HandleFunc("/api/jobs/run", s.authenticated(s.handleRunJob))
	mux.HandleFunc("/api/history", s.authenticated(s.handleHistory))
	mux.HandleFunc("/api/backups", s.authenticated(s.handleBackups))
	mux.HandleFunc("/api/backups/restore", s.authenticated(s.handleRestore))
	mux.HandleFunc("/api/export", s.authenticated(s.handleExport))
	mux.HandleFunc("/api/export/crontab", s.authenticated(s.handleExportCrontab))
	mux.HandleFunc("/api/import", s.authenticated(s.handleImport))
	mux.HandleFunc("/api/update/status", s.authenticated(s.handleUpdateStatus))
	mux.HandleFunc("/api/update/apply", s.authenticated(s.handleUpdateApply))
	mux.HandleFunc("/api/settings/password", s.authenticated(s.handlePassword))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/", s.serveWeb)
	return s.lanOnly(s.securityHeaders(mux))
}

func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if strings.Contains(path, "..") {
		http.NotFound(w, r)
		return
	}
	b, err := webFS.ReadFile("web/" + path)
	if err != nil {
		b, err = webFS.ReadFile("web/index.html")
	}
	if err != nil {
		http.Error(w, "web assets unavailable", 500)
		return
	}
	switch {
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) lanOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			http.Error(w, "Cron Manager is available only from the local network", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("acm_session")
		if err != nil || !s.Auth.ValidSession(cookie.Value) {
			jsonError(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !sameOrigin(r) {
				jsonError(w, "origin check failed", http.StatusForbidden)
				return
			}
			if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				jsonError(w, "application/json required", http.StatusUnsupportedMediaType)
				return
			}
		}
		next(w, r)
	}
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	authed := false
	if c, err := r.Cookie("acm_session"); err == nil {
		authed = s.Auth.ValidSession(c.Value)
	}
	jsonWrite(w, map[string]any{"setupRequired": s.Auth.SetupRequired(), "authenticated": authed, "version": s.Version})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !sameOrigin(r) {
		jsonError(w, "origin check failed", 403)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if err := s.Auth.Setup(req.Password); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	s.issueSession(w)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !sameOrigin(r) {
		jsonError(w, "origin check failed", 403)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if !s.Auth.Verify(req.Password) {
		time.Sleep(350 * time.Millisecond)
		jsonError(w, "invalid password", 401)
		return
	}
	s.issueSession(w)
}

func (s *Server) issueSession(w http.ResponseWriter) {
	token, exp, err := s.Auth.NewSession()
	if err != nil {
		jsonError(w, "could not create session", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "acm_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: exp, MaxAge: int(time.Until(exp).Seconds())})
	jsonWrite(w, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if c, err := r.Cookie("acm_session"); err == nil {
		s.Auth.Revoke(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "acm_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	jsonWrite(w, map[string]any{"ok": true})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	jobs, err := s.Cron.ListJobs()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	for i := range jobs {
		if rec, _ := s.History.Latest(jobs[i].ID); rec != nil {
			jobs[i].LastRun = &cronpkg.RunSummary{Started: rec.Started, Finished: rec.Finished, ExitCode: rec.ExitCode, Source: rec.Source}
		}
		s.runMu.Lock()
		jobs[i].Running = s.running[jobs[i].ID]
		s.runMu.Unlock()
	}
	jsonWrite(w, map[string]any{"jobs": jobs})
}

func (s *Server) handleSaveJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID, Name, Schedule, Command string
		Enabled                     bool
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	id, err := s.Cron.SaveJob(req.ID, req.Name, req.Schedule, req.Command, req.Enabled)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true, "id": id})
}

func (s *Server) handleToggleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if err := s.Cron.ToggleJob(req.ID, req.Enabled); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true})
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if err := s.Cron.DeleteJob(req.ID); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true})
}

func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	job, err := s.Cron.ResolveJob(req.ID)
	if err != nil {
		jsonError(w, err.Error(), 404)
		return
	}
	s.runMu.Lock()
	if s.running[job.ID] {
		s.runMu.Unlock()
		jsonError(w, "job is already running", 409)
		return
	}
	s.running[job.ID] = true
	s.runMu.Unlock()
	runID := cronpkg.NewID()
	go func() {
		defer func() { s.runMu.Lock(); delete(s.running, job.ID); s.runMu.Unlock() }()
		rec := Execute(job.ID, job.Command, "manual", runID, 15*time.Minute)
		if err := s.History.Add(rec); err != nil {
			s.Logger.Printf("history: %v", err)
		}
	}()
	jsonWrite(w, map[string]any{"ok": true, "runId": runID})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		jsonError(w, "id is required", 400)
		return
	}
	rows, err := s.History.List(id)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonWrite(w, map[string]any{"history": rows})
}

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.Cron.Backups()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonWrite(w, map[string]any{"backups": rows})
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if err := s.Cron.Restore(req.Name); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	bundle, err := s.Cron.ExportManaged(s.Version)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="CronManager_export_%s.json"`, time.Now().UTC().Format("20060102T150405Z")))
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(bundle)
}

func (s *Server) handleExportCrontab(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	content, err := s.Cron.Read()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="root-crontab_%s.txt"`, time.Now().UTC().Format("20060102T150405Z")))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, content)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Bundle cronpkg.ExportBundle `json:"bundle"`
		Mode   string               `json:"mode"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	result, err := s.Cron.ImportManaged(req.Bundle, req.Mode)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true, "result": result})
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if s.Updater == nil {
		jsonError(w, "updater is unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	status, err := s.Updater.Check(ctx)
	if err != nil {
		jsonError(w, err.Error(), 502)
		return
	}
	jsonWrite(w, status)
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if s.Updater == nil {
		jsonError(w, "updater is unavailable", 503)
		return
	}
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if !req.Confirm {
		jsonError(w, "explicit confirmation is required", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	status, err := s.Updater.Apply(ctx)
	if err != nil {
		jsonError(w, err.Error(), 502)
		return
	}
	jsonWrite(w, map[string]any{"ok": true, "status": status, "restarting": true})
	s.Updater.RestartIntoUpdatedBinary(2*time.Second, s.Logger.Printf)
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	if err := s.Auth.Change(req.Current, req.New); err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	jsonWrite(w, map[string]any{"ok": true, "reauthenticate": true})
}

func Execute(jobID, command, source, runID string, timeout time.Duration) history.Record {
	started := time.Now().UTC()
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	buf := &limitBuffer{limit: 64 * 1024}
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			exitCode = 124
		} else {
			exitCode = 255
		}
	}
	return history.Record{RunID: runID, JobID: jobID, Started: started, Finished: time.Now().UTC(), ExitCode: exitCode, Source: source, Output: buf.String()}
}

type limitBuffer struct {
	b     strings.Builder
	limit int
}

func (l *limitBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remain := l.limit - l.b.Len()
	if remain > 0 {
		if len(p) > remain {
			p = p[:remain]
		}
		_, _ = l.b.Write(p)
	}
	return n, nil
}
func (l *limitBuffer) String() string {
	if l.b.Len() >= l.limit {
		return l.b.String() + "\n[output truncated]"
	}
	return l.b.String()
}

func decodeJSON(r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return fmt.Errorf("application/json required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func jsonWrite(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
func methodNotAllowed(w http.ResponseWriter) {
	jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
}

func ListenAndServe(addr string, h http.Handler, logger *log.Logger) error {
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Minute, IdleTimeout: 60 * time.Second, ErrorLog: logger}
	logger.Printf("Cron Manager listening on %s", addr)
	return srv.ListenAndServe()
}

func FormatDuration(d time.Duration) string { return strconv.FormatInt(int64(d.Seconds()), 10) }

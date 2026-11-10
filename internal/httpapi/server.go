// Package httpapi exposes the usecases over JSON and serves the web app.
// Handlers only parse the request, call a usecase and write the response.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/finance"
	"arrange-matches-bwd/internal/session"
)

type Config struct {
	Sessions *session.Service
	Finance  *finance.Service
	// Web is the built frontend; nil serves the API only.
	Web fs.FS

	AdminUsername string
	AdminPassword string
	// Secret signs login cookies.
	Secret []byte
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

type server struct {
	sessions *session.Service
	finance  *finance.Service
	auth     *auth
	now      func() time.Time
}

// New builds the HTTP handler: /api/* behind the admin login, everything
// else from the embedded web app.
func New(cfg Config) http.Handler {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	s := &server{
		sessions: cfg.Sessions,
		finance:  cfg.Finance,
		auth:     &auth{username: cfg.AdminUsername, password: cfg.AdminPassword, secret: cfg.Secret, now: now},
		now:      now,
	}

	api := http.NewServeMux()
	route := func(pattern string, h func(r *http.Request) (any, error)) {
		api.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			out, err := h(r)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		})
	}

	route("GET /api/settings", s.getSettings)
	route("PUT /api/settings", s.putSettings)

	route("GET /api/players", s.listPlayers)
	route("POST /api/players", s.createPlayer)
	route("PUT /api/players/{id}", s.updatePlayer)
	route("DELETE /api/players/{id}", s.deletePlayer)

	route("GET /api/session/current", s.currentSession)
	route("POST /api/sessions", s.createSession)
	route("GET /api/sessions/{id}", s.getSession)
	route("POST /api/sessions/{id}/end", s.endSession)
	route("PUT /api/sessions/{id}/courts", s.setCourtCount)
	route("POST /api/sessions/{id}/players", s.checkIn)
	route("DELETE /api/session-players/{id}", s.removeSessionPlayer)
	route("PUT /api/session-players/{id}/status", s.setPlayerStatus)
	route("PUT /api/session-players/{id}/payment", s.setPayment)

	route("POST /api/courts/{id}/propose", s.propose)
	route("PUT /api/matches/{id}/swap", s.swap)
	route("POST /api/matches/{id}/confirm", s.confirm)
	route("DELETE /api/matches/{id}", s.dismiss)
	route("POST /api/matches/{id}/finish", s.finish)

	route("GET /api/transactions", s.listTransactions)
	route("POST /api/transactions", s.createTransaction)
	route("PUT /api/transactions/{id}", s.updateTransaction)
	route("DELETE /api/transactions/{id}", s.deleteTransaction)

	route("GET /api/reports/summary", s.reportSummary)
	route("GET /api/reports/months/{month}", s.reportMonth)
	route("GET /api/reports/sessions/{id}", s.reportSession)

	api.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, domain.ErrNotFound)
	})

	root := http.NewServeMux()
	root.HandleFunc("POST /api/login", s.login)
	root.HandleFunc("POST /api/logout", s.logout)
	root.HandleFunc("GET /api/me", s.me)
	root.Handle("/api/", s.requireLogin(api))
	if cfg.Web != nil {
		root.Handle("/", webHandler(cfg.Web))
	}
	return root
}

func (s *server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.authenticated(r) {
			writeJSON(w, http.StatusUnauthorized, errorBody("unauthorized", "Bạn chưa đăng nhập."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if s.auth.throttled() {
		writeJSON(w, http.StatusTooManyRequests, errorBody("too_many_attempts", "Đăng nhập sai quá nhiều lần. Thử lại sau một phút."))
		return
	}
	if !s.auth.checkCredentials(req.Username, req.Password) {
		s.auth.recordFailure()
		writeJSON(w, http.StatusUnauthorized, errorBody("bad_credentials", "Sai tên đăng nhập hoặc mật khẩu."))
		return
	}
	s.auth.setCookie(w, r, s.auth.newToken(), int(loginTTL.Seconds()))
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.setCookie(w, r, "", -1)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": s.auth.authenticated(r)})
}

// webHandler serves the built single-page app; unknown paths fall back to
// index.html so client-side navigation survives a reload.
func webHandler(web fs.FS) http.Handler {
	files := http.FileServerFS(web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(web, name); err != nil {
			index, err := fs.ReadFile(web, "index.html")
			if err != nil {
				http.Error(w, "Chưa có bản build giao diện. Chạy `npm run build` trong thư mục web.", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(index)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			// Vite fingerprints these file names.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func init() {
	// Not in every system's MIME table; needed for the PWA manifest.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func errorBody(code, message string) errorResponse {
	var e errorResponse
	e.Error.Code, e.Error.Message = code, message
	return e
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var de *domain.Error
	if !errors.As(err, &de) {
		log.Printf("httpapi: internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorBody("internal", "Lỗi hệ thống. Vui lòng thử lại."))
		return
	}
	status := http.StatusBadRequest
	switch de.Kind {
	case domain.KindNotFound:
		status = http.StatusNotFound
	case domain.KindConflict:
		status = http.StatusConflict
	}
	writeJSON(w, status, errorBody(de.Code, de.Message))
}

// decode reads a JSON body. Requiring the JSON content type keeps plain
// HTML forms on other sites from posting to the API.
func decode(r *http.Request, v any) error {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		return domain.ErrInvalidInput
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return domain.ErrInvalidInput
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

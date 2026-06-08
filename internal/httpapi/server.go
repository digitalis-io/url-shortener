package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/digitalis-io/url-shortner/internal/auth"
	"github.com/digitalis-io/url-shortner/internal/config"
	"github.com/digitalis-io/url-shortner/internal/shorturl"
	"github.com/digitalis-io/url-shortner/web"
)

type Readiness interface {
	Ready(context.Context) error
}

type Server struct {
	cfg     config.Config
	auth    *auth.Auth
	service *shorturl.Service
	ready   Readiness
	logger  *slog.Logger
}

var requestCount = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "url_shortener_http_requests_total",
		Help: "Total HTTP requests.",
	},
	[]string{"method", "path", "status"},
)

func New(cfg config.Config, authn *auth.Auth, service *shorturl.Service, ready Readiness, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, auth: authn, service: service, ready: ready, logger: logger}
}

func (s *Server) Handler() http.Handler {
	admin := s.adminRouter()
	public := s.publicRouter()
	combined := s.combinedRouter()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			s.health(w, r)
			return
		case "/readyz":
			s.readyz(w, r)
			return
		}
		host := normalizeHost(r.Host)
		if s.cfg.AdminHost == "" || s.cfg.PublicHost == "" || s.cfg.AdminHost == s.cfg.PublicHost {
			combined.ServeHTTP(w, r)
			return
		}
		switch host {
		case normalizeHost(s.cfg.AdminHost):
			admin.ServeHTTP(w, r)
		case normalizeHost(s.cfg.PublicHost):
			public.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Server) combinedRouter() http.Handler {
	r := s.baseRouter()
	s.mountAdminRoutes(r)
	r.Get("/{code}", s.redirect)
	return r
}

func (s *Server) baseRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.metrics)
	r.Get("/healthz", s.health)
	r.Get("/readyz", s.readyz)
	r.Handle("/metrics", promhttp.Handler())
	return r
}

func (s *Server) adminRouter() http.Handler {
	r := s.baseRouter()
	s.mountAdminRoutes(r)
	return r
}

func (s *Server) mountAdminRoutes(r chi.Router) {
	staticFS, _ := fs.Sub(web.Static, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	r.Get("/login", s.auth.Login)
	r.Get("/logout", s.auth.Logout)
	r.Get("/saml/metadata", s.auth.Metadata)
	r.Post("/saml/acs", s.auth.ACS)

	r.Group(func(admin chi.Router) {
		admin.Use(s.auth.Require)
		admin.Get("/", s.index)
		admin.Route("/api/v1/urls", func(api chi.Router) {
			api.Get("/", s.listURLs)
			api.With(s.auth.RequireCSRF).Post("/", s.createURL)
			api.Get("/{code}", s.getURL)
			api.With(s.auth.RequireCSRF).Delete("/{code}", s.deleteURL)
			api.Get("/{code}/hits", s.getHits)
		})
	})
}

func (s *Server) publicRouter() http.Handler {
	r := s.baseRouter()
	r.Get("/{code}", s.redirect)
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.ready.Ready(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, web.Static, "static/index.html")
}

type createURLRequest struct {
	URL       string     `json:"url"`
	Alias     string     `json:"alias"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type urlResponse struct {
	Code      string        `json:"code"`
	ShortURL  string        `json:"short_url"`
	URL       string        `json:"url"`
	CreatedBy shorturl.User `json:"created_by"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
}

func (s *Server) createURL(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	var req createURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	result, err := s.service.Create(shorturl.CreateRequest{
		URL:       req.URL,
		Alias:     req.Alias,
		ExpiresAt: req.ExpiresAt,
		CreatedBy: user,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toURLResponse(result.Record))
}

func (s *Server) listURLs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := s.service.ListRecent(shorturl.ListOptions{
		Limit:     limit,
		PageToken: r.URL.Query().Get("page_token"),
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	items := make([]urlResponse, 0, len(result.URLs))
	for _, record := range result.URLs {
		items = append(items, toURLResponse(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"urls":            items,
		"next_page_token": result.NextPageToken,
	})
}

func (s *Server) getURL(w http.ResponseWriter, r *http.Request) {
	record, err := s.service.Get(chi.URLParam(r, "code"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toURLResponse(record))
}

func (s *Server) deleteURL(w http.ResponseWriter, r *http.Request) {
	if err := s.service.Delete(chi.URLParam(r, "code")); err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getHits(w http.ResponseWriter, r *http.Request) {
	from, err := parseOptionalTime(r.URL.Query().Get("from"))
	if err != nil {
		http.Error(w, "invalid from", http.StatusBadRequest)
		return
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"))
	if err != nil {
		http.Error(w, "invalid to", http.StatusBadRequest)
		return
	}
	record, buckets, err := s.service.HourlyHits(chi.URLParam(r, "code"), from, to)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":      record.Code,
		"short_url": record.ShortURL,
		"series":    buckets,
	})
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	record, err := s.service.Get(code)
	if err != nil {
		var expired shorturl.ErrExpired
		if errors.As(err, &expired) {
			http.Error(w, "gone", http.StatusGone)
			return
		}
		s.writeServiceError(w, err)
		return
	}

	go func() {
		if err := s.service.RecordHit(record); err != nil && s.logger != nil {
			s.logger.Warn("record hit failed", "code", record.Code, "err", err)
		}
	}()
	http.Redirect(w, r, record.OriginalURL, http.StatusFound)
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	var notFound shorturl.ErrNotFound
	var conflict shorturl.ErrConflict
	var expired shorturl.ErrExpired
	switch {
	case errors.As(err, &notFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.As(err, &conflict):
		http.Error(w, "conflict", http.StatusConflict)
	case errors.As(err, &expired):
		http.Error(w, "gone", http.StatusGone)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (s *Server) metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		requestCount.WithLabelValues(r.Method, routeLabel(r.URL.Path), strconv.Itoa(rec.status)).Inc()
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func toURLResponse(record shorturl.URLRecord) urlResponse {
	return urlResponse{
		Code:      record.Code,
		ShortURL:  record.ShortURL,
		URL:       record.OriginalURL,
		CreatedBy: record.CreatedBy,
		CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func routeLabel(path string) string {
	switch {
	case path == "/":
		return "/"
	case strings.HasPrefix(path, "/api/v1/urls"):
		return "/api/v1/urls"
	case strings.HasPrefix(path, "/static/"):
		return "/static"
	default:
		return path
	}
}

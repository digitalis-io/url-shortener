package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/digitalis-io/url-shortner/internal/auth"
	"github.com/digitalis-io/url-shortner/internal/config"
	"github.com/digitalis-io/url-shortner/internal/shorturl"
)

type fakeReady struct{}

func (fakeReady) Ready(context.Context) error { return nil }

type fakeRepo struct {
	mu      sync.Mutex
	records map[string]shorturl.URLRecord
	hits    map[string]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		records: map[string]shorturl.URLRecord{},
		hits:    map[string]int{},
	}
}

func (f *fakeRepo) CreateURL(record shorturl.URLRecord) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.records[record.Code]; ok {
		return false, nil
	}
	f.records[record.Code] = record
	return true, nil
}

func (f *fakeRepo) GetURL(code string) (shorturl.URLRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.records[code]
	if !ok {
		return shorturl.URLRecord{}, shorturl.ErrNotFound{}
	}
	return record, nil
}

func (f *fakeRepo) ListRecent(shorturl.ListOptions) (shorturl.ListResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]shorturl.URLRecord, 0, len(f.records))
	for _, record := range f.records {
		out = append(out, record)
	}
	return shorturl.ListResult{URLs: out}, nil
}

func (f *fakeRepo) SoftDelete(code string, deletedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.records[code]
	if !ok {
		return shorturl.ErrNotFound{}
	}
	record.DeletedAt = &deletedAt
	f.records[code] = record
	return nil
}

func (f *fakeRepo) IncrementHourlyHits(shortURL string, hourStart time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[shortURL+"|"+hourStart.Format(time.RFC3339)]++
	return nil
}

func (f *fakeRepo) GetHourlyHits(string, time.Time, time.Time) ([]shorturl.HitBucket, error) {
	return nil, nil
}

func TestPublicRedirectDoesNotRequireAuth(t *testing.T) {
	repo := newFakeRepo()
	repo.records["abc"] = shorturl.URLRecord{
		Code:        "abc",
		ShortURL:    "https://short.example/abc",
		OriginalURL: "https://example.com/target",
		CreatedBy:   shorturl.User{ID: "user-1"},
		CreatedAt:   time.Now().UTC(),
	}
	handler := testServer(t, repo, false).Handler()

	req := httptest.NewRequest(http.MethodGet, "https://short.example/abc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://example.com/target" {
		t.Fatalf("unexpected redirect location: %s", got)
	}
}

func TestAdminUIRequiresAuth(t *testing.T) {
	handler := testServer(t, newFakeRepo(), false).Handler()

	req := httptest.NewRequest(http.MethodGet, "https://admin.example/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("unexpected redirect location: %s", got)
	}
}

func TestCreateURLWithDevBypassAndCSRF(t *testing.T) {
	repo := newFakeRepo()
	handler := testServer(t, repo, true).Handler()

	loginReq := httptest.NewRequest(http.MethodGet, "https://admin.example/", nil)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)

	var cookies []*http.Cookie
	var csrf string
	for _, cookie := range loginRec.Result().Cookies() {
		cookies = append(cookies, cookie)
		if cookie.Name == "url_shortener_csrf" {
			csrf = cookie.Value
		}
	}
	if csrf == "" {
		t.Fatal("expected csrf cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "https://admin.example/api/v1/urls", strings.NewReader(`{"url":"https://example.com/a","alias":"abc"}`))
	req.Header.Set("X-CSRF-Token", csrf)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code      string        `json:"code"`
		ShortURL  string        `json:"short_url"`
		CreatedBy shorturl.User `json:"created_by"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ShortURL != "https://short.example/abc" {
		t.Fatalf("unexpected short URL: %s", body.ShortURL)
	}
	if body.CreatedBy.Email != "dev@example.com" {
		t.Fatalf("unexpected creator: %+v", body.CreatedBy)
	}
}

func testServer(t *testing.T, repo *fakeRepo, devBypass bool) *Server {
	t.Helper()
	cfg := config.Config{
		HTTPAddr:      ":8080",
		PublicBaseURL: "https://short.example",
		AdminBaseURL:  "https://admin.example",
		PublicHost:    "short.example",
		AdminHost:     "admin.example",
		SessionSecret: "test-secret",
		AuthDevBypass: devBypass,
		DevUserID:     "dev-user",
		DevUserEmail:  "dev@example.com",
		CodeLength:    7,
	}
	authn, err := auth.New(cfg)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	service := shorturl.NewService(repo, cfg.PublicBaseURL, cfg.CodeLength)
	return New(cfg, authn, service, fakeReady{}, nil)
}

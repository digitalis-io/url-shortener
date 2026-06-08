package cassandra

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/digitalis-io/url-shortner/internal/config"
	"github.com/digitalis-io/url-shortner/internal/shorturl"
)

func TestIntegrationURLLifecycleAndHourlyHits(t *testing.T) {
	if os.Getenv("CASSANDRA_INTEGRATION") != "1" {
		t.Skip("set CASSANDRA_INTEGRATION=1 to run Cassandra integration tests")
	}

	store, err := Connect(config.Config{
		CassandraHosts:              []string{envOr("CASSANDRA_HOSTS", "localhost:9042")},
		CassandraKeyspace:           envOr("CASSANDRA_KEYSPACE", "url_shortener_integration"),
		CassandraCreateKeyspace:     true,
		CassandraReplicationStrategy: "SimpleStrategy",
		CassandraReplicationFactor:  1,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	code := "it" + now.Format("150405000")
	shortURL := "http://localhost:18080/" + code
	record := shorturl.URLRecord{
		Code:        code,
		ShortURL:    shortURL,
		OriginalURL: "https://example.com/integration",
		URLHash:     "hash",
		CreatedBy: shorturl.User{
			ID:    "user-1",
			Email: "user@example.com",
		},
		CreatedAt: now,
	}

	applied, err := store.CreateURL(record)
	if err != nil {
		t.Fatalf("CreateURL: %v", err)
	}
	if !applied {
		t.Fatal("expected create to apply")
	}

	got, err := store.GetURL(code)
	if err != nil {
		t.Fatalf("GetURL: %v", err)
	}
	if got.OriginalURL != record.OriginalURL {
		t.Fatalf("unexpected original URL: %s", got.OriginalURL)
	}
	if got.CreatedBy.Email != record.CreatedBy.Email {
		t.Fatalf("unexpected creator: %+v", got.CreatedBy)
	}

	list, err := store.ListRecent(shorturl.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if !containsCode(list.URLs, code) {
		t.Fatalf("list did not contain created code %s", code)
	}

	hour := now.Truncate(time.Hour)
	if err := store.IncrementHourlyHits(shortURL, hour); err != nil {
		t.Fatalf("IncrementHourlyHits: %v", err)
	}
	hits, err := store.GetHourlyHits(shortURL, hour.Add(-time.Hour), hour.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetHourlyHits: %v", err)
	}
	if !containsHit(hits, hour, 1) {
		t.Fatalf("expected hourly hit at %s, got %+v", hour, hits)
	}

	deletedAt := now.Add(time.Minute)
	if err := store.SoftDelete(code, deletedAt); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	got, err = store.GetURL(code)
	if err != nil {
		t.Fatalf("GetURL after delete: %v", err)
	}
	if got.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set")
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func containsCode(records []shorturl.URLRecord, code string) bool {
	for _, record := range records {
		if record.Code == code {
			return true
		}
	}
	return false
}

func containsHit(hits []shorturl.HitBucket, hour time.Time, want int64) bool {
	for _, hit := range hits {
		if hit.HourStart.Equal(hour) && hit.Hits >= want {
			return true
		}
	}
	return false
}

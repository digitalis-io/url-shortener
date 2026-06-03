package shorturl

import (
	"testing"
	"time"
)

type fakeRepo struct {
	records map[string]URLRecord
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{records: map[string]URLRecord{}}
}

func (f *fakeRepo) CreateURL(record URLRecord) (bool, error) {
	if _, exists := f.records[record.Code]; exists {
		return false, nil
	}
	f.records[record.Code] = record
	return true, nil
}

func (f *fakeRepo) GetURL(code string) (URLRecord, error) {
	record, ok := f.records[code]
	if !ok {
		return URLRecord{}, ErrNotFound{}
	}
	return record, nil
}

func (f *fakeRepo) ListRecent(opts ListOptions) (ListResult, error) {
	out := make([]URLRecord, 0, len(f.records))
	for _, record := range f.records {
		out = append(out, record)
	}
	return ListResult{URLs: out}, nil
}

func (f *fakeRepo) SoftDelete(code string, deletedAt time.Time) error {
	record, ok := f.records[code]
	if !ok {
		return ErrNotFound{}
	}
	record.DeletedAt = &deletedAt
	f.records[code] = record
	return nil
}

func (f *fakeRepo) IncrementHourlyHits(string, time.Time) error {
	return nil
}

func (f *fakeRepo) GetHourlyHits(string, time.Time, time.Time) ([]HitBucket, error) {
	return nil, nil
}

func TestCreateStoresCreatorAndShortURL(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo, "https://short.example", 7)
	service.now = func() time.Time { return time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC) }

	result, err := service.Create(CreateRequest{
		URL:   "https://example.com/a",
		Alias: "custom",
		CreatedBy: User{
			ID:    "user-1",
			Email: "user@example.com",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Record.ShortURL != "https://short.example/custom" {
		t.Fatalf("unexpected short URL: %s", result.Record.ShortURL)
	}
	if result.Record.CreatedBy.ID != "user-1" {
		t.Fatalf("creator was not stored")
	}
}

func TestCreateAliasConflict(t *testing.T) {
	repo := newFakeRepo()
	service := NewService(repo, "https://short.example", 7)
	req := CreateRequest{URL: "https://example.com/a", Alias: "custom", CreatedBy: User{ID: "u"}}
	if _, err := service.Create(req); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if _, err := service.Create(req); err == nil {
		t.Fatal("expected conflict")
	}
}

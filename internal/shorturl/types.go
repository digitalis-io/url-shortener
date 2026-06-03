package shorturl

import "time"

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type URLRecord struct {
	Code        string
	ShortURL    string
	OriginalURL string
	URLHash     string
	CreatedBy   User
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	DeletedAt   *time.Time
	CustomAlias bool
}

type CreateRequest struct {
	URL       string
	Alias     string
	ExpiresAt *time.Time
	CreatedBy User
}

type CreateResult struct {
	Record URLRecord
}

type ListOptions struct {
	Limit     int
	PageToken string
}

type ListResult struct {
	URLs          []URLRecord
	NextPageToken string
}

type HitBucket struct {
	HourStart time.Time `json:"hour_start"`
	Hits      int64     `json:"hits"`
}

type Repository interface {
	CreateURL(record URLRecord) (bool, error)
	GetURL(code string) (URLRecord, error)
	ListRecent(opts ListOptions) (ListResult, error)
	SoftDelete(code string, deletedAt time.Time) error
	IncrementHourlyHits(shortURL string, hourStart time.Time) error
	GetHourlyHits(shortURL string, from, to time.Time) ([]HitBucket, error)
}

type ErrNotFound struct{}

func (ErrNotFound) Error() string {
	return "short URL not found"
}

type ErrConflict struct{}

func (ErrConflict) Error() string {
	return "short code already exists"
}

type ErrExpired struct{}

func (ErrExpired) Error() string {
	return "short URL has expired"
}

package shorturl

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"
)

type Service struct {
	repo          Repository
	publicBaseURL string
	codeLength    int
	now           func() time.Time
}

func NewService(repo Repository, publicBaseURL string, codeLength int) *Service {
	return &Service{
		repo:          repo,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		codeLength:    codeLength,
		now:           time.Now,
	}
}

func (s *Service) Create(req CreateRequest) (CreateResult, error) {
	now := s.now().UTC()
	normalized, err := ValidateAndNormalizeURL(req.URL)
	if err != nil {
		return CreateResult{}, err
	}
	if err := ValidateAlias(req.Alias); err != nil {
		return CreateResult{}, err
	}
	if err := ValidateExpiration(req.ExpiresAt, now); err != nil {
		return CreateResult{}, err
	}
	if req.CreatedBy.ID == "" {
		return CreateResult{}, errors.New("creator identity is required")
	}

	code := req.Alias
	customAlias := code != ""
	maxAttempts := 1
	if !customAlias {
		maxAttempts = 5
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if !customAlias {
			code, err = GenerateCode(s.codeLength)
			if err != nil {
				return CreateResult{}, err
			}
		}

		record := URLRecord{
			Code:        code,
			ShortURL:    s.ShortURL(code),
			OriginalURL: normalized,
			URLHash:     hashURL(normalized),
			CreatedBy:   req.CreatedBy,
			CreatedAt:   now,
			ExpiresAt:   req.ExpiresAt,
			CustomAlias: customAlias,
		}

		applied, err := s.repo.CreateURL(record)
		if err != nil {
			return CreateResult{}, err
		}
		if applied {
			return CreateResult{Record: record}, nil
		}
		lastErr = ErrConflict{}
		if customAlias {
			break
		}
	}

	if lastErr == nil {
		lastErr = ErrConflict{}
	}
	return CreateResult{}, lastErr
}

func (s *Service) Get(code string) (URLRecord, error) {
	record, err := s.repo.GetURL(code)
	if err != nil {
		return URLRecord{}, err
	}
	if record.DeletedAt != nil {
		return URLRecord{}, ErrNotFound{}
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(s.now().UTC()) {
		return URLRecord{}, ErrExpired{}
	}
	record.ShortURL = s.ShortURL(record.Code)
	return record, nil
}

func (s *Service) ListRecent(opts ListOptions) (ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 50
	}
	result, err := s.repo.ListRecent(opts)
	if err != nil {
		return ListResult{}, err
	}
	for i := range result.URLs {
		result.URLs[i].ShortURL = s.ShortURL(result.URLs[i].Code)
	}
	return result, nil
}

func (s *Service) Delete(code string) error {
	if _, err := s.Get(code); err != nil {
		return err
	}
	return s.repo.SoftDelete(code, s.now().UTC())
}

func (s *Service) RecordHit(record URLRecord) error {
	hour := s.now().UTC().Truncate(time.Hour)
	return s.repo.IncrementHourlyHits(record.ShortURL, hour)
}

func (s *Service) HourlyHits(code string, from, to time.Time) (URLRecord, []HitBucket, error) {
	record, err := s.Get(code)
	if err != nil {
		return URLRecord{}, nil, err
	}

	now := s.now().UTC()
	if from.IsZero() && to.IsZero() {
		to = now.Truncate(time.Hour).Add(time.Hour)
		from = to.Add(-24 * time.Hour)
	}
	from = from.UTC().Truncate(time.Hour)
	to = to.UTC().Truncate(time.Hour)
	if !to.After(from) {
		return URLRecord{}, nil, errors.New("to must be after from")
	}
	if to.Sub(from) > 30*24*time.Hour {
		return URLRecord{}, nil, errors.New("range cannot exceed 30 days")
	}

	buckets, err := s.repo.GetHourlyHits(record.ShortURL, from, to)
	if err != nil {
		return URLRecord{}, nil, err
	}
	return record, fillZeroBuckets(from, to, buckets), nil
}

func (s *Service) ShortURL(code string) string {
	return s.publicBaseURL + "/" + url.PathEscape(code)
}

func hashURL(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func fillZeroBuckets(from, to time.Time, existing []HitBucket) []HitBucket {
	byHour := make(map[time.Time]int64, len(existing))
	for _, bucket := range existing {
		byHour[bucket.HourStart.UTC().Truncate(time.Hour)] = bucket.Hits
	}

	out := make([]HitBucket, 0, int(to.Sub(from).Hours()))
	for cursor := from; cursor.Before(to); cursor = cursor.Add(time.Hour) {
		out = append(out, HitBucket{
			HourStart: cursor,
			Hits:      byHour[cursor],
		})
	}
	return out
}

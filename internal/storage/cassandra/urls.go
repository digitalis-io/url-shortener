package cassandra

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"

	"github.com/digitalis-io/url-shortner/internal/shorturl"
)

func (s *Store) CreateURL(record shorturl.URLRecord) (bool, error) {
	code := normalizeCode(record.Code)
	applied, err := s.session.Query(
		`INSERT INTO urls_by_code (
			code, original_url, url_hash, created_by_id, created_by_email,
			created_at, expires_at, deleted_at, custom_alias
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
		code,
		record.OriginalURL,
		record.URLHash,
		record.CreatedBy.ID,
		record.CreatedBy.Email,
		record.CreatedAt,
		record.ExpiresAt,
		record.DeletedAt,
		record.CustomAlias,
	).ScanCAS()
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}

	err = s.session.Query(
		`INSERT INTO urls_by_created_day (
			day, created_at, code, original_url, created_by_id, created_by_email,
			expires_at, deleted_at, custom_alias
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dayKey(record.CreatedAt),
		record.CreatedAt,
		code,
		record.OriginalURL,
		record.CreatedBy.ID,
		record.CreatedBy.Email,
		record.ExpiresAt,
		record.DeletedAt,
		record.CustomAlias,
	).Exec()
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Store) GetURL(code string) (shorturl.URLRecord, error) {
	var record shorturl.URLRecord
	var expiresAt, deletedAt *time.Time
	err := s.session.Query(
		`SELECT code, original_url, url_hash, created_by_id, created_by_email,
			created_at, expires_at, deleted_at, custom_alias
		 FROM urls_by_code WHERE code = ?`,
		normalizeCode(code),
	).Scan(
		&record.Code,
		&record.OriginalURL,
		&record.URLHash,
		&record.CreatedBy.ID,
		&record.CreatedBy.Email,
		&record.CreatedAt,
		&expiresAt,
		&deletedAt,
		&record.CustomAlias,
	)
	if errors.Is(err, gocql.ErrNotFound) {
		return shorturl.URLRecord{}, shorturl.ErrNotFound{}
	}
	if err != nil {
		return shorturl.URLRecord{}, err
	}
	record.ExpiresAt = expiresAt
	record.DeletedAt = deletedAt
	return record, nil
}

func (s *Store) ListRecent(opts shorturl.ListOptions) (shorturl.ListResult, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursor := time.Now().UTC().Add(time.Hour)
	if opts.PageToken != "" {
		parsed, err := decodePageToken(opts.PageToken)
		if err != nil {
			return shorturl.ListResult{}, err
		}
		cursor = parsed
	}

	records := make([]shorturl.URLRecord, 0, limit)
	for dayCursor := cursor; len(records) < limit; dayCursor = beginningOfPreviousDay(dayCursor) {
		if time.Since(dayCursor) > 370*24*time.Hour {
			break
		}

		remaining := limit - len(records)
		iter := s.session.Query(
			`SELECT code, original_url, created_by_id, created_by_email,
				created_at, expires_at, deleted_at, custom_alias
			 FROM urls_by_created_day
			 WHERE day = ? AND created_at < ?
			 LIMIT ?`,
			dayKey(dayCursor),
			dayCursor,
			remaining,
		).Iter()

		var (
			record               shorturl.URLRecord
			expiresAt, deletedAt *time.Time
		)
		for iter.Scan(
			&record.Code,
			&record.OriginalURL,
			&record.CreatedBy.ID,
			&record.CreatedBy.Email,
			&record.CreatedAt,
			&expiresAt,
			&deletedAt,
			&record.CustomAlias,
		) {
			record.ExpiresAt = expiresAt
			record.DeletedAt = deletedAt
			if record.DeletedAt == nil {
				records = append(records, record)
			}
			record = shorturl.URLRecord{}
			expiresAt = nil
			deletedAt = nil
		}
		if err := iter.Close(); err != nil {
			return shorturl.ListResult{}, err
		}
		if len(records) >= limit {
			break
		}
	}

	var next string
	if len(records) == limit {
		next = encodePageToken(records[len(records)-1].CreatedAt)
	}
	return shorturl.ListResult{URLs: records, NextPageToken: next}, nil
}

func (s *Store) SoftDelete(code string, deletedAt time.Time) error {
	code = normalizeCode(code)
	record, err := s.GetURL(code)
	if err != nil {
		return err
	}
	if err := s.session.Query(
		`UPDATE urls_by_code SET deleted_at = ? WHERE code = ?`,
		deletedAt,
		code,
	).Exec(); err != nil {
		return err
	}
	return s.session.Query(
		`UPDATE urls_by_created_day SET deleted_at = ?
		 WHERE day = ? AND created_at = ? AND code = ?`,
		deletedAt,
		dayKey(record.CreatedAt),
		record.CreatedAt,
		code,
	).Exec()
}

func (s *Store) IncrementHourlyHits(shortURL string, hourStart time.Time) error {
	return s.session.Query(
		`UPDATE hits_by_short_url_hour
		 SET hits = hits + 1
		 WHERE short_url = ? AND hour_start = ?`,
		shortURL,
		hourStart.UTC().Truncate(time.Hour),
	).Exec()
}

func (s *Store) GetHourlyHits(shortURL string, from, to time.Time) ([]shorturl.HitBucket, error) {
	iter := s.session.Query(
		`SELECT hour_start, hits
		 FROM hits_by_short_url_hour
		 WHERE short_url = ? AND hour_start >= ? AND hour_start < ?`,
		shortURL,
		from.UTC().Truncate(time.Hour),
		to.UTC().Truncate(time.Hour),
	).Iter()

	var out []shorturl.HitBucket
	var bucket shorturl.HitBucket
	for iter.Scan(&bucket.HourStart, &bucket.Hits) {
		out = append(out, bucket)
		bucket = shorturl.HitBucket{}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

func beginningOfPreviousDay(t time.Time) time.Time {
	day := time.Date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return day.Add(-time.Nanosecond)
}

func encodePageToken(t time.Time) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano)))
}

func decodePageToken(token string) (time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid page token")
	}
	t, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid page token")
	}
	return t.UTC(), nil
}

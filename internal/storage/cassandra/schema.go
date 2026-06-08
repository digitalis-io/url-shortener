package cassandra

import (
	"fmt"
	"strings"
)

func schemaStatements(keyspace, replicationStrategy string, replicationFactor int) []string {
	replication := fmt.Sprintf("{'class': '%s', 'replication_factor': %d}", replicationStrategy, replicationFactor)
	schema := strings.ReplaceAll(baseSchema, "url_shortener.", keyspace+".")
	schema = strings.ReplaceAll(schema, "CREATE KEYSPACE IF NOT EXISTS url_shortener", "CREATE KEYSPACE IF NOT EXISTS "+keyspace)
	schema = strings.ReplaceAll(schema, "__REPLICATION__", replication)
	parts := strings.Split(schema, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

const baseSchema = `
CREATE KEYSPACE IF NOT EXISTS url_shortener
WITH replication = __REPLICATION__;

CREATE TABLE IF NOT EXISTS url_shortener.urls_by_code (
  code text PRIMARY KEY,
  original_url text,
  url_hash text,
  created_by_id text,
  created_by_email text,
  created_at timestamp,
  expires_at timestamp,
  deleted_at timestamp,
  custom_alias boolean
) WITH default_time_to_live = 7776000;

CREATE TABLE IF NOT EXISTS url_shortener.urls_by_created_day (
  day text,
  created_at timestamp,
  code text,
  original_url text,
  created_by_id text,
  created_by_email text,
  expires_at timestamp,
  deleted_at timestamp,
  custom_alias boolean,
  PRIMARY KEY ((day), created_at, code)
) WITH CLUSTERING ORDER BY (created_at DESC)
  AND default_time_to_live = 7776000;

ALTER TABLE url_shortener.urls_by_code
WITH default_time_to_live = 7776000;

ALTER TABLE url_shortener.urls_by_created_day
WITH default_time_to_live = 7776000;

CREATE TABLE IF NOT EXISTS url_shortener.hits_by_short_url_hour (
  short_url text,
  hour_start timestamp,
  hits counter,
  PRIMARY KEY ((short_url), hour_start)
) WITH CLUSTERING ORDER BY (hour_start DESC);
`

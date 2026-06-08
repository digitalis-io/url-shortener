package cassandra

import (
	"strings"
	"testing"
)

func TestSchemaConfiguresURLTableTTL(t *testing.T) {
	schema := strings.Join(schemaStatements("url_shortener_test", "SimpleStrategy", 1), ";\n")

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS url_shortener_test.urls_by_code",
		"CREATE TABLE IF NOT EXISTS url_shortener_test.urls_by_created_day",
		"default_time_to_live = 7776000",
		"ALTER TABLE url_shortener_test.urls_by_code\nWITH default_time_to_live = 7776000",
		"ALTER TABLE url_shortener_test.urls_by_created_day\nWITH default_time_to_live = 7776000",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema did not contain %q:\n%s", want, schema)
		}
	}
}

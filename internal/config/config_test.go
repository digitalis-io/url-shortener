package config

import (
	"testing"
	"time"
)

func TestLoadCassandraClientTuning(t *testing.T) {
	t.Setenv("CASSANDRA_CONSISTENCY", "QUORUM")
	t.Setenv("CASSANDRA_SERIAL_CONSISTENCY", "SERIAL")
	t.Setenv("CASSANDRA_LOCAL_DC", "dc1")
	t.Setenv("CASSANDRA_PROTO_VERSION", "4")
	t.Setenv("CASSANDRA_NUM_CONNS", "8")
	t.Setenv("CASSANDRA_PAGE_SIZE", "2000")
	t.Setenv("CASSANDRA_TIMEOUT", "2s")
	t.Setenv("CASSANDRA_RETRY_ATTEMPTS", "4")
	t.Setenv("CASSANDRA_SPECULATIVE_EXECUTION_ENABLED", "true")
	t.Setenv("CASSANDRA_SPECULATIVE_DELAY", "25ms")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.CassandraConsistency != "QUORUM" {
		t.Fatalf("unexpected consistency: %s", cfg.CassandraConsistency)
	}
	if cfg.CassandraSerialConsistency != "SERIAL" {
		t.Fatalf("unexpected serial consistency: %s", cfg.CassandraSerialConsistency)
	}
	if cfg.CassandraLocalDC != "dc1" {
		t.Fatalf("unexpected local DC: %s", cfg.CassandraLocalDC)
	}
	if cfg.CassandraNumConns != 8 {
		t.Fatalf("unexpected num conns: %d", cfg.CassandraNumConns)
	}
	if cfg.CassandraPageSize != 2000 {
		t.Fatalf("unexpected page size: %d", cfg.CassandraPageSize)
	}
	if cfg.CassandraTimeout != 2*time.Second {
		t.Fatalf("unexpected timeout: %v", cfg.CassandraTimeout)
	}
	if cfg.CassandraRetryAttempts != 4 {
		t.Fatalf("unexpected retry attempts: %d", cfg.CassandraRetryAttempts)
	}
	if !cfg.CassandraSpeculativeEnabled {
		t.Fatal("expected speculative execution to be enabled")
	}
	if cfg.CassandraSpeculativeDelay != 25*time.Millisecond {
		t.Fatalf("unexpected speculative delay: %v", cfg.CassandraSpeculativeDelay)
	}
}

func TestLoadCassandraClientTuningDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.CassandraConsistency != "LOCAL_QUORUM" {
		t.Fatalf("expected default LOCAL_QUORUM, got %s", cfg.CassandraConsistency)
	}
	if cfg.CassandraSerialConsistency != "LOCAL_SERIAL" {
		t.Fatalf("expected default LOCAL_SERIAL, got %s", cfg.CassandraSerialConsistency)
	}
	if cfg.CassandraNumConns != 2 {
		t.Fatalf("expected default 2 conns, got %d", cfg.CassandraNumConns)
	}
	if cfg.CassandraRetryAttempts != 3 {
		t.Fatalf("expected default 3 retries, got %d", cfg.CassandraRetryAttempts)
	}
	if cfg.CassandraSpeculativeEnabled {
		t.Fatal("expected speculative execution disabled by default")
	}
}

func TestLoadCassandraSSLConfig(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:8080")
	t.Setenv("ADMIN_BASE_URL", "http://localhost:8080")
	t.Setenv("CASSANDRA_SSL_ENABLED", "true")
	t.Setenv("CASSANDRA_SSL_CA_FILE", "/etc/certs/ca.pem")
	t.Setenv("CASSANDRA_SSL_SERVER_NAME", "cassandra.example.com")
	t.Setenv("CASSANDRA_SSL_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("CASSANDRA_SSL_CERT_FILE", "/etc/certs/client.pem")
	t.Setenv("CASSANDRA_SSL_KEY_FILE", "/etc/certs/client-key.pem")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.CassandraSSLEnabled {
		t.Fatal("expected Cassandra SSL to be enabled")
	}
	if cfg.CassandraSSLCAFile != "/etc/certs/ca.pem" {
		t.Fatalf("unexpected CA file: %s", cfg.CassandraSSLCAFile)
	}
	if cfg.CassandraSSLServerName != "cassandra.example.com" {
		t.Fatalf("unexpected server name: %s", cfg.CassandraSSLServerName)
	}
	if !cfg.CassandraSSLInsecureSkipVerify {
		t.Fatal("expected insecure skip verify to be enabled")
	}
	if cfg.CassandraSSLCertFile != "/etc/certs/client.pem" {
		t.Fatalf("unexpected client cert file: %s", cfg.CassandraSSLCertFile)
	}
	if cfg.CassandraSSLKeyFile != "/etc/certs/client-key.pem" {
		t.Fatalf("unexpected client key file: %s", cfg.CassandraSSLKeyFile)
	}
}

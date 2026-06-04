package config

import "testing"

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

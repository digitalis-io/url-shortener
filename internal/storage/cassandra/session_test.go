package cassandra

import (
	"crypto/tls"
	"testing"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"

	"github.com/digitalis-io/url-shortner/internal/config"
)

func TestParseConsistency(t *testing.T) {
	got, err := parseConsistency("", gocql.LocalQuorum)
	if err != nil {
		t.Fatalf("parseConsistency empty: %v", err)
	}
	if got != gocql.LocalQuorum {
		t.Fatalf("expected fallback LocalQuorum, got %v", got)
	}

	got, err = parseConsistency("quorum", gocql.LocalQuorum)
	if err != nil {
		t.Fatalf("parseConsistency quorum: %v", err)
	}
	if got != gocql.Quorum {
		t.Fatalf("expected Quorum, got %v", got)
	}

	if _, err := parseConsistency("NOT_A_LEVEL", gocql.LocalQuorum); err == nil {
		t.Fatal("expected error for invalid consistency level")
	}
}

func TestBaseClusterAppliesTuning(t *testing.T) {
	cfg := config.Config{
		CassandraHosts:                  []string{"localhost:9042"},
		CassandraConsistency:            "EACH_QUORUM",
		CassandraSerialConsistency:      "SERIAL",
		CassandraLocalDC:                "dc1",
		CassandraProtoVersion:           4,
		CassandraNumConns:               4,
		CassandraPageSize:               1234,
		CassandraConnectTimeout:         3 * time.Second,
		CassandraTimeout:                4 * time.Second,
		CassandraWriteTimeout:           2 * time.Second,
		CassandraReconnectInterval:      30 * time.Second,
		CassandraSocketKeepalive:        15 * time.Second,
		CassandraMaxWaitSchemaAgreement: 45 * time.Second,
		CassandraRetryAttempts:          5,
		CassandraRetryMinBackoff:        50 * time.Millisecond,
		CassandraRetryMaxBackoff:        1 * time.Second,
	}

	cluster, err := baseCluster(cfg)
	if err != nil {
		t.Fatalf("baseCluster: %v", err)
	}

	if cluster.Consistency != gocql.EachQuorum {
		t.Fatalf("unexpected consistency: %v", cluster.Consistency)
	}
	if cluster.SerialConsistency != gocql.Serial {
		t.Fatalf("unexpected serial consistency: %v", cluster.SerialConsistency)
	}
	if cluster.ProtoVersion != 4 {
		t.Fatalf("unexpected proto version: %d", cluster.ProtoVersion)
	}
	if cluster.NumConns != 4 {
		t.Fatalf("unexpected num conns: %d", cluster.NumConns)
	}
	if cluster.PageSize != 1234 {
		t.Fatalf("unexpected page size: %d", cluster.PageSize)
	}
	if cluster.ConnectTimeout != 3*time.Second {
		t.Fatalf("unexpected connect timeout: %v", cluster.ConnectTimeout)
	}
	if cluster.Timeout != 4*time.Second {
		t.Fatalf("unexpected timeout: %v", cluster.Timeout)
	}
	if cluster.WriteTimeout != 2*time.Second {
		t.Fatalf("unexpected write timeout: %v", cluster.WriteTimeout)
	}
	if cluster.ReconnectInterval != 30*time.Second {
		t.Fatalf("unexpected reconnect interval: %v", cluster.ReconnectInterval)
	}
	if cluster.SocketKeepalive != 15*time.Second {
		t.Fatalf("unexpected socket keepalive: %v", cluster.SocketKeepalive)
	}
	if cluster.MaxWaitSchemaAgreement != 45*time.Second {
		t.Fatalf("unexpected schema agreement wait: %v", cluster.MaxWaitSchemaAgreement)
	}
	if cluster.PoolConfig.HostSelectionPolicy == nil {
		t.Fatal("expected a host selection policy")
	}
	retry, ok := cluster.RetryPolicy.(*gocql.ExponentialBackoffRetryPolicy)
	if !ok {
		t.Fatalf("expected exponential backoff retry policy, got %T", cluster.RetryPolicy)
	}
	if retry.NumRetries != 5 || retry.Min != 50*time.Millisecond || retry.Max != 1*time.Second {
		t.Fatalf("unexpected retry policy: %+v", retry)
	}
}

func TestBaseClusterRejectsInvalidConsistency(t *testing.T) {
	_, err := baseCluster(config.Config{
		CassandraHosts:       []string{"localhost:9042"},
		CassandraConsistency: "BOGUS",
	})
	if err == nil {
		t.Fatal("expected error for invalid consistency")
	}
}

func TestSpeculativeExecutionPolicy(t *testing.T) {
	if policy := speculativeExecutionPolicy(config.Config{}); policy != nil {
		t.Fatalf("expected nil policy when disabled, got %+v", policy)
	}

	policy := speculativeExecutionPolicy(config.Config{
		CassandraSpeculativeEnabled:  true,
		CassandraSpeculativeAttempts: 2,
		CassandraSpeculativeDelay:    20 * time.Millisecond,
	})
	simple, ok := policy.(*gocql.SimpleSpeculativeExecution)
	if !ok {
		t.Fatalf("expected SimpleSpeculativeExecution, got %T", policy)
	}
	if simple.NumAttempts != 2 || simple.TimeoutDelay != 20*time.Millisecond {
		t.Fatalf("unexpected speculative policy: %+v", simple)
	}
}

func TestCassandraSSLOptionsDisabled(t *testing.T) {
	opts, err := cassandraSSLOptions(config.Config{})
	if err != nil {
		t.Fatalf("cassandraSSLOptions: %v", err)
	}
	if opts != nil {
		t.Fatalf("expected nil SSL options when disabled, got %+v", opts)
	}
}

func TestCassandraSSLOptionsEnabledWithVerification(t *testing.T) {
	opts, err := cassandraSSLOptions(config.Config{
		CassandraSSLEnabled:    true,
		CassandraSSLCAFile:     "/etc/cassandra/ca.pem",
		CassandraSSLServerName: "cassandra.example.com",
		CassandraSSLCertFile:   "/etc/cassandra/client.pem",
		CassandraSSLKeyFile:    "/etc/cassandra/client-key.pem",
	})
	if err != nil {
		t.Fatalf("cassandraSSLOptions: %v", err)
	}
	if opts == nil {
		t.Fatal("expected SSL options")
	}
	if opts.Config == nil {
		t.Fatal("expected TLS config")
	}
	if opts.Config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("unexpected minimum TLS version: %d", opts.Config.MinVersion)
	}
	if opts.Config.ServerName != "cassandra.example.com" {
		t.Fatalf("unexpected server name: %s", opts.Config.ServerName)
	}
	if opts.Config.InsecureSkipVerify {
		t.Fatal("expected certificate verification by default")
	}
	if !opts.EnableHostVerification {
		t.Fatal("expected host verification by default")
	}
	if opts.CaPath != "/etc/cassandra/ca.pem" {
		t.Fatalf("unexpected CA path: %s", opts.CaPath)
	}
	if opts.CertPath != "/etc/cassandra/client.pem" {
		t.Fatalf("unexpected client cert path: %s", opts.CertPath)
	}
	if opts.KeyPath != "/etc/cassandra/client-key.pem" {
		t.Fatalf("unexpected client key path: %s", opts.KeyPath)
	}
}

func TestCassandraSSLOptionsInsecureSkipVerify(t *testing.T) {
	opts, err := cassandraSSLOptions(config.Config{
		CassandraSSLEnabled:            true,
		CassandraSSLInsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("cassandraSSLOptions: %v", err)
	}
	if opts == nil || opts.Config == nil {
		t.Fatal("expected TLS config")
	}
	if !opts.Config.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify")
	}
	if opts.EnableHostVerification {
		t.Fatal("expected host verification to be disabled only for explicit insecure mode")
	}
}

func TestCassandraSSLOptionsRequiresClientCertPair(t *testing.T) {
	_, err := cassandraSSLOptions(config.Config{
		CassandraSSLEnabled:  true,
		CassandraSSLCertFile: "/etc/cassandra/client.pem",
	})
	if err == nil {
		t.Fatal("expected error for incomplete client cert pair")
	}
}

package cassandra

import (
	"crypto/tls"
	"testing"

	"github.com/digitalis-io/url-shortner/internal/config"
)

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

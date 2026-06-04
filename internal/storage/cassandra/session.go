package cassandra

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"

	"github.com/digitalis-io/url-shortner/internal/config"
)

type Store struct {
	session  *gocql.Session
	keyspace string
}

func Connect(cfg config.Config) (*Store, error) {
	if err := ensureSchema(cfg); err != nil {
		return nil, err
	}

	cluster, err := baseCluster(cfg)
	if err != nil {
		return nil, err
	}
	cluster.Keyspace = cfg.CassandraKeyspace
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	return &Store{session: session, keyspace: cfg.CassandraKeyspace}, nil
}

func baseCluster(cfg config.Config) (*gocql.ClusterConfig, error) {
	cluster := gocql.NewCluster(cfg.CassandraHosts...)
	cluster.Consistency = gocql.LocalQuorum
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Timeout = 10 * time.Second
	if cfg.CassandraUsername != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.CassandraUsername,
			Password: cfg.CassandraPassword,
		}
	}
	sslOpts, err := cassandraSSLOptions(cfg)
	if err != nil {
		return nil, err
	}
	cluster.SslOpts = sslOpts
	return cluster, nil
}

func cassandraSSLOptions(cfg config.Config) (*gocql.SslOptions, error) {
	if !cfg.CassandraSSLEnabled {
		return nil, nil
	}
	if (cfg.CassandraSSLCertFile == "") != (cfg.CassandraSSLKeyFile == "") {
		return nil, errors.New("CASSANDRA_SSL_CERT_FILE and CASSANDRA_SSL_KEY_FILE must be configured together")
	}

	return &gocql.SslOptions{
		Config: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			ServerName:         cfg.CassandraSSLServerName,
			InsecureSkipVerify: cfg.CassandraSSLInsecureSkipVerify,
		},
		CaPath:                 cfg.CassandraSSLCAFile,
		CertPath:               cfg.CassandraSSLCertFile,
		KeyPath:                cfg.CassandraSSLKeyFile,
		EnableHostVerification: !cfg.CassandraSSLInsecureSkipVerify,
	}, nil
}

func ensureSchema(cfg config.Config) error {
	cluster, err := baseCluster(cfg)
	if err != nil {
		return err
	}
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()

	for _, stmt := range schemaStatements(cfg.CassandraKeyspace) {
		if err := session.Query(stmt).Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() {
	if s != nil && s.session != nil {
		s.session.Close()
	}
}

func (s *Store) Ready(ctx context.Context) error {
	return s.session.Query("SELECT release_version FROM system.local").WithContext(ctx).Exec()
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func normalizeCode(code string) string {
	return strings.TrimSpace(code)
}

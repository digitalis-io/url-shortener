package cassandra

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"

	"github.com/digitalis-io/url-shortner/internal/config"
)

type Store struct {
	session  *gocql.Session
	keyspace string
	specExec gocql.SpeculativeExecutionPolicy
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
	return &Store{
		session:  session,
		keyspace: cfg.CassandraKeyspace,
		specExec: speculativeExecutionPolicy(cfg),
	}, nil
}

func baseCluster(cfg config.Config) (*gocql.ClusterConfig, error) {
	cluster := gocql.NewCluster(cfg.CassandraHosts...)

	consistency, err := parseConsistency(cfg.CassandraConsistency, gocql.LocalQuorum)
	if err != nil {
		return nil, err
	}
	serial, err := parseConsistency(cfg.CassandraSerialConsistency, gocql.LocalSerial)
	if err != nil {
		return nil, err
	}
	cluster.Consistency = consistency
	cluster.SerialConsistency = serial

	cluster.ConnectTimeout = durationOr(cfg.CassandraConnectTimeout, 10*time.Second)
	cluster.Timeout = durationOr(cfg.CassandraTimeout, 10*time.Second)
	if cfg.CassandraWriteTimeout > 0 {
		cluster.WriteTimeout = cfg.CassandraWriteTimeout
	}
	if cfg.CassandraProtoVersion > 0 {
		cluster.ProtoVersion = cfg.CassandraProtoVersion
	}
	if cfg.CassandraNumConns > 0 {
		cluster.NumConns = cfg.CassandraNumConns
	}
	if cfg.CassandraPageSize > 0 {
		cluster.PageSize = cfg.CassandraPageSize
	}
	if cfg.CassandraReconnectInterval > 0 {
		cluster.ReconnectInterval = cfg.CassandraReconnectInterval
	}
	if cfg.CassandraSocketKeepalive > 0 {
		cluster.SocketKeepalive = cfg.CassandraSocketKeepalive
	}
	if cfg.CassandraMaxWaitSchemaAgreement > 0 {
		cluster.MaxWaitSchemaAgreement = cfg.CassandraMaxWaitSchemaAgreement
	}

	// Token-aware routing keeps coordinator selection on replicas that own the
	// partition; when a local DC is configured the fallback stays DC-local so a
	// LOCAL_* consistency level does not silently cross datacenters.
	var fallback gocql.HostSelectionPolicy
	if cfg.CassandraLocalDC != "" {
		fallback = gocql.DCAwareRoundRobinPolicy(cfg.CassandraLocalDC)
	} else {
		fallback = gocql.RoundRobinHostPolicy()
	}
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(fallback)

	if cfg.CassandraRetryAttempts > 0 {
		cluster.RetryPolicy = &gocql.ExponentialBackoffRetryPolicy{
			NumRetries: cfg.CassandraRetryAttempts,
			Min:        durationOr(cfg.CassandraRetryMinBackoff, 100*time.Millisecond),
			Max:        durationOr(cfg.CassandraRetryMaxBackoff, 2*time.Second),
		}
	}

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

// parseConsistency resolves a configured consistency name, falling back when the
// value is empty and returning a descriptive error for an unknown level.
func parseConsistency(name string, fallback gocql.Consistency) (gocql.Consistency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback, nil
	}
	consistency, err := gocql.ParseConsistencyWrapper(name)
	if err != nil {
		return 0, fmt.Errorf("invalid cassandra consistency %q: %w", name, err)
	}
	return consistency, nil
}

// speculativeExecutionPolicy returns a per-query speculative execution policy
// when enabled, or nil to leave speculative execution off.
func speculativeExecutionPolicy(cfg config.Config) gocql.SpeculativeExecutionPolicy {
	if !cfg.CassandraSpeculativeEnabled {
		return nil
	}
	attempts := cfg.CassandraSpeculativeAttempts
	if attempts <= 0 {
		attempts = 1
	}
	return &gocql.SimpleSpeculativeExecution{
		NumAttempts:  attempts,
		TimeoutDelay: durationOr(cfg.CassandraSpeculativeDelay, 50*time.Millisecond),
	}
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
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
	if !cfg.CassandraCreateKeyspace {
		return nil
	}

	stmts := schemaStatements(cfg.CassandraKeyspace, cfg.CassandraReplicationStrategy, cfg.CassandraReplicationFactor)

	cluster, err := baseCluster(cfg)
	if err != nil {
		return err
	}

	// CREATE KEYSPACE requires a no-keyspace session; needs CREATE ON ALL KEYSPACES.
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if !isKeyspaceDDL(stmt) {
			continue
		}
		if err := session.Query(stmt).Exec(); err != nil {
			session.Close()
			return err
		}
	}
	session.Close()

	// gocql forbids reusing a TokenAwareHostPolicy across sessions — build a fresh cluster.
	cluster, err = baseCluster(cfg)
	if err != nil {
		return err
	}
	cluster.Keyspace = cfg.CassandraKeyspace
	session, err = cluster.CreateSession()
	if err != nil {
		return err
	}
	defer session.Close()
	for _, stmt := range stmts {
		if isKeyspaceDDL(stmt) {
			continue
		}
		if err := session.Query(stmt).Exec(); err != nil {
			return err
		}
	}
	return nil
}

func isKeyspaceDDL(stmt string) bool {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	return strings.HasPrefix(upper, "CREATE KEYSPACE") || strings.HasPrefix(upper, "DROP KEYSPACE") || strings.HasPrefix(upper, "ALTER KEYSPACE")
}

func (s *Store) Close() {
	if s != nil && s.session != nil {
		s.session.Close()
	}
}

func (s *Store) Ready(ctx context.Context) error {
	return s.readQuery("SELECT release_version FROM system.local").WithContext(ctx).Exec()
}

// readQuery builds a query for a read-only, retry-safe statement. Reads are
// marked idempotent so the retry policy may safely re-issue them, and inherit
// the configured speculative execution policy to trim tail latency.
func (s *Store) readQuery(stmt string, values ...any) *gocql.Query {
	query := s.session.Query(stmt, values...).Idempotent(true)
	if s.specExec != nil {
		query.SetSpeculativeExecutionPolicy(s.specExec)
	}
	return query
}

func dayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func normalizeCode(code string) string {
	return strings.TrimSpace(code)
}

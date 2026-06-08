package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv        string
	HTTPAddr      string
	PublicBaseURL string
	AdminBaseURL  string
	PublicHost    string
	AdminHost     string

	CassandraHosts                 []string
	CassandraKeyspace              string
	CassandraUsername              string
	CassandraPassword              string
	CassandraSSLEnabled            bool
	CassandraSSLCAFile             string
	CassandraSSLServerName         string
	CassandraSSLInsecureSkipVerify bool
	CassandraSSLCertFile           string
	CassandraSSLKeyFile            string

	CassandraConsistency            string
	CassandraSerialConsistency      string
	CassandraLocalDC                string
	CassandraProtoVersion           int
	CassandraNumConns               int
	CassandraPageSize               int
	CassandraConnectTimeout         time.Duration
	CassandraTimeout                time.Duration
	CassandraWriteTimeout           time.Duration
	CassandraReconnectInterval      time.Duration
	CassandraSocketKeepalive        time.Duration
	CassandraMaxWaitSchemaAgreement time.Duration
	CassandraRetryAttempts          int
	CassandraRetryMinBackoff        time.Duration
	CassandraRetryMaxBackoff        time.Duration
	CassandraSpeculativeEnabled     bool
	CassandraSpeculativeAttempts    int
	CassandraSpeculativeDelay       time.Duration

	SAMLEntityID        string
	SAMLACSURL          string
	SAMLIDPMetadataURL  string
	SAMLIDPMetadata     string
	SAMLPrivateKey      string
	SAMLCertificate     string
	SessionSecret       string
	AuthDevBypass       bool
	AuthHeaderEnabled   bool
	AuthUserEmailHeader string
	DevUserID           string
	DevUserEmail        string

	CodeLength              int
	CreateRateLimitPerMin   int
	ClickEventBufferSize    int
	ClickEventFlushInterval time.Duration
}

func Load() (Config, error) {
	publicBaseURL := env("PUBLIC_BASE_URL", "http://localhost:8080")
	adminBaseURL := env("ADMIN_BASE_URL", publicBaseURL)

	cfg := Config{
		AppEnv:                         env("APP_ENV", "local"),
		HTTPAddr:                       env("HTTP_ADDR", ":8080"),
		PublicBaseURL:                  trimRightSlash(publicBaseURL),
		AdminBaseURL:                   trimRightSlash(adminBaseURL),
		CassandraHosts:                 splitCSV(env("CASSANDRA_HOSTS", "localhost:9042")),
		CassandraKeyspace:              env("CASSANDRA_KEYSPACE", "url_shortener"),
		CassandraUsername:              os.Getenv("CASSANDRA_USERNAME"),
		CassandraPassword:              os.Getenv("CASSANDRA_PASSWORD"),
		CassandraSSLEnabled:            envBool("CASSANDRA_SSL_ENABLED", false),
		CassandraSSLCAFile:             os.Getenv("CASSANDRA_SSL_CA_FILE"),
		CassandraSSLServerName:         os.Getenv("CASSANDRA_SSL_SERVER_NAME"),
		CassandraSSLInsecureSkipVerify: envBool("CASSANDRA_SSL_INSECURE_SKIP_VERIFY", false),
		CassandraSSLCertFile:           os.Getenv("CASSANDRA_SSL_CERT_FILE"),
		CassandraSSLKeyFile:            os.Getenv("CASSANDRA_SSL_KEY_FILE"),

		CassandraConsistency:            env("CASSANDRA_CONSISTENCY", "LOCAL_QUORUM"),
		CassandraSerialConsistency:      env("CASSANDRA_SERIAL_CONSISTENCY", "LOCAL_SERIAL"),
		CassandraLocalDC:                os.Getenv("CASSANDRA_LOCAL_DC"),
		CassandraProtoVersion:           envInt("CASSANDRA_PROTO_VERSION", 4),
		CassandraNumConns:               envInt("CASSANDRA_NUM_CONNS", 2),
		CassandraPageSize:               envInt("CASSANDRA_PAGE_SIZE", 5000),
		CassandraConnectTimeout:         envDuration("CASSANDRA_CONNECT_TIMEOUT", 10*time.Second),
		CassandraTimeout:                envDuration("CASSANDRA_TIMEOUT", 10*time.Second),
		CassandraWriteTimeout:           envDuration("CASSANDRA_WRITE_TIMEOUT", 0),
		CassandraReconnectInterval:      envDuration("CASSANDRA_RECONNECT_INTERVAL", 60*time.Second),
		CassandraSocketKeepalive:        envDuration("CASSANDRA_SOCKET_KEEPALIVE", 15*time.Second),
		CassandraMaxWaitSchemaAgreement: envDuration("CASSANDRA_MAX_WAIT_SCHEMA_AGREEMENT", 60*time.Second),
		CassandraRetryAttempts:          envInt("CASSANDRA_RETRY_ATTEMPTS", 3),
		CassandraRetryMinBackoff:        envDuration("CASSANDRA_RETRY_MIN_BACKOFF", 100*time.Millisecond),
		CassandraRetryMaxBackoff:        envDuration("CASSANDRA_RETRY_MAX_BACKOFF", 2*time.Second),
		CassandraSpeculativeEnabled:     envBool("CASSANDRA_SPECULATIVE_EXECUTION_ENABLED", false),
		CassandraSpeculativeAttempts:    envInt("CASSANDRA_SPECULATIVE_ATTEMPTS", 1),
		CassandraSpeculativeDelay:       envDuration("CASSANDRA_SPECULATIVE_DELAY", 50*time.Millisecond),

		SAMLEntityID:            env("SAML_ENTITY_ID", adminBaseURL+"/saml/metadata"),
		SAMLACSURL:              env("SAML_ACS_URL", adminBaseURL+"/saml/acs"),
		SAMLIDPMetadataURL:      os.Getenv("SAML_IDP_METADATA_URL"),
		SAMLIDPMetadata:         os.Getenv("SAML_IDP_METADATA"),
		SAMLPrivateKey:          os.Getenv("SAML_PRIVATE_KEY"),
		SAMLCertificate:         os.Getenv("SAML_CERTIFICATE"),
		SessionSecret:           os.Getenv("SESSION_SECRET"),
		AuthDevBypass:           envBool("AUTH_DEV_BYPASS", false),
		AuthHeaderEnabled:       envBool("AUTH_HEADER_ENABLED", false),
		AuthUserEmailHeader:     env("AUTH_USER_EMAIL_HEADER", "Cf-Access-Authenticated-User-Email"),
		DevUserID:               env("DEV_USER_ID", "local-dev-user"),
		DevUserEmail:            env("DEV_USER_EMAIL", "dev@example.com"),
		CodeLength:              envInt("CODE_LENGTH", 7),
		CreateRateLimitPerMin:   envInt("CREATE_RATE_LIMIT_PER_MINUTE", 60),
		ClickEventBufferSize:    envInt("CLICK_EVENT_BUFFER_SIZE", 1000),
		ClickEventFlushInterval: envDuration("CLICK_EVENT_FLUSH_INTERVAL", time.Second),
	}

	cfg.PublicHost = hostFromURL(cfg.PublicBaseURL)
	cfg.AdminHost = hostFromURL(cfg.AdminBaseURL)
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func trimRightSlash(value string) string {
	return strings.TrimRight(value, "/")
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

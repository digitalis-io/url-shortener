package shorturl

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const MaxURLLength = 4096

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,64}$`)

var reservedAliases = map[string]struct{}{
	"admin":   {},
	"api":     {},
	"healthz": {},
	"login":   {},
	"logout":  {},
	"metrics": {},
	"readyz":  {},
	"saml":    {},
	"static":  {},
}

func ValidateAndNormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("url is required")
	}
	if len(raw) > MaxURLLength {
		return "", errors.New("url is too long")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("url host is required")
	}
	return parsed.String(), nil
}

func ValidateAlias(alias string) error {
	if alias == "" {
		return nil
	}
	if !aliasPattern.MatchString(alias) {
		return errors.New("alias must be 3-64 characters using letters, numbers, underscore, or hyphen")
	}
	if _, reserved := reservedAliases[strings.ToLower(alias)]; reserved {
		return errors.New("alias is reserved")
	}
	return nil
}

func ValidateExpiration(expiresAt *time.Time, now time.Time) error {
	if expiresAt == nil {
		return nil
	}
	if !expiresAt.After(now) {
		return errors.New("expires_at must be in the future")
	}
	return nil
}

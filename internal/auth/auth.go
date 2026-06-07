package auth

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"

	"github.com/digitalis-io/url-shortner/internal/config"
	"github.com/digitalis-io/url-shortner/internal/shorturl"
)

const (
	sessionCookie = "url_shortener_session"
	csrfCookie    = "url_shortener_csrf"
)

type contextKey string

const userContextKey contextKey = "user"

type Auth struct {
	cfg    config.Config
	secret []byte
	secure bool
	samlSP *samlsp.Middleware
}

type sessionPayload struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func New(cfg config.Config) (*Auth, error) {
	secret := []byte(cfg.SessionSecret)
	if len(secret) == 0 {
		secret = []byte("local-dev-session-secret-change-me")
	}
	a := &Auth{
		cfg:    cfg,
		secret: secret,
		secure: strings.HasPrefix(cfg.AdminBaseURL, "https://"),
	}

	samlSP, err := newSAMLSP(cfg)
	if err != nil {
		return nil, err
	}
	a.samlSP = samlSP
	return a, nil
}

func UserFromContext(ctx context.Context) (shorturl.User, bool) {
	user, ok := ctx.Value(userContextKey).(shorturl.User)
	return user, ok
}

func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := a.sessionUser(r)
		if ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
			return
		}
		if a.cfg.AuthDevBypass {
			user := shorturl.User{ID: a.cfg.DevUserID, Email: a.cfg.DevUserEmail}
			a.setSession(w, user)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
			return
		}
		if user, ok := a.samlUser(r); ok {
			a.setSession(w, user)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func (a *Auth) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, session, ok := a.sessionUser(r)
		if !ok {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		token := r.Header.Get("X-CSRF-Token")
		if token == "" || token != session.CSRFToken {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if a.cfg.AuthDevBypass {
		a.setSession(w, shorturl.User{ID: a.cfg.DevUserID, Email: a.cfg.DevUserEmail})
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if a.samlSP == nil {
		http.Error(w, "SAML login is not configured", http.StatusNotImplemented)
		return
	}
	a.samlSP.HandleStartAuthFlow(w, r)
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	a.clearSession(w)
	if a.samlSP != nil {
		_ = a.samlSP.Session.DeleteSession(w, r)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *Auth) Metadata(w http.ResponseWriter, r *http.Request) {
	if a.samlSP == nil {
		http.Error(w, "SAML metadata is not configured", http.StatusNotImplemented)
		return
	}
	a.samlSP.ServeMetadata(w, r)
}

func (a *Auth) ACS(w http.ResponseWriter, r *http.Request) {
	if a.samlSP == nil {
		http.Error(w, "SAML ACS is not configured", http.StatusNotImplemented)
		return
	}
	a.samlSP.ServeACS(w, r)
}

func (a *Auth) CSRFToken(r *http.Request) string {
	_, session, ok := a.sessionUser(r)
	if !ok {
		return ""
	}
	return session.CSRFToken
}

func (a *Auth) setSession(w http.ResponseWriter, user shorturl.User) {
	session := sessionPayload{
		UserID:    user.ID,
		Email:     user.Email,
		CSRFToken: randomToken(),
		ExpiresAt: time.Now().UTC().Add(12 * time.Hour),
	}
	value := a.sign(session)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    session.CSRFToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

func (a *Auth) clearSession(w http.ResponseWriter) {
	expired := time.Now().UTC().Add(-time.Hour)
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", Expires: expired, MaxAge: -1})
	}
}

func (a *Auth) sessionUser(r *http.Request) (shorturl.User, sessionPayload, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return shorturl.User{}, sessionPayload{}, false
	}
	session, err := a.verify(cookie.Value)
	if err != nil || time.Now().UTC().After(session.ExpiresAt) {
		return shorturl.User{}, sessionPayload{}, false
	}
	return shorturl.User{ID: session.UserID, Email: session.Email}, session, true
}

func (a *Auth) samlUser(r *http.Request) (shorturl.User, bool) {
	if a.samlSP == nil {
		return shorturl.User{}, false
	}
	session, err := a.samlSP.Session.GetSession(r)
	if err != nil || session == nil {
		return shorturl.User{}, false
	}
	withAttrs, ok := session.(samlsp.SessionWithAttributes)
	if !ok {
		return shorturl.User{}, false
	}
	attrs := withAttrs.GetAttributes()
	id := firstAttr(attrs,
		"http://schemas.microsoft.com/identity/claims/objectidentifier",
		"objectidentifier",
		"oid",
		"uid",
		"sub",
	)
	email := firstAttr(attrs,
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"emailaddress",
		"email",
		"mail",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"name",
		"upn",
	)
	if id == "" {
		id = email
	}
	if id == "" {
		return shorturl.User{}, false
	}
	return shorturl.User{ID: id, Email: email}, true
}

func firstAttr(attrs samlsp.Attributes, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(attrs.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func (a *Auth) sign(session sessionPayload) string {
	payload, _ := json.Marshal(session)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + sig
}

func (a *Auth) verify(value string) (sessionPayload, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return sessionPayload{}, errors.New("invalid session")
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return sessionPayload{}, errors.New("invalid session")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return sessionPayload{}, err
	}
	var session sessionPayload
	if err := json.Unmarshal(raw, &session); err != nil {
		return sessionPayload{}, err
	}
	return session, nil
}

func newSAMLSP(cfg config.Config) (*samlsp.Middleware, error) {
	hasMetadata := cfg.SAMLIDPMetadataURL != "" || cfg.SAMLIDPMetadata != ""
	hasCert := cfg.SAMLCertificate != "" && cfg.SAMLPrivateKey != ""
	if !hasMetadata && !hasCert {
		return nil, nil
	}
	if !hasMetadata || !hasCert {
		return nil, fmt.Errorf("SAML configuration requires IdP metadata plus a certificate and private key")
	}

	adminURL, err := url.Parse(cfg.AdminBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ADMIN_BASE_URL: %w", err)
	}

	keyPair, err := tls.X509KeyPair([]byte(cfg.SAMLCertificate), []byte(cfg.SAMLPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("load SAML certificate/key: %w", err)
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse SAML certificate: %w", err)
	}
	signer, ok := keyPair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("SAML private key does not implement crypto.Signer")
	}

	metadata, err := loadIDPMetadata(cfg)
	if err != nil {
		return nil, err
	}

	opts := samlsp.Options{
		EntityID:           cfg.SAMLEntityID,
		URL:                *adminURL,
		Key:                signer,
		Certificate:        keyPair.Leaf,
		IDPMetadata:        metadata,
		DefaultRedirectURI: "/",
		CookieName:         "url_shortener_saml",
		CookieSameSite:     http.SameSiteLaxMode,
	}
	return samlsp.New(opts)
}

func loadIDPMetadata(cfg config.Config) (*saml.EntityDescriptor, error) {
	if cfg.SAMLIDPMetadata != "" {
		metadata, err := samlsp.ParseMetadata([]byte(cfg.SAMLIDPMetadata))
		if err != nil {
			return nil, fmt.Errorf("parse SAML IdP metadata: %w", err)
		}
		return metadata, nil
	}

	metadataURL, err := url.Parse(cfg.SAMLIDPMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("parse SAML_IDP_METADATA_URL: %w", err)
	}
	metadata, err := samlsp.FetchMetadata(context.Background(), http.DefaultClient, *metadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetch SAML IdP metadata: %w", err)
	}
	return metadata, nil
}

func randomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

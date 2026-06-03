package auth

import (
	"testing"

	"github.com/digitalis-io/url-shortner/internal/config"
)

func TestNewAllowsMissingSAMLConfig(t *testing.T) {
	authn, err := New(config.Config{
		AdminBaseURL:  "http://localhost:8080",
		SessionSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if authn == nil {
		t.Fatal("expected auth instance")
	}
}

func TestNewRejectsPartialSAMLConfig(t *testing.T) {
	_, err := New(config.Config{
		AdminBaseURL:        "http://localhost:8080",
		SessionSecret:       "test-secret",
		SAMLIDPMetadataURL:  "https://login.microsoftonline.com/example/federationmetadata/2007-06/federationmetadata.xml",
		SAMLCertificateFile: "",
		SAMLPrivateKeyFile:  "",
	})
	if err == nil {
		t.Fatal("expected partial SAML config error")
	}
}

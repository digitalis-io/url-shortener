package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

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
		AdminBaseURL:       "http://localhost:8080",
		SessionSecret:      "test-secret",
		SAMLIDPMetadataURL: "https://login.microsoftonline.com/example/federationmetadata/2007-06/federationmetadata.xml",
		SAMLCertificate:    "",
		SAMLPrivateKey:     "",
	})
	if err == nil {
		t.Fatal("expected partial SAML config error")
	}
}

func TestNewBuildsSAMLSPFromInlineValues(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedPEM(t)

	authn, err := New(config.Config{
		AdminBaseURL:    "https://admin.example.com",
		SessionSecret:   "test-secret",
		SAMLEntityID:    "https://admin.example.com/saml/metadata",
		SAMLCertificate: certPEM,
		SAMLPrivateKey:  keyPEM,
		SAMLIDPMetadata: idpMetadataXML,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if authn.samlSP == nil {
		t.Fatal("expected SAML SP to be configured from inline values")
	}
}

func TestNewRejectsInvalidInlineCertificate(t *testing.T) {
	_, err := New(config.Config{
		AdminBaseURL:    "https://admin.example.com",
		SessionSecret:   "test-secret",
		SAMLCertificate: "not-a-pem-certificate",
		SAMLPrivateKey:  "not-a-pem-key",
		SAMLIDPMetadata: idpMetadataXML,
	})
	if err == nil {
		t.Fatal("expected error for invalid inline certificate/key")
	}
}

func generateSelfSignedPEM(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "url-shortener-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return string(certPEM), string(keyPEM)
}

const idpMetadataXML = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com/metadata">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

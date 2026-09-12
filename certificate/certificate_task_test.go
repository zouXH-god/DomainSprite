package certificate

import (
	"DDNSServer/models"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeDomains(t *testing.T) {
	got, sans, _, err := NormalizeCertificateDomains([]models.CertificateDomain{{DomainName: " Example.COM. "}, {DomainName: "example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DomainName != "example.com" {
		t.Fatalf("unexpected: %#v", got)
	}
	if len(sans) != 2 {
		t.Fatalf("unexpected SAN count %d", len(sans))
	}
}

func TestDelegatedRecordNameWithPrefix(t *testing.T) {
	without := DelegatedRecordName("example.com")
	with := DelegatedRecordName("example.com", "cert.prod")
	if with != "cert.prod."+without {
		t.Fatalf("unexpected delegated name %q", with)
	}
	if prefix, err := NormalizeDelegationPrefix(" Cert.Prod. "); err != nil || prefix != "cert.prod" {
		t.Fatalf("unexpected normalized prefix %q: %v", prefix, err)
	}
	if _, err := NormalizeDelegationPrefix("bad_prefix"); err == nil {
		t.Fatal("invalid prefix accepted")
	}
}
func TestNormalizeDomainsRejectsEmpty(t *testing.T) {
	if _, _, _, err := NormalizeCertificateDomains(nil); err == nil {
		t.Fatal("expected error")
	}
}
func TestNormalizeDomainsRejectsInvalidAndTooMany(t *testing.T) {
	for _, name := range []string{"*.example.com", "https://example.com", "a..example.com", "127.0.0.1"} {
		if _, _, _, err := NormalizeCertificateDomains([]models.CertificateDomain{{DomainName: name}}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	many := make([]models.CertificateDomain, 51)
	for i := range many {
		many[i].DomainName = fmt.Sprintf("d%d.example.com", i)
	}
	if _, _, _, err := NormalizeCertificateDomains(many); err == nil {
		t.Fatal("accepted more than 100 SANs")
	}
}
func TestPayloadContainsNoCredentialsOrProvider(t *testing.T) {
	b, err := json.Marshal(CreatePayload{TaskID: "task1"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(strings.ToLower(s), "secret") || strings.Contains(strings.ToLower(s), "provider") {
		t.Fatalf("unsafe payload: %s", s)
	}
	var p CreatePayload
	if err := json.Unmarshal(b, &p); err != nil || p.TaskID != "task1" {
		t.Fatal("payload did not round-trip")
	}
}

func TestTaskLoggerIsIndependentAndStructured(t *testing.T) {
	old := models.AccountConfig.Certificate.SavePath
	models.AccountConfig.Certificate.SavePath = t.TempDir()
	defer func() { models.AccountConfig.Certificate.SavePath = old }()
	p := CreatePayload{TaskID: "task-abc", CertificateID: 42}
	logger, closeLog, err := taskLogger(p)
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("task.failed", "stage", "issued", "error", errors.New("disk full"))
	closeLog()
	b, err := os.ReadFile(filepath.Join(models.AccountConfig.Certificate.SavePath, "logs", "task-abc.log"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"task_id":"task-abc"`, `"certificate_id":42`, `"msg":"task.failed"`, `"stage":"issued"`, `"error":"disk full"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("log missing %s: %s", want, s)
		}
	}
}

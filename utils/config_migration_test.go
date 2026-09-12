package utils

import (
	"DDNSServer/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeLegacyConfigRemovesSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("legacy-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := models.Config{BaseConfig: models.BaseConfig{Host: "127.0.0.1", Port: "2485", RedisPoint: "localhost:6379", AccessKeySecret: "legacy-secret", EncryptionKeyEnv: "MASTER"}, Certificate: models.CertificateConfig{SavePath: "./certs", EmailList: []string{"admin@example.com"}}, FastConfig: models.FastConfig{DataPath: "./data", AccessSalt: "fast-secret"}, Accounts: []models.Account{{Name: "dns", AccessKeySecret: "provider-secret"}}}
	if err := SanitizeLegacyConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, secret := range []string{"legacy-secret", "fast-secret", "provider-secret", "admin@example.com"} {
		if strings.Contains(text, secret) {
			t.Fatalf("sanitized config leaked %q", secret)
		}
	}
}

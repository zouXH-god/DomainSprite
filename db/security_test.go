package db

import (
	"encoding/base64"
	"strings"
	"testing"
)

func initTestKey(t *testing.T) {
	t.Helper()
	name := "DOMAINSPRITE_TEST_MASTER_KEY"
	t.Setenv(name, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err := InitMasterKey(name); err != nil {
		t.Fatal(err)
	}
}
func TestSecretEncryptionAuthenticated(t *testing.T) {
	initTestKey(t)
	encrypted, err := EncryptSecret("cloud-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "cloud-secret") {
		t.Fatal("ciphertext contains plaintext")
	}
	plain, err := DecryptSecret(encrypted)
	if err != nil || plain != "cloud-secret" {
		t.Fatalf("roundtrip=%q err=%v", plain, err)
	}
	last := encrypted[len(encrypted)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	if _, err = DecryptSecret(encrypted[:len(encrypted)-1] + string(replacement)); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
func TestPasswordAndTokenHashing(t *testing.T) {
	hash, err := HashPassword("a-long-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "a-long-password" || !VerifyPassword(hash, "a-long-password") || VerifyPassword(hash, "wrong-password") {
		t.Fatal("password verification failed")
	}
	token, _ := RandomToken(32)
	if token == "" || HashToken(token) == token {
		t.Fatal("token generation/hash failed")
	}
}

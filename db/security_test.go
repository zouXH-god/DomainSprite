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
	encoded := strings.TrimPrefix(encrypted, "v1:")
	ciphertext, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	tampered := "v1:" + base64.RawStdEncoding.EncodeToString(ciphertext)
	if _, err = DecryptSecret(tampered); err == nil {
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

func TestInitMasterKeyValueSupportsBase64AndHex(t *testing.T) {
	raw := []byte("0123456789abcdef0123456789abcdef")
	for name, value := range map[string]string{
		"base64": base64.StdEncoding.EncodeToString(raw),
		"hex":    "3031323334353637383961626364656630313233343536373839616263646566",
	} {
		t.Run(name, func(t *testing.T) {
			if err := InitMasterKeyValue(value); err != nil {
				t.Fatal(err)
			}
			encrypted, err := EncryptSecret("direct-config-secret")
			if err != nil {
				t.Fatal(err)
			}
			plain, err := DecryptSecret(encrypted)
			if err != nil || plain != "direct-config-secret" {
				t.Fatalf("plain=%q err=%v", plain, err)
			}
		})
	}
	if err := InitMasterKeyValue("too-short"); err == nil {
		t.Fatal("invalid direct key accepted")
	}
}

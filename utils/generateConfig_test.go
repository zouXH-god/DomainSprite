package utils

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureMasterKeyGeneratesAndReusesKey(t *testing.T) {
	envName := "DOMAINSPRITE_TEST_MASTER_KEY"
	t.Setenv(envName, "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	created, keyPath, err := EnsureMasterKey(configPath, envName)
	if err != nil || !created {
		t.Fatalf("generate key: created=%v err=%v", created, err)
	}
	first := os.Getenv(envName)
	decoded, err := base64.StdEncoding.DecodeString(first)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("invalid generated key")
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("key permission = %o", info.Mode().Perm())
	}
	if err = os.Unsetenv(envName); err != nil {
		t.Fatal(err)
	}
	created, _, err = EnsureMasterKey(configPath, envName)
	if err != nil || created || os.Getenv(envName) != first {
		t.Fatalf("key was not reused: created=%v err=%v", created, err)
	}
}

func TestEnsureMasterKeyDoesNotOverwriteEnvironment(t *testing.T) {
	envName := "DOMAINSPRITE_TEST_EXTERNAL_KEY"
	external := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv(envName, external)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	created, source, err := EnsureMasterKey(configPath, envName)
	if err != nil || created || source != "environment" {
		t.Fatalf("unexpected result: %v %q %v", created, source, err)
	}
	if _, err = os.Stat(configPath + ".master-key"); !os.IsNotExist(err) {
		t.Fatal("sidecar key should not be created when environment is set")
	}
}

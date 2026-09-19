package db

import (
	"DDNSServer/models"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLegacyFastData(t *testing.T) {
	previous := DB
	t.Cleanup(func() { DB = previous })
	var err error
	DB, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = DB.AutoMigrate(&models.FastDDNSRecord{}, &models.SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	if err = InitMasterKeyValue("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "fastData.json")
	legacy := models.FastDataJson{LastId: 12, DataList: []models.FastData{{Token: "legacy-token", RecordInfo: models.RecordInfo{
		Id: "provider-1", DomainId: "zone-1", DomainName: "example.com", RecordName: "host00011",
		RecordType: "A", RecordContent: "192.0.2.10", Ttl: 600,
	}}}}
	b, _ := json.Marshal(legacy)
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}

	count, err := MigrateLegacyFastData(path, "account-a")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("imported %d records, want 1", count)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy JSON still exists: %v", err)
	}
	if _, err = os.Stat(path + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	var row models.FastDDNSRecord
	if err = DB.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.TokenHash != FastTokenHash("legacy-token") || row.TokenEncrypted == "legacy-token" {
		t.Fatal("token was not protected")
	}
	plain, err := DecryptSecret(row.TokenEncrypted)
	if err != nil || plain != "legacy-token" {
		t.Fatalf("token cannot round trip: %q %v", plain, err)
	}
	if _, err = MigrateLegacyFastData(path, "account-a"); err != nil {
		t.Fatalf("backup must not be read: %v", err)
	}
}

package db

import (
	"DDNSServer/models"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"
)

var masterKey struct {
	sync.RWMutex
	value []byte
}

func InitMasterKey(envName string) error {
	if envName == "" {
		envName = "DOMAINSPRITE_MASTER_KEY"
	}
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return fmt.Errorf("环境变量 %s 未设置", envName)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		key, err = hex.DecodeString(raw)
	}
	if err != nil || len(key) != 32 {
		return fmt.Errorf("%s 必须是 32 字节密钥的 base64 或 hex", envName)
	}
	masterKey.Lock()
	masterKey.value = append([]byte(nil), key...)
	masterKey.Unlock()
	return nil
}

func EncryptSecret(plain string) (string, error) {
	masterKey.RLock()
	key := append([]byte(nil), masterKey.value...)
	masterKey.RUnlock()
	if len(key) != 32 {
		return "", errors.New("数据库主密钥未初始化")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plain), []byte("domainsprite:v1"))
	return "v1:" + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func DecryptSecret(value string) (string, error) {
	if !strings.HasPrefix(value, "v1:") {
		return "", errors.New("未知密文版本")
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, "v1:"))
	if err != nil {
		return "", fmt.Errorf("解码密文: %w", err)
	}
	masterKey.RLock()
	key := append([]byte(nil), masterKey.value...)
	masterKey.RUnlock()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("密文损坏")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte("domainsprite:v1"))
	if err != nil {
		return "", errors.New("密文认证失败")
	}
	return string(plain), nil
}

func HashPassword(password string) (string, error) {
	if len(password) < 10 {
		return "", errors.New("密码至少需要 10 个字符")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return "argon2id$v=19$m=65536,t=3,p=2$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[3])
	expected, e2 := base64.RawStdEncoding.DecodeString(parts[4])
	if e1 != nil || e2 != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtleHashEqual(actual, expected)
}

func HashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func subtleHashEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
func RandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func ImportLegacyConfig(cfg models.Config) (bool, error) {
	imported := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var marker models.SystemSetting
		if err := tx.First(&marker, "key = ?", "migration.legacy_imported").Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Model(&models.DNSAccount{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			for _, account := range cfg.Accounts {
				id, err := EncryptSecret(account.AccessKeyId)
				if err != nil {
					return err
				}
				secret, err := EncryptSecret(account.AccessKeySecret)
				if err != nil {
					return err
				}
				tail := account.AccessKeyId
				if len(tail) > 4 {
					tail = tail[len(tail)-4:]
				}
				if err = tx.Create(&models.DNSAccount{Name: account.Name, ProviderType: account.Type, AccessKeyIDEncrypted: id, SecretEncrypted: secret, CredentialTail: tail, Enabled: true, Version: 1}).Error; err != nil {
					return err
				}
			}
		}
		var profiles int64
		tx.Model(&models.ACMEProfile{}).Count(&profiles)
		if profiles == 0 {
			for i, email := range cfg.Certificate.EmailList {
				if err := tx.Create(&models.ACMEProfile{Name: email, Email: email, CADirURL: cfg.Certificate.CADirURL, IsDefault: i == 0, Enabled: true}).Error; err != nil {
					return err
				}
			}
		}
		settings := map[string]string{"registration.open": "false", "certificate.max_request": fmt.Sprint(cfg.Certificate.MaxRequest), "certificate.concurrency": fmt.Sprint(cfg.Certificate.ConcurrencyTask), "certificate.task_timeout_minutes": fmt.Sprint(cfg.Certificate.TaskTimeoutMinutes), "certificate.task_max_retry": fmt.Sprint(cfg.Certificate.TaskMaxRetry), "certificate.apply_account": cfg.Certificate.ApplyAccount, "certificate.apply_domain_id": cfg.Certificate.ApplyDomainId, "certificate.apply_domain_name": cfg.Certificate.ApplyDomainName, "certificate.apply_prefix": cfg.Certificate.ApplyPrefix, "fast.use_account": cfg.FastConfig.UseAccount, "fast.domain_id": cfg.FastConfig.DomainId, "fast.domain_name": cfg.FastConfig.DomainName, "fast.name_strata": cfg.FastConfig.NameStrata, "fast.id_length": fmt.Sprint(cfg.FastConfig.IdLength), "fast.start_id": fmt.Sprint(cfg.FastConfig.StartId)}
		if cfg.FastConfig.AccessSalt != "" {
			encrypted, err := EncryptSecret(cfg.FastConfig.AccessSalt)
			if err != nil {
				return err
			}
			settings["fast.access_salt"] = encrypted
		}
		for key, value := range settings {
			if value == "" {
				continue
			}
			sensitive := strings.Contains(key, "salt")
			if err := tx.Where(models.SystemSetting{Key: key}).Assign(models.SystemSetting{Value: value, Sensitive: sensitive, UpdatedAt: time.Now()}).FirstOrCreate(&models.SystemSetting{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&models.SystemSetting{Key: "migration.legacy_imported", Value: "true", UpdatedAt: time.Now()}).Error; err != nil {
			return err
		}
		imported = true
		return nil
	})
	return imported, err
}

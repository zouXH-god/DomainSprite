package db

import (
	"DDNSServer/models"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"gorm.io/gorm"
)

var fastConfig atomic.Value

func StoreFastConfig(config models.FastConfig) { fastConfig.Store(config) }

func CurrentFastConfig() models.FastConfig {
	if value := fastConfig.Load(); value != nil {
		return value.(models.FastConfig)
	}
	return models.AccountConfig.FastConfig
}

func FastTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MigrateLegacyFastData imports fastData.json once and renames it only after a
// committed, count-checked transaction. A pre-existing .bak is never read.
func MigrateLegacyFastData(path, accountName string) (int, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("读取快速解析 JSON: %w", err)
	}
	var legacy models.FastDataJson
	if err = json.Unmarshal(b, &legacy); err != nil {
		return 0, fmt.Errorf("解析快速解析 JSON %s: %w", path, err)
	}
	if strings.TrimSpace(accountName) == "" && len(legacy.DataList) > 0 {
		return 0, errors.New("迁移快速解析前必须配置 DNS 账号")
	}
	bak := path + ".bak"
	if _, statErr := os.Stat(bak); statErr == nil {
		return 0, fmt.Errorf("快速解析备份已存在，无法将 %s 改名为 %s", path, bak)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return 0, statErr
	}
	inserted := 0
	err = DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range legacy.DataList {
			if item.Token == "" || item.RecordInfo.Id == "" {
				return errors.New("旧快速解析数据包含空 token 或 RecordId")
			}
			tokenEncrypted, encryptErr := EncryptSecret(item.Token)
			if encryptErr != nil {
				return encryptErr
			}
			row := models.FastDDNSRecord{
				DNSAccountName: accountName, ProviderDomainID: item.RecordInfo.DomainId,
				DomainName: item.RecordInfo.DomainName, ProviderRecordID: item.RecordInfo.Id,
				RecordName: item.RecordInfo.RecordName, RecordType: item.RecordInfo.RecordType,
				RecordContent: item.RecordInfo.RecordContent, Line: item.RecordInfo.Line,
				Status: item.RecordInfo.Status, TTL: item.RecordInfo.Ttl, DNSFrom: item.RecordInfo.DnsFrom,
				TokenHash: FastTokenHash(item.Token), TokenEncrypted: tokenEncrypted, Revision: 1,
			}
			result := tx.Where("token_hash = ? OR (dns_account_name = ? AND provider_record_id = ?)", row.TokenHash, row.DNSAccountName, row.ProviderRecordID).FirstOrCreate(&row)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				inserted++
			}
		}
		return tx.Where(models.SystemSetting{Key: "fast.next_id"}).Assign(models.SystemSetting{Value: strconv.Itoa(legacy.LastId)}).FirstOrCreate(&models.SystemSetting{}).Error
	})
	if err != nil {
		return 0, fmt.Errorf("迁移快速解析数据: %w", err)
	}
	if err = os.Rename(path, bak); err != nil {
		return 0, fmt.Errorf("快速解析迁移成功但备份改名失败: %w", err)
	}
	if dir, openErr := os.Open(filepath.Dir(path)); openErr == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return inserted, nil
}

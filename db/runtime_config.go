package db

import (
	"DDNSServer/models"
	"errors"
	"strconv"

	"gorm.io/gorm"
)

func GetDNSAccount(name string) (models.Account, error) {
	var row models.DNSAccount
	if err := DB.Where("name = ? AND enabled = ?", name, true).First(&row).Error; err != nil {
		return models.Account{}, err
	}
	id, err := DecryptSecret(row.AccessKeyIDEncrypted)
	if err != nil {
		return models.Account{}, err
	}
	secret, err := DecryptSecret(row.SecretEncrypted)
	if err != nil {
		return models.Account{}, err
	}
	return models.Account{Name: row.Name, Type: row.ProviderType, AccessKeyId: id, AccessKeySecret: secret}, nil
}
func GetDNSAccountByID(id any) (models.Account, error) {
	var row models.DNSAccount
	if err := DB.First(&row, id).Error; err != nil {
		return models.Account{}, err
	}
	return GetDNSAccount(row.Name)
}

func ListRuntimeAccounts() ([]models.Account, error) {
	var rows []models.DNSAccount
	if err := DB.Where("enabled = ?", true).Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]models.Account, 0, len(rows))
	for _, row := range rows {
		a, err := GetDNSAccount(row.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, nil
}

func setting(key string) (string, error) {
	var s models.SystemSetting
	err := DB.First(&s, "key = ?", key).Error
	return s.Value, err
}
func settingInt(key string, current int) int {
	value, err := setting(key)
	if err != nil {
		return current
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return current
	}
	return v
}

func LoadRuntimeConfig() error {
	accounts, err := ListRuntimeAccounts()
	if err != nil {
		return err
	}
	models.AccountConfig.Accounts = accounts
	var profiles []models.ACMEProfile
	if err = DB.Where("enabled = ?", true).Order("is_default desc, id").Find(&profiles).Error; err != nil {
		return err
	}
	emails := make([]string, 0, len(profiles))
	for _, p := range profiles {
		emails = append(emails, p.Email)
		if p.IsDefault && p.CADirURL != "" {
			models.AccountConfig.Certificate.CADirURL = p.CADirURL
		}
	}
	if len(emails) > 0 {
		models.AccountConfig.Certificate.EmailList = emails
	}
	models.AccountConfig.Certificate.MaxRequest = settingInt("certificate.max_request", models.AccountConfig.Certificate.MaxRequest)
	models.AccountConfig.Certificate.ConcurrencyTask = settingInt("certificate.concurrency", models.AccountConfig.Certificate.ConcurrencyTask)
	models.AccountConfig.Certificate.TaskTimeoutMinutes = settingInt("certificate.task_timeout_minutes", models.AccountConfig.Certificate.TaskTimeoutMinutes)
	models.AccountConfig.Certificate.TaskMaxRetry = settingInt("certificate.task_max_retry", models.AccountConfig.Certificate.TaskMaxRetry)
	if v, e := setting("certificate.apply_account"); e == nil {
		models.AccountConfig.Certificate.ApplyAccount = v
	}
	if v, e := setting("certificate.apply_domain_id"); e == nil {
		models.AccountConfig.Certificate.ApplyDomainId = v
	}
	if v, e := setting("certificate.apply_domain_name"); e == nil {
		models.AccountConfig.Certificate.ApplyDomainName = v
	}
	if v, e := setting("certificate.apply_prefix"); e == nil {
		models.AccountConfig.Certificate.ApplyPrefix = v
	}
	if v, e := setting("fast.use_account"); e == nil {
		models.AccountConfig.FastConfig.UseAccount = v
	}
	if v, e := setting("fast.domain_id"); e == nil {
		models.AccountConfig.FastConfig.DomainId = v
	}
	if v, e := setting("fast.domain_name"); e == nil {
		models.AccountConfig.FastConfig.DomainName = v
	}
	if v, e := setting("fast.name_strata"); e == nil {
		models.AccountConfig.FastConfig.NameStrata = v
	}
	models.AccountConfig.FastConfig.IdLength = settingInt("fast.id_length", models.AccountConfig.FastConfig.IdLength)
	models.AccountConfig.FastConfig.StartId = settingInt("fast.start_id", models.AccountConfig.FastConfig.StartId)
	if v, e := setting("fast.access_salt"); e == nil {
		plain, de := DecryptSecret(v)
		if de != nil {
			return de
		}
		models.AccountConfig.FastConfig.AccessSalt = plain
	}
	StoreFastConfig(models.AccountConfig.FastConfig)
	return nil
}

func HasUsers() (bool, error) {
	var n int64
	err := DB.Model(&models.User{}).Count(&n).Error
	return n > 0, err
}
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

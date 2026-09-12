package db

import (
	"DDNSServer/models"
	"errors"

	"gorm.io/gorm"
)

func InstallACMEStorage() {
	models.LoadACMEIdentity = func(email string) ([]byte, []byte, error) {
		var profile models.ACMEProfile
		if err := DB.Where("email = ? AND enabled = ?", email, true).First(&profile).Error; err != nil {
			return nil, nil, err
		}
		if profile.AccountKeyEncrypted == "" {
			return nil, nil, gorm.ErrRecordNotFound
		}
		key, err := DecryptSecret(profile.AccountKeyEncrypted)
		if err != nil {
			return nil, nil, err
		}
		var registration string
		if profile.RegistrationEncrypted != "" {
			registration, err = DecryptSecret(profile.RegistrationEncrypted)
			if err != nil {
				return nil, nil, err
			}
		}
		return []byte(key), []byte(registration), nil
	}
	models.SaveACMEIdentity = func(email string, keyPEM, registration []byte) error {
		updates := map[string]any{}
		if len(keyPEM) > 0 {
			value, err := EncryptSecret(string(keyPEM))
			if err != nil {
				return err
			}
			updates["account_key_encrypted"] = value
		}
		if len(registration) > 0 {
			value, err := EncryptSecret(string(registration))
			if err != nil {
				return err
			}
			updates["registration_encrypted"] = value
		}
		result := DB.Model(&models.ACMEProfile{}).Where("email = ?", email).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("ACME Profile 不存在")
		}
		return nil
	}
}

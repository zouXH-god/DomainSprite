package utils

import (
	"DDNSServer/models"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

func SanitizeLegacyConfig(path string, cfg models.Config) error {
	if len(cfg.Accounts) == 0 && cfg.BaseConfig.AccessKeyId == "" && cfg.BaseConfig.AccessKeySecret == "" && len(cfg.Certificate.EmailList) == 0 && cfg.FastConfig.AccessSalt == "" {
		return nil
	}
	type certificateInfra struct {
		SavePath string `toml:"SavePath"`
	}
	type fastInfra struct {
		DataPath string `toml:"DataPath"`
	}
	type baseInfra struct {
		Host                   string   `toml:"Host"`
		Port                   string   `toml:"Port"`
		RedisPoint             string   `toml:"RedisPoint"`
		AllowedOrigins         []string `toml:"AllowedOrigins"`
		ProviderTimeoutSeconds int      `toml:"ProviderTimeoutSeconds"`
		EncryptionKeyEnv       string   `toml:"EncryptionKeyEnv"`
		EncryptionKey          string   `toml:"EncryptionKey"`
	}
	base := baseInfra{cfg.BaseConfig.Host, cfg.BaseConfig.Port, cfg.BaseConfig.RedisPoint, cfg.BaseConfig.AllowedOrigins, cfg.BaseConfig.ProviderTimeoutSeconds, cfg.BaseConfig.EncryptionKeyEnv, cfg.BaseConfig.EncryptionKey}
	clean := struct {
		Base        baseInfra         `toml:"baseConfig"`
		Certificate certificateInfra  `toml:"certificateConfig"`
		Fast        fastInfra         `toml:"fastConfig"`
		GRPC        models.GRPCConfig `toml:"grpc"`
	}{base, certificateInfra{cfg.Certificate.SavePath}, fastInfra{cfg.FastConfig.DataPath}, cfg.GRPC}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err = toml.NewEncoder(file).Encode(clean); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	backup := path + ".pre-db-config." + time.Now().Format("20060102-150405")
	if err = os.Rename(path, backup); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("备份旧配置: %w", err)
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Rename(backup, path)
		return fmt.Errorf("写入精简配置: %w", err)
	}
	return nil
}

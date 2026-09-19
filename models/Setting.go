package models

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Account struct {
	Name            string `toml:"Name" json:"name"`
	AccessKeyId     string `toml:"AccessKeyId" json:"accessKeyId"`
	AccessKeySecret string `toml:"AccessKeySecret" json:"accessKeySecret"`
	Type            string `toml:"Type" json:"type"`
}
type FastConfig struct {
	UseAccount string `toml:"UseAccount" json:"useAccount"`
	DomainId   string `toml:"DomainId" json:"domainId"`
	DomainName string `toml:"DomainName" json:"domainName"`
	NameStrata string `toml:"NameStrata" json:"nameStrata"`
	IdLength   int    `toml:"IdLength" json:"idLength"`
	StartId    int    `toml:"StartId" json:"startId"`
	DataPath   string `toml:"DataPath" json:"dataPath"`
	AccessSalt string `toml:"AccessSalt" json:"accessSalt"`
}
type BaseConfig struct {
	Host                   string   `toml:"Host"`
	Port                   string   `toml:"Port"`
	AccessKeyId            string   `toml:"AccessKeyId"`
	AccessKeySecret        string   `toml:"AccessKeySecret"`
	RedisPoint             string   `toml:"RedisPoint" json:"redisPoint"`
	AllowedOrigins         []string `toml:"AllowedOrigins" json:"allowedOrigins"`
	ProviderTimeoutSeconds int      `toml:"ProviderTimeoutSeconds"`
	EncryptionKeyEnv       string   `toml:"EncryptionKeyEnv" json:"-"`
	EncryptionKey          string   `toml:"EncryptionKey" json:"-"`
}
type GRPCConfig struct {
	Enabled          bool   `toml:"Enabled"`
	Host             string `toml:"Host"`
	Port             string `toml:"Port"`
	TLSCertFile      string `toml:"TLSCertFile"`
	TLSKeyFile       string `toml:"TLSKeyFile"`
	PublicURL        string `toml:"PublicURL"`
	HeartbeatSeconds int    `toml:"HeartbeatSeconds"`
	MaxArtifactBytes int64  `toml:"MaxArtifactBytes"`
	EmbeddedNode     bool   `toml:"EmbeddedNode"`
}
type CertificateConfig struct {
	EmailList          []string `toml:"EmailList"`
	MaxRequest         int      `toml:"MaxRequest"`
	SavePath           string   `toml:"SavePath"`
	ApplyAccount       string   `toml:"ApplyAccount"`
	ApplyDomainId      string   `toml:"ApplyDomainId"`
	ApplyDomainName    string   `toml:"ApplyDomainName"`
	ApplyPrefix        string   `toml:"ApplyPrefix"`
	ConcurrencyTask    int      `toml:"ConcurrencyTask"`
	TaskTimeoutMinutes int      `toml:"TaskTimeoutMinutes"`
	TaskMaxRetry       int      `toml:"TaskMaxRetry"`
	LegacyFastSunset   string   `toml:"LegacyFastSunset"`
	CADirURL           string   `toml:"CADirURL"`
}
type Config struct {
	BaseConfig  BaseConfig        `toml:"baseConfig" json:"baseConfig"`
	Certificate CertificateConfig `toml:"certificateConfig" json:"certificateConfig"`
	FastConfig  FastConfig        `toml:"fastConfig" json:"fastConfig"`
	Accounts    []Account         `toml:"account" json:"account"`
	GRPC        GRPCConfig        `toml:"grpc" json:"grpc"`
}

var AccountConfig Config

func LoadConfig(path string) error {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return fmt.Errorf("读取配置: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	AccountConfig = cfg
	return nil
}

func (c *Config) Validate() error {
	host := strings.TrimSpace(c.BaseConfig.Host)
	if host == "" || (net.ParseIP(host) == nil && host != "localhost") {
		return errors.New("baseConfig.Host 无效")
	}
	p, err := strconv.Atoi(c.BaseConfig.Port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("baseConfig.Port 无效")
	}
	if c.BaseConfig.RedisPoint == "" {
		return errors.New("RedisPoint 不能为空")
	}
	if c.Certificate.SavePath == "" {
		return errors.New("证书保存路径必须配置")
	}
	if c.Certificate.ConcurrencyTask < 1 {
		c.Certificate.ConcurrencyTask = 1
	}
	if c.Certificate.MaxRequest < 1 {
		c.Certificate.MaxRequest = 20
	}
	if c.Certificate.TaskTimeoutMinutes < 1 {
		c.Certificate.TaskTimeoutMinutes = 30
	}
	if c.Certificate.TaskMaxRetry < 0 {
		return errors.New("TaskMaxRetry 不能为负数")
	}
	if c.Certificate.TaskMaxRetry == 0 {
		c.Certificate.TaskMaxRetry = 3
	}
	if c.BaseConfig.ProviderTimeoutSeconds < 1 {
		c.BaseConfig.ProviderTimeoutSeconds = 30
	}
	for _, origin := range c.BaseConfig.AllowedOrigins {
		if origin == "*" {
			return errors.New("启用 Cookie 鉴权时 AllowedOrigins 不能包含通配符")
		}
	}
	if c.FastConfig.DataPath == "" {
		return errors.New("fastConfig.DataPath 不能为空")
	}
	if c.FastConfig.IdLength < 1 {
		c.FastConfig.IdLength = 5
	}
	names := map[string]bool{}
	for _, a := range c.Accounts {
		if a.Name == "" || a.AccessKeyId == "" || a.AccessKeySecret == "" {
			return errors.New("账户名称和凭据不能为空")
		}
		if names[a.Name] {
			return fmt.Errorf("账户名称重复: %s", a.Name)
		}
		names[a.Name] = true
		if a.Type != "Ali" && a.Type != "Tencent" && a.Type != "Cloudflare" {
			return fmt.Errorf("账户 %s 的类型无效", a.Name)
		}
	}
	if c.Certificate.ApplyAccount != "" && !names[c.Certificate.ApplyAccount] {
		return errors.New("ApplyAccount 引用了不存在的账户")
	}
	if c.FastConfig.UseAccount != "" && !names[c.FastConfig.UseAccount] {
		return errors.New("UseAccount 引用了不存在的账户")
	}
	if c.BaseConfig.EncryptionKey == "" && c.BaseConfig.EncryptionKeyEnv == "" {
		c.BaseConfig.EncryptionKeyEnv = "DOMAINSPRITE_MASTER_KEY"
	}
	if c.GRPC.Port == "" {
		c.GRPC.Port = "2486"
	}
	if c.GRPC.HeartbeatSeconds < 1 {
		c.GRPC.HeartbeatSeconds = 10
	}
	if c.GRPC.MaxArtifactBytes < 1 {
		c.GRPC.MaxArtifactBytes = 10 << 20
	}
	return nil
}

func InitDataDirectories() error {
	for _, p := range []string{AccountConfig.FastConfig.DataPath, AccountConfig.Certificate.SavePath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", p, err)
		}
	}
	return nil
}
func ProviderTimeout() time.Duration {
	return time.Duration(AccountConfig.BaseConfig.ProviderTimeoutSeconds) * time.Second
}

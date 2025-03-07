package models

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
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
	Host            string `toml:"Host"`
	Port            string `toml:"Port"`
	AccessKeyId     string `toml:"AccessKeyId"`
	AccessKeySecret string `toml:"AccessKeySecret"`
}

type CertificateConfig struct {
	EmailList       []string `toml:"EmailList"`
	MaxRequest      int      `toml:"MaxRequest"`
	SavePath        string   `toml:"SavePath"`
	ApplyAccount    string   `toml:"ApplyAccount"`
	ApplyDomainId   string   `toml:"ApplyDomainId"`
	ApplyDomainName string   `toml:"ApplyDomainName"`
}

type Config struct {
	BaseConfig  BaseConfig        `toml:"baseConfig" json:"baseConfig"`
	Certificate CertificateConfig `toml:"certificateConfig" json:"certificateConfig"`
	FastConfig  FastConfig        `toml:"fastConfig" json:"fastConfig"`
	Accounts    []Account         `toml:"account" json:"account"`
}

var AccountConfig Config

func init() {
	if _, err := toml.DecodeFile("config.toml", &AccountConfig); err != nil {
		fmt.Println("Error decoding TOML:", err)
		return
	}
	// 使用 os.Stat 检查文件夹是否存在
	_, err := os.Stat(AccountConfig.FastConfig.DataPath)
	if os.IsNotExist(err) {
		// 文件夹不存在，创建它
		err = os.MkdirAll(AccountConfig.FastConfig.DataPath, 0755)
		if err != nil {
			fmt.Println("Error creating folder:", err)
			return
		}
	}
}

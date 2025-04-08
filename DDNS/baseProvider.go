package DDNS

import (
	"DDNSServer/DDNS/providers/ali"
	"DDNSServer/DDNS/providers/cloudflare"
	"DDNSServer/DDNS/providers/tencent"
	"DDNSServer/models"
	"errors"
)

func NewBaseProvider(info models.Account) (models.RecordProvider, error) {
	switch info.Type {
	case "Tencent":
		return tencent.NewTencentProvider(info, info.AccessKeyId, info.AccessKeySecret)
	case "Cloudflare":
		return cloudflare.NewCloudflareProvider(info, info.AccessKeyId, info.AccessKeySecret)
	case "Ali":
		return ali.NewAliDNSClient(info, info.AccessKeyId, info.AccessKeySecret)
	default:
		return nil, nil
	}
}

func GetAccount(AccountName string) (models.Account, error) {
	for _, account := range models.AccountConfig.Accounts {
		if account.Name == AccountName {
			return account, nil
		}
	}
	return models.Account{}, errors.New("account not found")
}

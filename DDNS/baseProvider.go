package DDNS

import (
	"DDNSServer/DDNS/providers/ali"
	"DDNSServer/DDNS/providers/cloudflare"
	"DDNSServer/DDNS/providers/tencent"
	"DDNSServer/db"
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
		return nil, errors.New("unsupported DNS provider: " + info.Type)
	}
}

func GetAccount(AccountName string) (models.Account, error) {
	account, err := db.GetDNSAccount(AccountName)
	if err != nil {
		return models.Account{}, errors.New("account not found")
	}
	return account, nil
}

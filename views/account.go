package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"github.com/gin-gonic/gin"
)

func GetAccounts(c *gin.Context) {
	var accountDatas []models.Account
	accounts, err := db.ListRuntimeAccounts()
	if err != nil {
		requestModel.Error(c, 500, "查询 DNS 账号失败", nil)
		return
	}
	for _, account := range accounts {
		if !canUseAccount(c, account.Name, 1) {
			continue
		}
		accountData := models.Account{
			AccessKeyId:     maskCredential(account.AccessKeyId),
			AccessKeySecret: "********",
			Name:            account.Name,
			Type:            account.Type,
		}
		accountDatas = append(accountDatas, accountData)
	}
	requestModel.Success(c, accountDatas)
}

func maskCredential(value string) string {
	if len(value) <= 4 {
		return "********"
	}
	return value[:4] + "********"
}

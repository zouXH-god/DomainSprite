package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"github.com/gin-gonic/gin"
)

func GetAccounts(c *gin.Context) {
	var accountDatas []models.Account
	for _, account := range models.AccountConfig.Accounts {
		accountData := models.Account{
			AccessKeyId:     account.AccessKeyId[:4] + "*********",
			AccessKeySecret: account.AccessKeySecret[:4] + "*********",
			Name:            account.Name,
			Type:            account.Type,
		}
		accountDatas = append(accountDatas, accountData)
	}
	requestModel.Success(c, accountDatas)
}

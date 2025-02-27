package views

import (
	"DDNSServer/models"
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
	c.JSON(200, gin.H{
		"data": accountDatas,
	})
}

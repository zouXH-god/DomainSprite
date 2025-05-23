package views

import (
	"DDNSServer/DDNS"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"github.com/gin-gonic/gin"
)

func getProvider(c *gin.Context) (models.RecordProvider, error) {
	accountName := c.Params.ByName("accountName")
	provider, err := getProviderForAccountName(accountName)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return nil, err
	}
	return provider, nil
}

func getProviderForAccountName(accountName string) (models.RecordProvider, error) {

	account, err := DDNS.GetAccount(accountName)
	if err != nil {
		return nil, err
	}
	provider, err := DDNS.NewBaseProvider(account)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// GetDomains 获取指定账号的域名列表
func GetDomains(c *gin.Context) {
	domainsSearch := models.DomainsSearch{}
	err := c.Bind(&domainsSearch)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}

	provider, err := getProvider(c)
	if err != nil {
		return
	}
	domainList, err := provider.GetDomainList(domainsSearch)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, domainList)
}

// GetRecords 获取指定域名的解析记录
func GetRecords(c *gin.Context) {
	recordSearch := models.DNSSearch{}
	err := c.Bind(&recordSearch)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	recordList, err := provider.GetRecordList(recordSearch)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, recordList)
}

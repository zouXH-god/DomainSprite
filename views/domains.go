package views

import (
	"DDNSServer/DDNS"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"errors"
	"github.com/gin-gonic/gin"
)

func getProvider(c *gin.Context) (models.RecordProvider, error) {
	accountName := c.Params.ByName("accountName")
	required := 1
	if c.Request.Method != "GET" {
		required = 2
	}
	if !canUseAccount(c, accountName, required) {
		requestModel.Forbidden(c, "无权访问该 DNS 账号")
		return nil, errors.New("forbidden")
	}
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
	err := c.ShouldBind(&domainsSearch)
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
	filtered := domainList.Domains[:0]
	for _, domain := range domainList.Domains {
		if canViewZone(c, c.Param("accountName"), domain.DomainName) {
			filtered = append(filtered, domain)
		}
	}
	domainList.Domains = filtered
	requestModel.Success(c, domainList)
}

// GetRecords 获取指定域名的解析记录
func GetRecords(c *gin.Context) {
	recordSearch := models.DNSSearch{}
	err := c.ShouldBind(&recordSearch)
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
	filtered := recordList.Records[:0]
	for _, record := range recordList.Records {
		if canUseDomain(c, c.Param("accountName"), recordFQDN(record.RecordName, record.DomainName), 1) {
			filtered = append(filtered, record)
		}
	}
	recordList.Records = filtered
	recordList.TotalCount = int64(len(filtered))
	requestModel.Success(c, recordList)
}

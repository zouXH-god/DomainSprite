package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"DDNSServer/utils"
	"fmt"
	"github.com/gin-gonic/gin"
)

const (
	fastDataFile = "fastData.json"
)

var FastData models.FastDataJson
var fastDataPath string

func init() {
	fastDataPath = models.AccountConfig.FastConfig.DataPath + fastDataFile
	var err error
	FastData, err = models.GetFastData(fastDataPath)
	if err != nil {
		fmt.Println("Error Load FastData:", err)
		return
	}
}

func getDomainRR(provider models.RecordProvider) string {
	// 拼接出域名
	id := fmt.Sprintf("%0*d", models.AccountConfig.FastConfig.IdLength, FastData.LastId)
	domainRR := fmt.Sprintf("%s%s", models.AccountConfig.FastConfig.NameStrata, id)
	// 判断这个解析是否存在
	list, err := provider.GetRecordList(models.DNSSearch{
		DomainId:   models.AccountConfig.FastConfig.DomainId,
		DomainName: models.AccountConfig.FastConfig.DomainName,
		KeyWord:    domainRR,
	})
	if err != nil {
		fmt.Print("Error GetRecordList:", err)
		return ""
	}
	if len(list.Records) > 0 {
		// 存在递增继续拼接
		FastData.LastId++
		return getDomainRR(provider)
	}
	return domainRR
}

// IpToDomainRecord 获取IP对应的域名记录
func IpToDomainRecord(c *gin.Context) {
	host := c.RemoteIP()
	// 判断当前ip是否已经拥有记录
	if fastData, ok := FastData.GetInfoForIp(host); ok {
		requestModel.Success(c, fastData)
		return
	}
	provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 拼接出域名
	domainRR := getDomainRR(provider)
	if domainRR == "" {
		requestModel.BadRequest(c, "Error Get DomainRR")
		return
	}
	// 新增解析
	recordInfo := models.RecordInfo{
		DomainId:      models.AccountConfig.FastConfig.DomainId,
		DomainName:    models.AccountConfig.FastConfig.DomainName,
		RecordName:    domainRR,
		RecordType:    "A",
		RecordContent: host,
	}
	recordInfo, err = provider.AddRecord(recordInfo)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 创建Token
	Token := utils.HashStringWithCurrentTime(domainRR + recordInfo.DomainName + models.AccountConfig.FastConfig.AccessSalt)
	// 保存这条记录
	fastData := models.FastData{
		RecordInfo: recordInfo,
		Token:      Token,
	}
	FastData.DataList = append(FastData.DataList, fastData)
	err = FastData.SaveToJson(fastDataPath)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 返回记录和Token
	requestModel.Success(c, fastData)
}

// UpdateForToken 更新Token对应的记录
func UpdateForToken(c *gin.Context) {
	token := c.Query("token")
	host := c.RemoteIP()
	fastData, exist := FastData.GetInfoForToken(token)
	if !exist {
		requestModel.BadRequest(c, "Token Not Exist")
		return
	}
	// 判断当前解析记录是否一致
	if fastData.RecordInfo.RecordContent == host {
		requestModel.Success(c, fastData)
		return
	} else {
		// 获取快速解析账号
		provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		// 修改解析
		fastData.RecordInfo.RecordContent = host
		fastData.RecordInfo, err = provider.UpdateRecord(fastData.RecordInfo)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		// 更新记录信息
		err = FastData.SaveToJson(fastDataPath)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		requestModel.Success(c, fastData)
	}
}

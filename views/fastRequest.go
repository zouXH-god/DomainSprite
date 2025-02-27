package views

import (
	"DDNSServer/models"
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
	if len(list) > 0 {
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
		c.JSON(200, gin.H{
			"message": "record is exist",
			"data":    fastData,
		})
		return
	}
	provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	// 拼接出域名
	domainRR := getDomainRR(provider)
	if domainRR == "" {
		c.JSON(400, gin.H{
			"message": "Error Get DomainRR",
		})
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
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
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
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	// 返回记录和Token
	c.JSON(200, gin.H{
		"message": "success",
		"data":    fastData,
	})
}

// UpdateForToken 更新Token对应的记录
func UpdateForToken(c *gin.Context) {
	token := c.Query("token")
	host := c.RemoteIP()
	fastData, exist := FastData.GetInfoForToken(token)
	if !exist {
		c.JSON(400, gin.H{
			"message": "Token Not Exist",
		})
		return
	}
	if fastData.RecordInfo.RecordContent == host {
		c.JSON(200, gin.H{
			"message": "host is same",
			"data":    fastData,
		})
		return
	} else {
		provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
		if err != nil {
			c.JSON(400, gin.H{
				"message": err.Error(),
			})
			return
		}
		fastData.RecordInfo.RecordContent = host
		fastData.RecordInfo, err = provider.UpdateRecord(fastData.RecordInfo)
		if err != nil {
			c.JSON(400, gin.H{
				"message": err.Error(),
			})
			return
		}
		err = FastData.SaveToJson(fastDataPath)
		if err != nil {
			c.JSON(400, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "success",
			"data":    fastData,
		})
	}
}

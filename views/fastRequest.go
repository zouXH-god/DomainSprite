package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

var fastStore *models.FastStore

func InitFastStore() error {
	s, err := models.NewFastStore(filepath.Join(models.AccountConfig.FastConfig.DataPath, "fastData.json"), models.AccountConfig.FastConfig.StartId)
	if err != nil {
		return err
	}
	fastStore = s
	return nil
}
func deprecatedFast(c *gin.Context) {
	c.Header("Deprecation", "true")
	if v := models.AccountConfig.Certificate.LegacyFastSunset; v != "" {
		c.Header("Sunset", v)
	}
	c.Header("Link", "</fast/record>; rel=\"successor-version\"")
}

func IpToDomainRecord(c *gin.Context) {
	if fastStore == nil {
		requestModel.Error(c, 500, "快速 DDNS 未初始化", nil)
		return
	}
	host := c.RemoteIP()
	if d, ok := fastStore.FindIP(host); ok {
		requestModel.Success(c, d)
		return
	}
	provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	var result models.FastData
	err = fastStore.WithWrite(func(data *models.FastDataJson) error {
		for _, d := range data.DataList {
			if d.RecordInfo.RecordContent == host {
				result = d
				return nil
			}
		}
		var name string
		for {
			name = fmt.Sprintf("%s%0*d", models.AccountConfig.FastConfig.NameStrata, models.AccountConfig.FastConfig.IdLength, data.LastId)
			list, e := provider.GetRecordList(models.DNSSearch{DomainId: models.AccountConfig.FastConfig.DomainId, DomainName: models.AccountConfig.FastConfig.DomainName, KeyWord: name})
			if e != nil {
				return e
			}
			data.LastId++
			if len(list.Records) == 0 {
				break
			}
		}
		record, e := provider.AddRecord(models.RecordInfo{DomainId: models.AccountConfig.FastConfig.DomainId, DomainName: models.AccountConfig.FastConfig.DomainName, RecordName: name, RecordType: "A", RecordContent: host})
		if e != nil {
			return e
		}
		token, e := models.NewFastToken()
		if e != nil {
			_, _ = provider.DeleteRecord(record.DomainName, record.Id)
			return e
		}
		result = models.FastData{RecordInfo: record, Token: token}
		data.DataList = append(data.DataList, result)
		return nil
	})
	if err != nil {
		if result.RecordInfo.Id != "" {
			if _, cleanupErr := provider.DeleteRecord(result.RecordInfo.DomainName, result.RecordInfo.Id); cleanupErr != nil {
				slog.Error("快速 DDNS 本地保存失败且云端补偿失败", "error", cleanupErr)
			}
		}
		requestModel.Error(c, 502, err.Error(), nil)
		return
	}
	requestModel.Success(c, result)
}

type fastUpdateRequest struct {
	Token string `json:"token" form:"token" binding:"required"`
}

func UpdateForToken(c *gin.Context) {
	if fastStore == nil {
		requestModel.Error(c, 500, "快速 DDNS 未初始化", nil)
		return
	}
	var req fastUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	host := c.RemoteIP()
	provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	var result models.FastData
	err = fastStore.WithWrite(func(data *models.FastDataJson) error {
		for i := range data.DataList {
			d := &data.DataList[i]
			found, _ := fastStoreTokenEqual(d.Token, req.Token)
			if found {
				if d.RecordInfo.RecordContent != host {
					old := d.RecordInfo.RecordContent
					d.RecordInfo.RecordContent = host
					updated, e := provider.UpdateRecord(d.RecordInfo)
					if e != nil {
						d.RecordInfo.RecordContent = old
						return e
					}
					d.RecordInfo = updated
				}
				result = *d
				return nil
			}
		}
		return errors.New("Token Not Exist")
	})
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, result)
}

func fastStoreTokenEqual(a, b string) (bool, error) {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1, nil
}
func LegacyIpToDomainRecord(c *gin.Context) { deprecatedFast(c); IpToDomainRecord(c) }
func LegacyUpdateForToken(c *gin.Context) {
	deprecatedFast(c)
	if c.Query("token") != "" {
		c.Request.Form = map[string][]string{"token": {c.Query("token")}}
	}
	UpdateForToken(c)
}

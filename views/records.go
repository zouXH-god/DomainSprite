package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"github.com/gin-gonic/gin"
)

// GetRecordInfo 获取指定域名的解析记录信息
func GetRecordInfo(c *gin.Context) {
	domainName := c.Query("domainName")
	recordId := c.Query("recordId")

	provider, err := getProvider(c)
	if err != nil {
		return
	}
	recordInfo, err := provider.GetRecordInfo(domainName, recordId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, recordInfo)
}

// AddRecord 添加解析记录
func AddRecord(c *gin.Context) {
	recordInfo := models.RecordInfo{}
	err := c.Bind(&recordInfo)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	recordInfo, err = provider.AddRecord(recordInfo)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, recordInfo)
}

// UpdateRecord 更新解析记录
func UpdateRecord(c *gin.Context) {
	recordInfo := models.RecordInfo{}
	err := c.Bind(&recordInfo)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	recordInfo, err = provider.UpdateRecord(recordInfo)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, recordInfo)
}

// DeleteRecord 删除解析记录
func DeleteRecord(c *gin.Context) {
	domainName := c.Query("domainName")
	recordId := c.Query("recordId")

	provider, err := getProvider(c)
	if err != nil {
		return
	}
	_, err = provider.DeleteRecord(domainName, recordId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, "ok")
}

// SetRecordStatus 设置解析记录状态
func SetRecordStatus(c *gin.Context) {
	domainName := c.Query("domainName")
	recordId := c.Query("recordId")
	status := c.Query("status")

	provider, err := getProvider(c)
	if err != nil {
		return
	}
	_, err = provider.SetRecordStatus(domainName, recordId, status)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, "ok")
}

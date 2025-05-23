package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"DDNSServer/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"strings"
)

type CnameInfo struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Domain         string `json:"domain"`
	FullDomainName string `json:"fullDomainName"`
	Value          string `json:"value"`
}

func analyzeDomainForId(domainId string, domainList *[]models.Domains, domainInfoList *[]models.DomainInfo) error {
	domain, err := db.GetDomainForId(domainId)
	if err != nil {
		return err
	}
	*domainList = append(*domainList, domain)
	domainInfo := db.DomainToDomainInfo(domain)
	*domainInfoList = append(*domainInfoList, domainInfo)
	return nil
}

func GetCnameInfoForDomain(domainNameList []string) (cnameInfoList []CnameInfo) {
	name := "_acme-challenge"
	for _, domainName := range domainNameList {
		fullDomainName := name + "." + domainName
		rr := utils.HashString(domainName)
		value := rr + "." + models.AccountConfig.Certificate.ApplyDomainName
		cnameInfo := CnameInfo{
			Name:           name,
			Type:           "cname",
			Domain:         domainName,
			FullDomainName: fullDomainName,
			Value:          value,
		}
		cnameInfoList = append(cnameInfoList, cnameInfo)
	}
	return cnameInfoList
}

// CreateCertificateView 申请证书(基于数据库一键申请)
func CreateCertificateView(c *gin.Context) {
	// 绑定参数
	var request requestModel.CreateCertificateRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	domainIdList := strings.Split(request.DomainIdList, ",")
	// 获取域名信息
	var domainInfoList []models.DomainInfo
	var domainList []models.Domains
	var err error
	if len(domainIdList) > 0 && domainIdList[0] != "" {
		for _, dId := range domainIdList {
			err = analyzeDomainForId(dId, &domainList, &domainInfoList)
			if err != nil {
				break
			}
		}
	} else if request.DomainId != "" {
		err = analyzeDomainForId(request.DomainId, &domainList, &domainInfoList)
	} else {
		requestModel.BadRequest(c, "域名ID不能为空")
		return
	}
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	// 申请证书
	// 创建空白证书记录
	taskId := uuid.New().String()
	certificateInfo := models.Certificate{State: "wait", TaskId: taskId}
	err = db.DB.Model(&models.Certificate{}).Create(&certificateInfo).Error
	if err != nil {
		requestModel.BadRequest(c, "证书记录创建失败："+err.Error())
		return
	}
	// 创建任务
	_, err = certificate.NewCertificateCreateTask(provider, domainInfoList, certificateInfo, taskId)
	if err != nil {
		return
	}
	for _, domain := range domainList {
		domain.CertificateId = certificateInfo.Id
		err = db.UpdateDomain(domain)
		if err != nil {
			slog.Log(c, slog.LevelError, "更新域名信息失败", "domain", domain.DomainName)
		}
	}
	requestModel.Success(c, gin.H{
		"taskId":      taskId,
		"certificate": certificateInfo,
	})
}

// GetCertificateListView 获取证书列表
func GetCertificateListView(c *gin.Context) {
	// 绑定参数
	var request requestModel.GetCertificateListRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	certificateList, err := db.GetCertificateList(request.Page, request.PageSize)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, certificateList)
}

// GetCertificateViewWithDomainInfo 基于域名信息获取需要解析的内容
func GetCertificateViewWithDomainInfo(c *gin.Context) {
	// 绑定参数
	var request requestModel.DomainNameListRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	domainNameList := strings.Split(request.DomainNameList, ",")
	cnameInfoList := GetCnameInfoForDomain(domainNameList)
	requestModel.Success(c, cnameInfoList)
}

// CreateCertificateViewWithDomainInfo 申请证书(基于域名信息)
func CreateCertificateViewWithDomainInfo(c *gin.Context) {
	// 绑定参数
	var request requestModel.DomainNameListRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	domainNameList := strings.Split(request.DomainNameList, ",")
	cnameInfoList := GetCnameInfoForDomain(domainNameList)
	// 判断是否所有域名CNAME解析到指定域名
	var errorCnameInfo []CnameInfo
	for _, cnameInfo := range cnameInfoList {
		if !utils.IsCNAMEEqual(cnameInfo.FullDomainName, cnameInfo.Value) {
			errorCnameInfo = append(errorCnameInfo, cnameInfo)
		}
	}
	if len(errorCnameInfo) > 0 {
		requestModel.BadRequestWithData(c, "请检查CNAME解析是否正确", errorCnameInfo)
		return
	}
	// 申请证书
	var domainInfoList []models.DomainInfo
	for _, domainName := range domainNameList {
		domainInfo := models.DomainInfo{}
		domainInfo.DomainName = domainName
		domainInfoList = append(domainInfoList, domainInfo)
	}
	// 获取账号信息
	provider, err := getProviderForAccountName(models.AccountConfig.Certificate.ApplyAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 创建空白证书记录
	taskId := uuid.New().String()
	certificateInfo := models.Certificate{State: "wait", TaskId: taskId}
	err = db.DB.Model(&models.Certificate{}).Create(&certificateInfo).Error
	if err != nil {
		requestModel.BadRequest(c, "证书记录创建失败："+err.Error())
		return
	}
	// 创建任务
	_, err = certificate.NewCertificateCreateTask(provider, domainInfoList, certificateInfo, taskId)
	if err != nil {
		return
	}
	requestModel.Success(c, gin.H{
		"taskId":      taskId,
		"certificate": certificateInfo,
	})
}

// GetCertificateViewWithId 根据id查询证书消息
func GetCertificateViewWithId(c *gin.Context) {
	// 绑定参数
	var request requestModel.CertificateIdRequest
	if err := c.Bind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	certificateDB, err := db.GetCertificateForId(request.CertificateId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, certificateDB)
}

// DownloadCertificateViewWithId 根据id下载证书
func DownloadCertificateViewWithId(c *gin.Context) {
	// 绑定参数
	var request requestModel.DownloadCertificateViewWithIdRequest
	if err := c.Bind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	certificateDB, err := db.GetCertificateForId(request.CertificateId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	certificatePrivate := models.CertificatePrivate{SavePath: certificateDB.SavePath}
	// 读取证书信息
	resource, err := certificatePrivate.LoadResource()
	if err != nil {
		requestModel.BadRequest(c, "证书历史读取失败："+err.Error())
		return
	}
	switch request.DownloadType {
	case "cert":
		c.File(resource.CertificatePath)
	case "key":
		c.File(resource.PrivateKeyPath)
	case "all":
		zipPath, err := utils.ZipFolder(resource.SavePath)
		if err != nil {
			requestModel.BadRequest(c, "证书压缩失败："+err.Error())
			return
		}
		c.File(zipPath)
	}
	return
}

// GetCertificateTaskInfoByCertificateId 根据证书id查询证书任务信息
func GetCertificateTaskInfoByCertificateId(c *gin.Context) {
	// 绑定参数
	var request requestModel.CertificateIdRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 查询证书
	certificateInfo, err := db.GetCertificateForId(request.CertificateId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 查询任务
	taskList, err := db.GetTaskInfoList(certificateInfo.TaskId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, taskList)
}

// GetTaskLog 根据任务id查询任务日志
func GetTaskLog(c *gin.Context) {
	// 绑定参数
	var request requestModel.TaskIdRequest
	if err := c.Bind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 查询任务信息
	taskInfo, err := db.GetTaskInfoById(request.Id)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 获取任务日志具体信息
	taskLog, err := utils.ReadFileContent(taskInfo.LogPath)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, taskLog)
}

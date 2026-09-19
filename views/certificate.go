package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"DDNSServer/utils"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CnameInfo struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Domain         string `json:"domain"`
	FullDomainName string `json:"fullDomainName"`
	Value          string `json:"value"`
}

func analyzeDomainForId(accountName, domainId string, domainList *[]models.Domains, domainInfoList *[]models.DomainInfo) error {
	domain, err := db.GetDomainForId(accountName, domainId)
	if err != nil {
		return err
	}
	*domainList = append(*domainList, domain)
	domainInfo := db.DomainToDomainInfo(domain)
	*domainInfoList = append(*domainInfoList, domainInfo)
	return nil
}

func GetCnameInfoForDomain(domainNameList []string) ([]CnameInfo, error) {
	applyDomain := strings.TrimSuffix(strings.TrimSpace(models.AccountConfig.Certificate.ApplyDomainName), ".")
	if strings.TrimSpace(models.AccountConfig.Certificate.ApplyAccount) == "" || strings.TrimSpace(models.AccountConfig.Certificate.ApplyDomainId) == "" || applyDomain == "" {
		return nil, errors.New("CNAME 委托承载域名尚未配置，请管理员在设置中选择 DNS 账号和承载域名")
	}
	if _, err := certificate.NormalizeDomain(applyDomain); err != nil {
		return nil, fmt.Errorf("CNAME 委托承载域名配置无效: %w", err)
	}
	cnameInfoList := make([]CnameInfo, 0, len(domainNameList))
	name := "_acme-challenge"
	for _, domainName := range domainNameList {
		fullDomainName := name + "." + domainName
		rr := certificate.DelegatedRecordName(domainName, models.AccountConfig.Certificate.ApplyPrefix)
		value := rr + "." + applyDomain + "."
		cnameInfo := CnameInfo{
			Name:           name,
			Type:           "cname",
			Domain:         domainName,
			FullDomainName: fullDomainName,
			Value:          value,
		}
		cnameInfoList = append(cnameInfoList, cnameInfo)
	}
	return cnameInfoList, nil
}

// CreateCertificateView 申请证书(基于数据库一键申请)
func CreateCertificateView(c *gin.Context) {
	// 绑定参数
	var request requestModel.CreateCertificateRequest
	if err := c.ShouldBind(&request); err != nil {
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
			err = analyzeDomainForId(c.Param("accountName"), dId, &domainList, &domainInfoList)
			if err != nil {
				break
			}
		}
	} else if request.DomainId != "" {
		err = analyzeDomainForId(c.Param("accountName"), request.DomainId, &domainList, &domainInfoList)
	} else {
		requestModel.BadRequest(c, "域名ID不能为空")
		return
	}
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	for _, domain := range domainList {
		if !canUseDomain(c, domain.AccountName, domain.DomainName, 3) {
			requestModel.Forbidden(c, "无权为域名申请证书: "+domain.DomainName)
			return
		}
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	// 申请证书
	// 创建空白证书记录
	_ = provider
	certificateInfo, err := certificate.EnqueueCertificate(c.Param("accountName"), domainInfoList, domainList)
	if err != nil {
		if errors.Is(err, certificate.ErrQueueUnavailable) {
			requestModel.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		} else {
			requestModel.BadRequest(c, err.Error())
		}
		return
	}
	requestModel.Success(c, gin.H{
		"taskId":      certificateInfo.TaskId,
		"certificate": certificateInfo,
	})
}

// GetCertificateListView 获取证书列表
func GetCertificateListView(c *gin.Context) {
	// 绑定参数
	var request requestModel.GetCertificateListRequest
	if err := c.ShouldBind(&request); err != nil {
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
	if err := c.ShouldBind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	domainNameList := strings.Split(request.DomainNameList, ",")
	for i, raw := range domainNameList {
		name, err := certificate.NormalizeDomain(raw)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		domainNameList[i] = name
	}
	cnameInfoList, err := GetCnameInfoForDomain(domainNameList)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, cnameInfoList)
}

// CreateCertificateViewWithDomainInfo 申请证书(基于域名信息)
func CreateCertificateViewWithDomainInfo(c *gin.Context) {
	// 绑定参数
	var request requestModel.DomainNameListRequest
	if err := c.ShouldBind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	rawNames := strings.Split(request.DomainNameList, ",")
	domainNameList := make([]string, 0, len(rawNames))
	for _, raw := range rawNames {
		name, e := certificate.NormalizeDomain(raw)
		if e != nil {
			requestModel.BadRequest(c, e.Error())
			return
		}
		domainNameList = append(domainNameList, name)
	}
	cnameInfoList, err := GetCnameInfoForDomain(domainNameList)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
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
	var bindings []models.CertificateDomain
	for _, domainName := range domainNameList {
		if !canUseDomain(c, models.AccountConfig.Certificate.ApplyAccount, domainName, 3) {
			requestModel.Forbidden(c, "无权为域名申请证书: "+domainName)
			return
		}
		bindings = append(bindings, models.CertificateDomain{DomainName: domainName, AccountName: models.AccountConfig.Certificate.ApplyAccount, ProviderType: "delegated", ProviderDomainID: models.AccountConfig.Certificate.ApplyDomainId, ChallengeMode: "delegated"})
	}
	// 获取账号信息
	provider, err := getProviderForAccountName(models.AccountConfig.Certificate.ApplyAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 创建空白证书记录
	_ = provider
	certificateInfo, err := certificate.EnqueueCertificateDomains(bindings, 0, "issue")
	if err != nil {
		if errors.Is(err, certificate.ErrQueueUnavailable) {
			requestModel.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		} else {
			requestModel.BadRequest(c, err.Error())
		}
		return
	}
	requestModel.Success(c, gin.H{
		"taskId":      certificateInfo.TaskId,
		"certificate": certificateInfo,
	})
}

func CreateMultiAccountCertificateView(c *gin.Context) {
	var request requestModel.MultiAccountCertificateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	bindings := make([]models.CertificateDomain, 0, len(request.Domains))
	for _, item := range request.Domains {
		domain, err := db.GetDomainForId(item.AccountName, item.DomainId)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		if !canUseDomain(c, item.AccountName, domain.DomainName, 3) {
			requestModel.Forbidden(c, "无权为域名申请证书: "+domain.DomainName)
			return
		}
		bindings = append(bindings, models.CertificateDomain{DomainName: domain.DomainName, AccountName: domain.AccountName, ProviderType: domain.DnsFrom, ProviderDomainID: domain.Id, ChallengeMode: "direct"})
	}
	cert, err := certificate.EnqueueCertificateDomains(bindings, 0, "issue")
	if err != nil {
		if errors.Is(err, certificate.ErrQueueUnavailable) {
			requestModel.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		} else {
			requestModel.BadRequest(c, err.Error())
		}
		return
	}
	requestModel.Success(c, gin.H{"taskId": cert.TaskId, "certificate": cert})
}

func RenewCertificateView(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		requestModel.BadRequest(c, "certificate id 无效")
		return
	}
	if !canUseCertificate(c, id, 3) {
		requestModel.Forbidden(c, "无权续期该证书")
		return
	}
	cert, err := certificate.EnqueueRenewal(id)
	if err != nil {
		if errors.Is(err, certificate.ErrRenewalActive) {
			requestModel.Error(c, http.StatusConflict, err.Error(), nil)
		} else if errors.Is(err, certificate.ErrQueueUnavailable) {
			requestModel.Error(c, http.StatusServiceUnavailable, err.Error(), nil)
		} else {
			requestModel.BadRequest(c, err.Error())
		}
		return
	}
	requestModel.Success(c, gin.H{"taskId": cert.TaskId, "certificate": cert})
}

// GetCertificateViewWithId 根据id查询证书消息
func GetCertificateViewWithId(c *gin.Context) {
	// 绑定参数
	var request requestModel.CertificateIdRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	certificateDB, err := db.GetCertificateForId(request.CertificateId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if !canUseCertificate(c, request.CertificateId, 1) {
		requestModel.Forbidden(c, "无权访问该证书")
		return
	}
	requestModel.Success(c, certificateDB)
}

// DownloadCertificateViewWithId 根据id下载证书
func DownloadCertificateViewWithId(c *gin.Context) {
	// 绑定参数
	var request requestModel.DownloadCertificateViewWithIdRequest
	if err := c.ShouldBind(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	certificateDB, err := db.GetCertificateForId(request.CertificateId)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if !canUseCertificate(c, request.CertificateId, 1) {
		requestModel.Forbidden(c, "无权下载该证书")
		return
	}
	certificatePrivate := models.CertificatePrivate{SavePath: certificateDB.SavePath}
	// 读取证书信息
	resource, err := certificatePrivate.LoadResource()
	if err != nil {
		requestModel.BadRequest(c, "证书历史读取失败："+err.Error())
		return
	}
	if request.DownloadType != "cert" && request.DownloadType != "key" && request.DownloadType != "all" {
		requestModel.BadRequest(c, "downloadType 必须是 cert、key 或 all")
		return
	}
	for _, p := range []string{resource.SavePath, resource.CertificatePath, resource.PrivateKeyPath} {
		if !pathWithin(models.AccountConfig.Certificate.SavePath, p) {
			requestModel.Error(c, http.StatusInternalServerError, "证书路径越界", nil)
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	switch request.DownloadType {
	case "cert":
		c.FileAttachment(resource.CertificatePath, "certificate.crt")
	case "key":
		c.FileAttachment(resource.PrivateKeyPath, "private.key")
	case "all":
		zipPath, err := utils.ZipFolder(resource.SavePath)
		if err != nil {
			requestModel.BadRequest(c, "证书压缩失败："+err.Error())
			return
		}
		c.FileAttachment(zipPath, certificateArchiveName(certificateDB.DomainList))
	}
	return
}

// certificateArchiveName builds a readable, cross-platform safe attachment
// name from the certificate SAN list without exposing the internal save path.
func certificateArchiveName(domainList string) string {
	const maxBaseLength = 180
	seen := make(map[string]struct{})
	domains := make([]string, 0)
	for _, raw := range strings.Split(domainList, ",") {
		domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if domain == "" {
			continue
		}
		if strings.HasPrefix(domain, "*.") {
			domain = "wildcard." + strings.TrimPrefix(domain, "*.")
		}
		domain = strings.Map(func(r rune) rune {
			if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, domain)
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	if len(domains) == 0 {
		return "certificate.zip"
	}
	parts := make([]string, 0, len(domains))
	length := 0
	for i, domain := range domains {
		extra := len(domain)
		if len(parts) > 0 {
			extra++
		}
		remaining := len(domains) - i
		reserve := 0
		if remaining > 1 {
			reserve = len(fmt.Sprintf("_and-%d-more", remaining))
		}
		if length+extra+reserve > maxBaseLength {
			break
		}
		parts = append(parts, domain)
		length += extra
	}
	base := strings.Join(parts, "_")
	if len(parts) < len(domains) {
		base += fmt.Sprintf("_and-%d-more", len(domains)-len(parts))
	}
	return base + ".zip"
}

func CertificateContent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		requestModel.BadRequest(c, "certificate id 无效")
		return
	}
	if !canUseCertificate(c, id, 1) {
		requestModel.Forbidden(c, "无权查看该证书")
		return
	}
	kind := c.DefaultQuery("type", "cert")
	if kind != "cert" && kind != "key" {
		requestModel.BadRequest(c, "type 必须是 cert 或 key")
		return
	}
	cert, err := db.GetCertificateForId(id)
	if err != nil || cert.Stage != "success" {
		requestModel.NotFound(c, "证书不存在或尚未签发")
		return
	}
	resource, err := (&models.CertificatePrivate{SavePath: cert.SavePath}).LoadResource()
	if err != nil {
		requestModel.Error(c, http.StatusInternalServerError, "读取证书文件失败", nil)
		return
	}
	path := resource.CertificatePath
	if kind == "key" {
		path = resource.PrivateKeyPath
	}
	if !pathWithin(models.AccountConfig.Certificate.SavePath, path) {
		requestModel.Error(c, http.StatusInternalServerError, "证书路径越界", nil)
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		requestModel.Error(c, http.StatusInternalServerError, "读取证书内容失败", nil)
		return
	}
	if len(content) > 10<<20 {
		requestModel.Error(c, http.StatusInternalServerError, "证书内容超出限制", nil)
		return
	}
	c.Header("Cache-Control", "no-store")
	requestModel.Success(c, gin.H{"type": kind, "content": string(content)})
}

func DeleteCertificate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		requestModel.BadRequest(c, "certificate id 无效")
		return
	}
	if !canUseCertificate(c, id, 3) {
		requestModel.Forbidden(c, "无权删除该证书")
		return
	}
	var cert models.Certificate
	if err = db.DB.First(&cert, id).Error; err != nil {
		requestModel.NotFound(c, "证书不存在")
		return
	}
	var references, running, children int64
	var certificateTasks []models.CertificateTask
	if err = db.DB.Where("cert_id = ?", id).Find(&certificateTasks).Error; err != nil {
		requestModel.Error(c, http.StatusInternalServerError, "读取证书任务失败", nil)
		return
	}
	db.DB.Model(&models.Domains{}).Where("certificate_id = ?", id).Count(&references)
	for _, task := range certificateTasks {
		if task.State == "wait" || task.State == "apply" {
			running++
		}
	}
	db.DB.Model(&models.Certificate{}).Where("parent_id = ?", id).Count(&children)
	if references > 0 {
		requestModel.Error(c, http.StatusConflict, "证书仍被域名使用，不能删除", nil)
		return
	}
	if running > 0 {
		requestModel.Error(c, http.StatusConflict, "证书任务仍在运行，不能删除", nil)
		return
	}
	if children > 0 {
		requestModel.Error(c, http.StatusConflict, "证书仍有后继版本，不能删除", nil)
		return
	}

	originalPath, trashPath := "", ""
	if strings.TrimSpace(cert.SavePath) != "" {
		if !pathWithin(models.AccountConfig.Certificate.SavePath, cert.SavePath) {
			requestModel.Error(c, http.StatusInternalServerError, "证书路径越界", nil)
			return
		}
		if _, statErr := os.Stat(cert.SavePath); statErr == nil {
			trashRoot := filepath.Join(models.AccountConfig.Certificate.SavePath, ".trash")
			if err = os.MkdirAll(trashRoot, 0700); err != nil {
				requestModel.Error(c, http.StatusInternalServerError, "创建证书隔离目录失败", nil)
				return
			}
			originalPath = cert.SavePath
			trashPath = filepath.Join(trashRoot, fmt.Sprintf("certificate-%d-%d", id, time.Now().UnixNano()))
			if err = os.Rename(originalPath, trashPath); err != nil {
				requestModel.Error(c, http.StatusInternalServerError, "隔离证书文件失败", nil)
				return
			}
		} else if !os.IsNotExist(statErr) {
			requestModel.Error(c, http.StatusInternalServerError, "检查证书文件失败", nil)
			return
		}
	}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("cert_id = ?", id).Delete(&models.CertificateTask{}).Error; e != nil {
			return e
		}
		if e := tx.Where("certificate_id = ?", id).Delete(&models.CertificateDomain{}).Error; e != nil {
			return e
		}
		return tx.Delete(&models.Certificate{}, id).Error
	})
	if err != nil {
		if trashPath != "" {
			_ = os.Rename(trashPath, originalPath)
		}
		requestModel.Error(c, http.StatusInternalServerError, "删除证书数据失败", nil)
		return
	}
	if trashPath != "" {
		_ = os.RemoveAll(trashPath)
	}
	for _, task := range certificateTasks {
		if task.LogPath != "" && pathWithin(models.AccountConfig.Certificate.SavePath, task.LogPath) {
			_ = os.Remove(task.LogPath)
		}
	}
	user, _ := currentUser(c)
	audit(c, user.ID, "certificate.delete", fmt.Sprint(id), "success")
	requestModel.Success(c, gin.H{"id": id})
}

func pathWithin(root, candidate string) bool {
	a, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	b, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(a, b)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// GetCertificateTaskInfoByCertificateId 根据证书id查询证书任务信息
func GetCertificateTaskInfoByCertificateId(c *gin.Context) {
	// 绑定参数
	var request requestModel.CertificateIdRequest
	if err := c.ShouldBind(&request); err != nil {
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
	if err := c.ShouldBind(&request); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	// 查询任务信息
	taskInfo, err := db.GetTaskInfoById(request.Id)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if !canUseCertificate(c, taskInfo.CertId, 1) {
		requestModel.Forbidden(c, "无权访问该任务")
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

func GetTaskLogByTaskID(c *gin.Context) {
	taskInfo, err := db.GetTaskInfoByTaskID(c.Param("taskId"))
	if err != nil {
		requestModel.NotFound(c, err.Error())
		return
	}
	if !canUseCertificate(c, taskInfo.CertId, 1) {
		requestModel.Forbidden(c, "无权访问该任务")
		return
	}
	taskLog, err := utils.ReadFileContent(taskInfo.LogPath)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, gin.H{"task": taskInfo, "log": taskLog})
}

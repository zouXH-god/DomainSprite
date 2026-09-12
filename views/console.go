package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"DDNSServer/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type pageResult struct {
	Items    any   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

func paging(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func GetMeta(c *gin.Context) {
	options := func(values ...string) []gin.H {
		result := make([]gin.H, 0, len(values))
		for _, value := range values {
			result = append(result, gin.H{"value": value, "label": value})
		}
		return result
	}
	requestModel.Success(c, gin.H{
		"providers":         []gin.H{{"value": "Ali", "label": "阿里云"}, {"value": "Tencent", "label": "腾讯云"}, {"value": "Cloudflare", "label": "Cloudflare"}},
		"recordTypes":       options("A", "AAAA", "CNAME", "TXT", "MX", "NS", "SRV", "CAA"),
		"recordStates":      []gin.H{{"value": "ENABLE", "label": "启用"}, {"value": "DISABLE", "label": "停用"}},
		"lines":             []gin.H{{"value": "default", "label": "默认线路"}},
		"ttl":               gin.H{"min": 60, "max": 86400, "recommended": []int{60, 300, 600, 3600, 86400}},
		"certificateStages": options("wait", "challenging", "issued", "persisted", "success", "fail"),
		"downloadTypes":     options("cert", "key", "all"),
		"limits":            gin.H{"certificateBaseDomains": 50, "pageSize": 100},
	})
}
func GetDashboard(c *gin.Context) {
	var domains, certs, tasks int64
	var success, pending, failed, expiring int64
	queries := []*gorm.DB{
		db.DB.Model(&models.Domains{}).Count(&domains),
		db.DB.Model(&models.Certificate{}).Count(&certs),
		db.DB.Model(&models.CertificateTask{}).Count(&tasks),
		db.DB.Model(&models.Certificate{}).Where("stage = ?", "success").Count(&success),
		db.DB.Model(&models.Certificate{}).Where("stage IN ?", []string{"wait", "challenging", "issued", "persisted"}).Count(&pending),
		db.DB.Model(&models.Certificate{}).Where("stage = ?", "fail").Count(&failed),
		db.DB.Model(&models.Certificate{}).Where("stage = ? AND not_after <= ?", "success", time.Now().Add(30*24*time.Hour)).Count(&expiring),
	}
	for _, query := range queries {
		if query.Error != nil {
			requestModel.Error(c, http.StatusInternalServerError, "读取概览统计失败", nil)
			return
		}
	}
	var recent []models.CertificateTask
	if err := db.DB.Order("id desc").Limit(8).Find(&recent).Error; err != nil {
		requestModel.Error(c, http.StatusInternalServerError, "读取最近任务失败", nil)
		return
	}
	var accountRows []struct {
		ProviderType string
		Count        int
	}
	if err := db.DB.Model(&models.DNSAccount{}).Select("provider_type, count(*) AS count").Where("enabled = ?", true).Group("provider_type").Scan(&accountRows).Error; err != nil {
		requestModel.Error(c, http.StatusInternalServerError, "读取 DNS 账号统计失败", nil)
		return
	}
	providerCounts := map[string]int{}
	accountCount := 0
	for _, row := range accountRows {
		providerCounts[row.ProviderType] = row.Count
		accountCount += row.Count
	}
	queueErr := certificate.TaskQueueHealthy()
	requestModel.Success(c, gin.H{"accounts": accountCount, "providerDistribution": providerCounts, "domains": domains, "certificates": gin.H{"total": certs, "success": success, "pending": pending, "failed": failed, "expiring": expiring}, "tasks": tasks, "recentTasks": recent, "health": gin.H{"database": "ok", "redis": map[bool]string{true: "ok", false: "error"}[queueErr == nil], "worker": map[bool]string{true: "ok", false: "error"}[queueErr == nil]}})
}
func GetCertificatePage(c *gin.Context) {
	page, size := paging(c)
	var items []models.Certificate
	var total int64
	q := db.DB.Model(&models.Certificate{})
	if stage := c.Query("stage"); stage != "" {
		q = q.Where("stage = ?", stage)
	}
	q.Count(&total)
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		requestModel.Error(c, 500, err.Error(), nil)
		return
	}
	requestModel.Success(c, pageResult{items, page, size, total})
}
func GetCertificateDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		requestModel.BadRequest(c, "certificate id 无效")
		return
	}
	if !canUseCertificate(c, id, 1) {
		requestModel.Forbidden(c, "无权访问该证书")
		return
	}
	var cert models.Certificate
	if err = db.DB.First(&cert, id).Error; err != nil {
		requestModel.NotFound(c, err.Error())
		return
	}
	var domains []models.CertificateDomain
	var tasks []models.CertificateTask
	var versions []models.Certificate
	db.DB.Where("certificate_id = ?", id).Find(&domains)
	db.DB.Where("cert_id = ?", id).Order("id desc").Find(&tasks)
	db.DB.Where("lineage_id = ?", cert.LineageID).Order("id desc").Find(&versions)
	requestModel.Success(c, gin.H{"certificate": cert, "domains": domains, "tasks": tasks, "versions": versions})
}
func GetCertificateTasks(c *gin.Context) {
	page, size := paging(c)
	var items []models.CertificateTask
	var total int64
	q := db.DB.Model(&models.CertificateTask{})
	if v := c.Query("state"); v != "" {
		q = q.Where("state = ?", v)
	}
	if v := c.Query("kind"); v != "" {
		q = q.Where("kind = ?", v)
	}
	if v := c.Query("taskId"); v != "" {
		q = q.Where("task_id LIKE ?", "%"+v+"%")
	}
	if v := c.Query("certificateId"); v != "" {
		q = q.Where("cert_id = ?", v)
	}
	q.Count(&total)
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		requestModel.Error(c, 500, err.Error(), nil)
		return
	}
	requestModel.Success(c, pageResult{items, page, size, total})
}
func GetCertificateTask(c *gin.Context) {
	task, err := db.GetTaskInfoByTaskID(c.Param("taskId"))
	if err != nil {
		requestModel.NotFound(c, err.Error())
		return
	}
	if !canUseCertificate(c, task.CertId, 1) {
		requestModel.Forbidden(c, "无权访问该任务")
		return
	}
	requestModel.Success(c, task)
}

type cnameCheckRequest struct {
	Domains []string `json:"domains" binding:"required,min=1,max=50"`
}

func CheckCNAME(c *gin.Context) {
	var req cnameCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	normalized := make([]string, 0, len(req.Domains))
	for _, raw := range req.Domains {
		name, err := certificate.NormalizeDomain(raw)
		if err != nil {
			requestModel.BadRequest(c, err.Error())
			return
		}
		normalized = append(normalized, name)
	}
	items, err := GetCnameInfoForDomain(normalized)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{"name": item.Name, "type": item.Type, "domain": item.Domain, "fullDomainName": item.FullDomainName, "value": item.Value, "valid": utils.IsCNAMEEqual(item.FullDomainName, item.Value)})
	}
	requestModel.Success(c, result)
}
func PublicHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "DomainSprite"})
}
func PrivateHealth(c *gin.Context) {
	err := certificate.TaskQueueHealthy()
	status := http.StatusOK
	if err != nil {
		status = http.StatusServiceUnavailable
	}
	requestModel.Error(c, status, map[bool]string{true: "ok", false: "degraded"}[err == nil], gin.H{"database": db.DB != nil, "redis": err == nil, "worker": err == nil})
}

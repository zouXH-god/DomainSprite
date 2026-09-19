package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var fastOperationMu sync.Mutex

func InitFastStore() error {
	config := db.CurrentFastConfig()
	count, err := db.MigrateLegacyFastData(filepath.Join(config.DataPath, "fastData.json"), config.UseAccount)
	if err != nil {
		if errors.Is(err, db.ErrFastMigrationNeedsConfig) {
			slog.Warn("检测到旧快速解析 JSON，但无法唯一确定 DNS 账号；请在管理后台配置快速解析后重试迁移")
			return nil
		}
		return err
	}
	if count > 0 {
		slog.Info("快速解析 JSON 已迁移到数据库", "records", count)
	}
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
	host := c.RemoteIP()
	fastOperationMu.Lock()
	defer fastOperationMu.Unlock()
	var existing models.FastDDNSRecord
	if err := db.DB.Where("record_content = ?", host).First(&existing).Error; err == nil {
		token, decryptErr := db.DecryptSecret(existing.TokenEncrypted)
		if decryptErr != nil {
			requestModel.Error(c, 500, "读取快速解析凭证失败", nil)
			return
		}
		requestModel.Success(c, models.FastData{Token: token, RecordInfo: fastRowRecord(existing)})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		requestModel.Error(c, 500, "查询快速解析记录失败", nil)
		return
	}
	config := db.CurrentFastConfig()
	if config.UseAccount == "" || config.DomainId == "" || config.DomainName == "" {
		requestModel.Error(c, 503, "快速解析尚未配置 DNS 账号和承载域名", nil)
		return
	}
	provider, err := getProviderForAccountName(config.UseAccount)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	nextID := config.StartId
	var sequence models.SystemSetting
	if err := db.DB.First(&sequence, "key = ?", "fast.next_id").Error; err == nil {
		if parsed, parseErr := strconv.Atoi(sequence.Value); parseErr == nil {
			nextID = parsed
		}
	}
	var name string
	for {
		name = fmt.Sprintf("%s%0*d", config.NameStrata, config.IdLength, nextID)
		list, listErr := provider.GetRecordList(models.DNSSearch{DomainId: config.DomainId, DomainName: config.DomainName, KeyWord: name})
		if listErr != nil {
			requestModel.Error(c, 502, listErr.Error(), nil)
			return
		}
		nextID++
		if len(list.Records) == 0 {
			break
		}
	}
	record, err := provider.AddRecord(models.RecordInfo{DomainId: config.DomainId, DomainName: config.DomainName, RecordName: name, RecordType: "A", RecordContent: host})
	if err != nil {
		requestModel.Error(c, 502, err.Error(), nil)
		return
	}
	if record.DomainId == "" {
		record.DomainId = config.DomainId
	}
	if record.DomainName == "" {
		record.DomainName = config.DomainName
	}
	if record.RecordName == "" {
		record.RecordName = name
	}
	if record.RecordType == "" {
		record.RecordType = "A"
	}
	if record.RecordContent == "" {
		record.RecordContent = host
	}
	token, err := models.NewFastToken()
	if err != nil {
		_, _ = provider.DeleteRecord(record.DomainName, record.Id)
		requestModel.Error(c, 500, "生成快速解析凭证失败", nil)
		return
	}
	encrypted, err := db.EncryptSecret(token)
	if err != nil {
		_, _ = provider.DeleteRecord(record.DomainName, record.Id)
		requestModel.Error(c, 500, "加密快速解析凭证失败", nil)
		return
	}
	row := fastRecordRow(config.UseAccount, record, db.FastTokenHash(token), encrypted)
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if createErr := tx.Create(&row).Error; createErr != nil {
			return createErr
		}
		return tx.Where(models.SystemSetting{Key: "fast.next_id"}).Assign(models.SystemSetting{Value: strconv.Itoa(nextID)}).FirstOrCreate(&models.SystemSetting{}).Error
	})
	if err != nil {
		if _, cleanupErr := provider.DeleteRecord(record.DomainName, record.Id); cleanupErr != nil {
			slog.Error("快速 DDNS 数据库保存失败且云端补偿失败", "record_id", record.Id, "error", cleanupErr)
		}
		requestModel.Error(c, 500, "保存快速解析记录失败", nil)
		return
	}
	requestModel.Success(c, models.FastData{RecordInfo: record, Token: token})
}

type fastUpdateRequest struct {
	Token string `json:"token" form:"token" binding:"required"`
}

func UpdateForToken(c *gin.Context) {
	var req fastUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	host := c.RemoteIP()
	fastOperationMu.Lock()
	defer fastOperationMu.Unlock()
	var row models.FastDDNSRecord
	if err := db.DB.First(&row, "token_hash = ?", db.FastTokenHash(req.Token)).Error; err != nil {
		requestModel.BadRequest(c, "Token Not Exist")
		return
	}
	record := fastRowRecord(row)
	if record.RecordContent != host {
		record.RecordContent = host
		updated, updateErr := updateFastRecordRow(&row, record, 0)
		if updateErr != nil {
			requestModel.Error(c, 502, updateErr.Error(), nil)
			return
		}
		record = updated
	}
	requestModel.Success(c, models.FastData{Token: req.Token, RecordInfo: record})
}

type fastRecordView struct {
	ID               uint   `json:"id"`
	ProviderRecordID string `json:"providerRecordId"`
	Revision         uint64 `json:"revision"`
	DomainID         string `json:"domainId"`
	DomainName       string `json:"domainName"`
	FQDN             string `json:"fqdn"`
	RecordName       string `json:"recordName"`
	RecordType       string `json:"recordType"`
	RecordContent    string `json:"recordContent"`
	Line             string `json:"line"`
	Status           string `json:"status"`
	TTL              int64  `json:"ttl"`
	DNSFrom          string `json:"dnsFrom"`
	UpdateTime       string `json:"updateTime,omitempty"`
}

func toFastRecordView(row models.FastDDNSRecord) fastRecordView {
	info := fastRowRecord(row)
	fqdn := strings.TrimSuffix(info.RecordName+"."+info.DomainName, ".")
	if info.RecordName == "@" || info.RecordName == "" {
		fqdn = info.DomainName
	}
	view := fastRecordView{ID: row.ID, ProviderRecordID: row.ProviderRecordID, Revision: row.Revision, DomainID: info.DomainId, DomainName: info.DomainName, FQDN: fqdn,
		RecordName: info.RecordName, RecordType: info.RecordType, RecordContent: info.RecordContent,
		Line: info.Line, Status: info.Status, TTL: info.Ttl, DNSFrom: info.DnsFrom}
	if !info.UpdateTime.IsZero() {
		view.UpdateTime = info.UpdateTime.Format(time.RFC3339)
	}
	return view
}

func fastRowRecord(row models.FastDDNSRecord) models.RecordInfo {
	return models.RecordInfo{Id: row.ProviderRecordID, DomainId: row.ProviderDomainID, DomainName: row.DomainName,
		RecordName: row.RecordName, RecordType: row.RecordType, RecordContent: row.RecordContent,
		Line: row.Line, Status: row.Status, Ttl: row.TTL, DnsFrom: row.DNSFrom, CreateTime: row.CreatedAt, UpdateTime: row.UpdatedAt}
}

func fastRecordRow(account string, info models.RecordInfo, tokenHash, tokenEncrypted string) models.FastDDNSRecord {
	return models.FastDDNSRecord{DNSAccountName: account, ProviderDomainID: info.DomainId, DomainName: info.DomainName,
		ProviderRecordID: info.Id, RecordName: info.RecordName, RecordType: info.RecordType, RecordContent: info.RecordContent,
		Line: info.Line, Status: info.Status, TTL: info.Ttl, DNSFrom: info.DnsFrom, TokenHash: tokenHash,
		TokenEncrypted: tokenEncrypted, Revision: 1}
}

func fastPageParams(c *gin.Context) (int, int, error) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 0, 0, errors.New("page 必须为正整数")
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		return 0, 0, errors.New("pageSize 必须在 1 到 100 之间")
	}
	return page, pageSize, nil
}

// FastRecords returns the managed quick-DDNS records without their update tokens.
func FastRecords(c *gin.Context) {
	page, pageSize, err := fastPageParams(c)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	keyword := strings.ToLower(strings.TrimSpace(c.Query("search")))
	query := db.DB.Model(&models.FastDDNSRecord{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("lower(record_name || '.' || domain_name) LIKE ? OR lower(record_content) LIKE ? OR lower(dns_account_name) LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		requestModel.Error(c, 500, "查询快速解析数量失败", nil)
		return
	}
	var rows []models.FastDDNSRecord
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		requestModel.Error(c, 500, "查询快速解析记录失败", nil)
		return
	}
	items := make([]fastRecordView, 0, len(rows))
	for _, row := range rows {
		items = append(items, toFastRecordView(row))
	}
	requestModel.Success(c, gin.H{"items": items, "page": page, "pageSize": pageSize, "total": total})
}

type fastRecordEditRequest struct {
	RecordName    string `json:"recordName" binding:"required"`
	RecordContent string `json:"recordContent" binding:"required"`
	TTL           int64  `json:"ttl" binding:"required"`
	Revision      uint64 `json:"revision" binding:"required"`
}

var errFastRecordNotFound = errors.New("快速解析记录不存在")

func validFastRecordName(name string) bool {
	if name == "@" {
		return true
	}
	if name == "" || len(name) > 253 || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// UpdateFastRecord edits a quick-DDNS record while preserving its private token.
func UpdateFastRecord(c *gin.Context) {
	var req fastRecordEditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, "记录名、IPv4 地址和 TTL 均为必填项")
		return
	}
	req.RecordName = strings.TrimSpace(req.RecordName)
	req.RecordContent = strings.TrimSpace(req.RecordContent)
	if !validFastRecordName(req.RecordName) {
		requestModel.BadRequest(c, "记录名格式不正确")
		return
	}
	if ip := net.ParseIP(req.RecordContent); ip == nil || ip.To4() == nil {
		requestModel.BadRequest(c, "快速解析仅支持有效的 IPv4 地址")
		return
	}
	if req.TTL < 1 || req.TTL > 86400 {
		requestModel.BadRequest(c, "TTL 必须在 1 到 86400 秒之间")
		return
	}
	fastOperationMu.Lock()
	defer fastOperationMu.Unlock()
	var row models.FastDDNSRecord
	err := db.DB.First(&row, c.Param("recordId")).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			requestModel.Error(c, 404, errFastRecordNotFound.Error(), nil)
		} else {
			requestModel.Error(c, 500, "查询快速解析记录失败", nil)
		}
		return
	}
	if row.Revision != req.Revision {
		requestModel.Error(c, 409, "记录已被其他请求修改，请刷新后重试", nil)
		return
	}
	candidate := fastRowRecord(row)
	candidate.RecordName, candidate.RecordContent, candidate.Ttl = req.RecordName, req.RecordContent, req.TTL
	result, err := updateFastRecordRow(&row, candidate, req.Revision)
	if err != nil {
		if errors.Is(err, errFastRecordConflict) {
			requestModel.Error(c, 409, err.Error(), nil)
		} else {
			requestModel.Error(c, 502, err.Error(), nil)
		}
		return
	}
	if user, ok := currentUser(c); ok {
		audit(c, user.ID, "update", "fast-ddns-record:"+c.Param("recordId"), "success")
	}
	row.RecordName, row.RecordContent, row.TTL, row.Revision = result.RecordName, result.RecordContent, result.Ttl, row.Revision+1
	requestModel.Success(c, toFastRecordView(row))
}

var errFastRecordConflict = errors.New("记录已被其他请求修改，请刷新后重试")

func updateFastRecordRow(row *models.FastDDNSRecord, candidate models.RecordInfo, expectedRevision uint64) (models.RecordInfo, error) {
	provider, err := getProviderForAccountName(row.DNSAccountName)
	if err != nil {
		return models.RecordInfo{}, err
	}
	old := fastRowRecord(*row)
	updated, err := provider.UpdateRecord(candidate)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("更新 DNS 厂商记录: %w", err)
	}
	if updated.Id == "" {
		updated = candidate
	}
	revision := row.Revision
	if expectedRevision > 0 {
		revision = expectedRevision
	}
	result := db.DB.Model(&models.FastDDNSRecord{}).Where("id = ? AND revision = ?", row.ID, revision).Updates(map[string]any{
		"record_name": updated.RecordName, "record_content": updated.RecordContent, "ttl": updated.Ttl,
		"line": updated.Line, "status": updated.Status, "dns_from": updated.DnsFrom,
		"revision": gorm.Expr("revision + 1"), "last_error": "",
	})
	if result.Error == nil && result.RowsAffected == 1 {
		return updated, nil
	}
	if _, rollbackErr := provider.UpdateRecord(old); rollbackErr != nil {
		slog.Error("快速解析数据库更新失败且云端回滚失败", "record_id", row.ProviderRecordID, "error", rollbackErr)
	}
	if result.Error != nil {
		return models.RecordInfo{}, fmt.Errorf("保存快速解析记录: %w", result.Error)
	}
	return models.RecordInfo{}, errFastRecordConflict
}
func LegacyIpToDomainRecord(c *gin.Context) { deprecatedFast(c); IpToDomainRecord(c) }
func LegacyUpdateForToken(c *gin.Context) {
	deprecatedFast(c)
	if c.Query("token") != "" {
		c.Request.Form = map[string][]string{"token": {c.Query("token")}}
	}
	UpdateForToken(c)
}

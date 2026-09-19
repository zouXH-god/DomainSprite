package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

type fastRecordView struct {
	ID            string `json:"id"`
	DomainID      string `json:"domainId"`
	DomainName    string `json:"domainName"`
	FQDN          string `json:"fqdn"`
	RecordName    string `json:"recordName"`
	RecordType    string `json:"recordType"`
	RecordContent string `json:"recordContent"`
	Line          string `json:"line"`
	Status        string `json:"status"`
	TTL           int64  `json:"ttl"`
	DNSFrom       string `json:"dnsFrom"`
	UpdateTime    string `json:"updateTime,omitempty"`
}

func toFastRecordView(info models.RecordInfo) fastRecordView {
	fqdn := strings.TrimSuffix(info.RecordName+"."+info.DomainName, ".")
	if info.RecordName == "@" || info.RecordName == "" {
		fqdn = info.DomainName
	}
	view := fastRecordView{ID: info.Id, DomainID: info.DomainId, DomainName: info.DomainName, FQDN: fqdn,
		RecordName: info.RecordName, RecordType: info.RecordType, RecordContent: info.RecordContent,
		Line: info.Line, Status: info.Status, TTL: info.Ttl, DNSFrom: info.DnsFrom}
	if !info.UpdateTime.IsZero() {
		view.UpdateTime = info.UpdateTime.Format(time.RFC3339)
	}
	return view
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
	if fastStore == nil {
		requestModel.Error(c, 500, "快速 DDNS 未初始化", nil)
		return
	}
	page, pageSize, err := fastPageParams(c)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	keyword := strings.ToLower(strings.TrimSpace(c.Query("search")))
	snapshot := fastStore.Snapshot()
	items := make([]fastRecordView, 0, len(snapshot.DataList))
	for _, item := range snapshot.DataList {
		view := toFastRecordView(item.RecordInfo)
		haystack := strings.ToLower(strings.Join([]string{view.FQDN, view.RecordContent, view.RecordType, view.DNSFrom}, " "))
		if keyword == "" || strings.Contains(haystack, keyword) {
			items = append(items, view)
		}
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	requestModel.Success(c, gin.H{"items": items[start:end], "page": page, "pageSize": pageSize, "total": total})
}

type fastRecordEditRequest struct {
	RecordName    string `json:"recordName" binding:"required"`
	RecordContent string `json:"recordContent" binding:"required"`
	TTL           int64  `json:"ttl" binding:"required"`
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
	if fastStore == nil {
		requestModel.Error(c, 500, "快速 DDNS 未初始化", nil)
		return
	}
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
	provider, err := getProviderForAccountName(models.AccountConfig.FastConfig.UseAccount)
	if err != nil {
		requestModel.Error(c, 502, err.Error(), nil)
		return
	}
	recordID := c.Param("recordId")
	var oldRecord, result models.RecordInfo
	cloudUpdated := false
	err = fastStore.WithWrite(func(data *models.FastDataJson) error {
		for i := range data.DataList {
			item := &data.DataList[i]
			if item.RecordInfo.Id != recordID {
				continue
			}
			oldRecord = item.RecordInfo
			candidate := oldRecord
			candidate.RecordName = req.RecordName
			candidate.RecordContent = req.RecordContent
			candidate.Ttl = req.TTL
			updated, updateErr := provider.UpdateRecord(candidate)
			if updateErr != nil {
				return fmt.Errorf("更新 DNS 厂商记录: %w", updateErr)
			}
			cloudUpdated = true
			if updated.Id == "" {
				updated = candidate
			}
			item.RecordInfo = updated
			result = updated
			return nil
		}
		return errFastRecordNotFound
	})
	if err != nil {
		if errors.Is(err, errFastRecordNotFound) {
			requestModel.Error(c, 404, err.Error(), nil)
			return
		}
		if cloudUpdated {
			if _, rollbackErr := provider.UpdateRecord(oldRecord); rollbackErr != nil {
				slog.Error("快速解析编辑持久化失败且云端回滚失败", "record_id", recordID, "error", rollbackErr)
			}
			requestModel.Error(c, 500, "本地保存失败，已尝试回滚云端记录", nil)
			return
		}
		requestModel.Error(c, 502, err.Error(), nil)
		return
	}
	if user, ok := currentUser(c); ok {
		audit(c, user.ID, "update", "fast-ddns-record:"+recordID, "success")
	}
	requestModel.Success(c, toFastRecordView(result))
}
func LegacyIpToDomainRecord(c *gin.Context) { deprecatedFast(c); IpToDomainRecord(c) }
func LegacyUpdateForToken(c *gin.Context) {
	deprecatedFast(c)
	if c.Query("token") != "" {
		c.Request.Form = map[string][]string{"token": {c.Query("token")}}
	}
	UpdateForToken(c)
}

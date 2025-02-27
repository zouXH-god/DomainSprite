package cloudflare

import (
	"DDNSServer/models"
	"context"
	"errors"
	"fmt"
	"github.com/cloudflare/cloudflare-go"
)

const DNSFromTag = "Cloudflare"

var ZoneList = map[string]models.DomainInfo{}
var RecordList = map[string]models.RecordInfo{}

// CloudflareProvider 实现 Cloudflare 的适配器
type CloudflareProvider struct {
	api *cloudflare.API
}

// NewCloudflareProvider 创建 Cloudflare 适配器实例
func NewCloudflareProvider(apiKey, email string) (*CloudflareProvider, error) {
	api, err := cloudflare.New(apiKey, email)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare API client: %w", err)
	}
	return &CloudflareProvider{api: api}, nil
}

// GetDomainList 实现 DomainListProvider 接口，获取域名列表
func (c *CloudflareProvider) GetDomainList(info models.DomainsSearch) (models.DomainList, error) {
	domains, err := c.api.ListZones(context.Background())
	if err != nil {
		return models.DomainList{}, fmt.Errorf("failed to list zones: %w", err)
	}

	var domainList []models.DomainInfo
	for _, zone := range domains {
		domainInfo := models.DomainInfo{
			Id:          zone.ID,
			DomainName:  zone.Name,
			Status:      zone.Status,
			Paused:      zone.Paused,
			Type:        zone.Type,
			NameServers: zone.NameServers,
			CreateTime:  zone.CreatedOn,
			UpdateTime:  zone.ModifiedOn,
			DnsFrom:     DNSFromTag,
		}
		ZoneList[zone.Name] = domainInfo
		domainList = append(domainList, domainInfo)
	}

	return models.DomainList{
		Domains:    domainList,
		PageNumber: info.PageNumber,
		PageSize:   info.PageSize,
		DnsFrom:    DNSFromTag,
	}, nil
}

// getResourceContainer 获取资源容器
func getResourceContainer(domainId string) cloudflare.ResourceContainer {
	return cloudflare.ResourceContainer{
		Identifier: domainId,
		Type:       cloudflare.ZoneType,
	}
}

// getNotEmpty 返回第一个非空字符串
func getNotEmpty(s ...string) string {
	for _, i := range s {
		if i != "" {
			return i
		}
	}
	return ""
}

// GetRecordList 实现 DomainProvider 接口，获取域名解析记录列表
func (c *CloudflareProvider) GetRecordList(info models.DNSSearch) ([]models.RecordInfo, error) {
	resourceContainer := getResourceContainer(info.DomainId)
	ListDNSRecordsParams := cloudflare.ListDNSRecordsParams{
		Type:    info.TypeKeyWord,
		Name:    getNotEmpty(info.RRKeyWord, info.KeyWord),
		Content: getNotEmpty(info.ValueKeyWord, info.KeyWord),
		ResultInfo: cloudflare.ResultInfo{
			Page:    int(info.PageNumber),
			PerPage: int(info.PageSize),
		},
		Direction: cloudflare.ListDirection(info.Direction),
		Order:     info.OrderBy,
		Match:     "any",
	}
	records, _, err := c.api.ListDNSRecords(context.Background(), &resourceContainer, ListDNSRecordsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}

	var recordList []models.RecordInfo
	for _, record := range records {
		recordInfo := models.RecordInfo{
			Id:            record.ID,
			DomainId:      info.DomainId,
			DomainName:    record.Name,
			RecordName:    record.Name,
			RecordType:    record.Type,
			RecordContent: record.Content,
			Status:        "Enable", // Cloudflare 没有显式的状态字段
			Proxied:       record.Proxied != nil && *record.Proxied,
			Ttl:           int64(record.TTL),
			CreateTime:    record.CreatedOn,
			UpdateTime:    record.ModifiedOn,
			DnsFrom:       DNSFromTag,
		}
		RecordList[record.ID] = recordInfo
		recordList = append(recordList, recordInfo)
	}

	return recordList, nil
}

// AddRecord 实现 RecordProvider 接口，添加 DNS 记录
func (c *CloudflareProvider) AddRecord(info models.RecordInfo) (models.RecordInfo, error) {
	resourceContainer := getResourceContainer(info.DomainId)
	record := cloudflare.CreateDNSRecordParams{
		Type:    info.RecordType,
		Name:    info.RecordName,
		Content: info.RecordContent,
		TTL:     int(info.Ttl),
		Proxied: &info.Proxied,
	}

	resp, err := c.api.CreateDNSRecord(context.Background(), &resourceContainer, record)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("failed to create DNS record: %w", err)
	}

	return models.RecordInfo{
		Id:            resp.ID,
		DomainId:      info.DomainId,
		DomainName:    info.DomainName,
		RecordName:    resp.Name,
		RecordType:    resp.Type,
		RecordContent: resp.Content,
		Proxied:       resp.Proxied != nil && *resp.Proxied,
		Ttl:           int64(resp.TTL),
		CreateTime:    resp.CreatedOn,
		UpdateTime:    resp.ModifiedOn,
		DnsFrom:       DNSFromTag,
	}, nil
}

// UpdateRecord 实现 RecordProvider 接口，更新 DNS 记录
func (c *CloudflareProvider) UpdateRecord(info models.RecordInfo) (models.RecordInfo, error) {
	resourceContainer := getResourceContainer(info.DomainId)
	record := cloudflare.UpdateDNSRecordParams{
		ID:      info.Id,
		Type:    info.RecordType,
		Name:    info.RecordName,
		Content: info.RecordContent,
		TTL:     int(info.Ttl),
		Proxied: &info.Proxied,
	}

	_, err := c.api.UpdateDNSRecord(context.Background(), &resourceContainer, record)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("failed to update DNS record: %w", err)
	}

	return info, nil
}

// DeleteRecord 实现 RecordProvider 接口，删除 DNS 记录
func (c *CloudflareProvider) DeleteRecord(DomainName string, recordId string) (models.RecordInfo, error) {
	// 需要先获取记录信息
	record, err := c.GetRecordInfo(DomainName, recordId)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("failed to get record info: %w", err)
	}
	resourceContainer := getResourceContainer(record.DomainId)

	err = c.api.DeleteDNSRecord(context.Background(), &resourceContainer, recordId)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return record, nil
}

// SetRecordStatus 实现 RecordProvider 接口，设置记录状态
func (c *CloudflareProvider) SetRecordStatus(DomainName string, recordId string, status string) (models.RecordInfo, error) {
	// Cloudflare 不支持直接设置记录状态，可以通过更新记录实现
	record, err := c.GetRecordInfo(DomainName, recordId)
	if err != nil {
		return models.RecordInfo{}, fmt.Errorf("failed to get record info: %w", err)
	}

	// 更新记录
	record.Status = status
	return c.UpdateRecord(record)
}

// GetRecordInfo 实现 RecordProvider 接口，获取记录信息
func (c *CloudflareProvider) GetRecordInfo(DomainName string, recordId string) (models.RecordInfo, error) {
	RecordInfo := RecordList[recordId]
	if RecordInfo.Id != "" {
		return RecordInfo, nil
	} else {
		// 判断 ZoneList 是否已初始化
		if len(ZoneList) == 0 {
			_, err := c.GetDomainList(models.DomainsSearch{
				PageNumber: 1,
				PageSize:   100,
			})
			if err != nil {
				return models.RecordInfo{}, fmt.Errorf("failed to get domain list: %w", err)
			}
		}
		// 获取域名对应的信息
		zone := ZoneList[DomainName]
		if zone.Id == "" {
			return models.RecordInfo{}, errors.New("domain not found")
		}
		// 获取域名下的指定记录
		resourceContainer := getResourceContainer(zone.Id)
		record, err := c.api.GetDNSRecord(context.Background(), &resourceContainer, recordId)
		if err != nil {
			return models.RecordInfo{}, fmt.Errorf("failed to get record info: %w", err)
		}
		recordInfo := models.RecordInfo{
			Id:            record.ID,
			DomainId:      zone.Id,
			DomainName:    record.Name,
			RecordName:    record.Name,
			RecordType:    record.Type,
			RecordContent: record.Content,
			Status:        "Enable", // Cloudflare 没有显式的状态字段
			Proxied:       record.Proxied != nil && *record.Proxied,
			Ttl:           int64(record.TTL),
			CreateTime:    record.CreatedOn,
			UpdateTime:    record.ModifiedOn,
			DnsFrom:       DNSFromTag,
		}
		RecordList[record.ID] = recordInfo
		return recordInfo, nil
	}
}

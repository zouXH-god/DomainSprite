package ali

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/utils"
	"fmt"
	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

type AliDNSClient struct {
	client *alidns20150109.Client
	info   models.Account
}

const DNSFromTag = "Ali"

// NewAliDNSClient 创建 Ali 适配器实例
func NewAliDNSClient(info models.Account, AccessKeyId, AccessKeySecret string) (*AliDNSClient, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(AccessKeyId),
		AccessKeySecret: tea.String(AccessKeySecret),
	}
	config.Endpoint = tea.String("alidns.cn-hangzhou.aliyuncs.com")

	// 创建客户端
	client, err := alidns20150109.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &AliDNSClient{client: client, info: info}, nil
}

func (c *AliDNSClient) GetAccountInfo() (info models.Account) {
	return c.info
}

// GetDomainList 获取域名列表
func (c *AliDNSClient) GetDomainList(info models.DomainsSearch) (models.DomainList, error) {
	describeDomainsRequest := &alidns20150109.DescribeDomainsRequest{}
	utils.SetRequestFieldsWithTag(&info, describeDomainsRequest, DNSFromTag)
	runtime := &util.RuntimeOptions{}
	result, _err := c.client.DescribeDomainsWithOptions(describeDomainsRequest, runtime)
	if _err != nil {
		return models.DomainList{}, _err
	}
	domainList := models.DomainList{
		DnsFrom:    DNSFromTag,
		PageNumber: tea.Int64Value(result.Body.PageNumber),
		PageSize:   tea.Int64Value(result.Body.TotalCount),
		Domains:    []models.DomainInfo{},
	}
	for _, domain := range result.Body.Domains.Domain {
		DomainInfo := models.DomainInfo{
			Domains: models.Domains{
				Id:          tea.StringValue(domain.DomainId),
				DomainName:  tea.StringValue(domain.DomainName),
				GroupId:     tea.StringValue(domain.GroupId),
				GroupName:   tea.StringValue(domain.GroupName),
				DnsFrom:     DNSFromTag,
				AccountName: c.info.Name,
			},
		}
		domainList.Domains = append(domainList.Domains, DomainInfo)
		err := db.AddDomainInfo(DomainInfo)
		if err != nil {
			fmt.Println("域名加入数据库失败：", err)
		}
	}

	return domainList, nil
}

// GetRecordList 获取域名解析记录列表
func (c *AliDNSClient) GetRecordList(info models.DNSSearch) (models.RecordInfoList, error) {
	describeDomainRecordsRequest := &alidns20150109.DescribeDomainRecordsRequest{}
	utils.SetRequestFieldsWithTag(&info, describeDomainRecordsRequest, DNSFromTag)
	runtime := &util.RuntimeOptions{}
	var recordList models.RecordInfoList
	result, _err := c.client.DescribeDomainRecordsWithOptions(describeDomainRecordsRequest, runtime)
	if _err != nil {
		return recordList, _err
	}
	for _, record := range result.Body.DomainRecords.Record {
		recordInfo := models.RecordInfo{
			Id:            tea.StringValue(record.RecordId),
			DomainId:      info.DomainId,
			DomainName:    tea.StringValue(record.DomainName),
			RecordName:    tea.StringValue(record.RR),
			RecordType:    tea.StringValue(record.Type),
			RecordContent: tea.StringValue(record.Value),
			Line:          tea.StringValue(record.Line),
			Status:        tea.StringValue(record.Status),
			Ttl:           tea.Int64Value(record.TTL),
			Weight:        tea.Int32Value(record.Weight),
			DnsFrom:       DNSFromTag,
		}
		recordList.Records = append(recordList.Records, recordInfo)
	}
	recordList.PageSize = tea.Int64Value(result.Body.PageSize)
	recordList.PageNumber = tea.Int64Value(result.Body.PageNumber)
	recordList.TotalCount = tea.Int64Value(result.Body.TotalCount)
	return recordList, nil
}

// AddRecord 实现 RecordProvider 接口，添加 DNS 记录
func (c *AliDNSClient) AddRecord(info models.RecordInfo) (models.RecordInfo, error) {
	addDomainRecordRequest := &alidns20150109.AddDomainRecordRequest{
		DomainName: tea.String(info.DomainName),
		RR:         tea.String(info.RecordName),
		Type:       tea.String(info.RecordType),
		Value:      tea.String(info.RecordContent),
	}
	runtime := &util.RuntimeOptions{}
	result, _err := c.client.AddDomainRecordWithOptions(addDomainRecordRequest, runtime)
	if _err != nil {
		return models.RecordInfo{}, _err
	}
	info.Id = tea.StringValue(result.Body.RecordId)
	return info, nil
}

// UpdateRecord 修改解析记录
func (c *AliDNSClient) UpdateRecord(info models.RecordInfo) (models.RecordInfo, error) {
	updateDomainRecordRequest := &alidns20150109.UpdateDomainRecordRequest{
		RecordId: tea.String(info.Id),
		RR:       tea.String(info.RecordName),
		Type:     tea.String(info.RecordType),
		Value:    tea.String(info.RecordContent),
	}
	if info.Ttl > 0 {
		updateDomainRecordRequest.TTL = tea.Int64(info.Ttl)
	}
	if info.Line != "" {
		updateDomainRecordRequest.Line = tea.String(info.Line)
	}
	runtime := &util.RuntimeOptions{}
	_, _err := c.client.UpdateDomainRecordWithOptions(updateDomainRecordRequest, runtime)
	if _err != nil {
		return info, _err
	}
	return info, nil
}

// DeleteRecord 删除解析记录
func (c *AliDNSClient) DeleteRecord(DomainName string, RecordId string) (models.RecordInfo, error) {
	deleteDomainRecordRequest := &alidns20150109.DeleteDomainRecordRequest{
		RecordId: tea.String(RecordId),
	}
	runtime := &util.RuntimeOptions{}
	_, _err := c.client.DeleteDomainRecordWithOptions(deleteDomainRecordRequest, runtime)
	if _err != nil {
		return models.RecordInfo{}, _err
	}
	return models.RecordInfo{
		Id: RecordId,
	}, nil
}

// SetRecordStatus 修改解析记录状态
func (c *AliDNSClient) SetRecordStatus(DomainName string, RecordId string, Status string) (models.RecordInfo, error) {
	setDomainRecordStatusRequest := &alidns20150109.SetDomainRecordStatusRequest{
		RecordId: tea.String(RecordId),
		Status:   tea.String(Status),
	}
	runtime := &util.RuntimeOptions{}
	_, _err := c.client.SetDomainRecordStatusWithOptions(setDomainRecordStatusRequest, runtime)
	if _err != nil {
		return models.RecordInfo{}, _err
	}
	return models.RecordInfo{}, nil
}

// GetRecordInfo 获取解析记录详细信息
func (c *AliDNSClient) GetRecordInfo(DomainName string, RecordId string) (models.RecordInfo, error) {
	describeDomainRecordInfoRequest := &alidns20150109.DescribeDomainRecordInfoRequest{
		RecordId: tea.String(RecordId),
	}
	runtime := &util.RuntimeOptions{}
	result, _err := c.client.DescribeDomainRecordInfoWithOptions(describeDomainRecordInfoRequest, runtime)
	if _err != nil {
		return models.RecordInfo{}, _err
	}
	return models.RecordInfo{
		Id:            tea.StringValue(result.Body.RecordId),
		DomainId:      tea.StringValue(result.Body.DomainId),
		DomainName:    tea.StringValue(result.Body.DomainName),
		RecordName:    tea.StringValue(result.Body.RR),
		RecordType:    tea.StringValue(result.Body.Type),
		RecordContent: tea.StringValue(result.Body.Value),
		Line:          tea.StringValue(result.Body.Line),
		Status:        tea.StringValue(result.Body.Status),
		Ttl:           tea.Int64Value(result.Body.TTL),
	}, nil
}

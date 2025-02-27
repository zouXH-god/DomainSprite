package tencent

import (
	"DDNSServer/models"
	"DDNSServer/utils"
	"fmt"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
	"strconv"
	"strings"
)

type TencentDNSClient struct {
	client *dnspod.Client
}

var RecordListData = map[string]models.RecordInfo{}

const DNSFromTag = "Tencent"

func NewTencentProvider(secretId, secretKey string) (*TencentDNSClient, error) {
	credential := common.NewCredential(
		secretId,
		secretKey,
	)
	// 实例化一个client选项，可选的，没有特殊需求可以跳过
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	// 实例化要请求产品的client对象,clientProfile是可选的
	client, err := dnspod.NewClient(credential, "", cpf)
	if err != nil {
		return nil, err
	}
	return &TencentDNSClient{
		client: client,
	}, nil
}

// GetDomainList 获取域名列表
func (c *TencentDNSClient) GetDomainList(info models.DomainsSearch) (models.DomainList, error) {
	request := dnspod.NewDescribeDomainListRequest()
	request.Offset = common.Int64Ptr((info.PageNumber - 1) * info.PageSize)
	utils.SetRequestFieldsWithTag(&info, request, DNSFromTag)
	response, err := c.client.DescribeDomainList(request)
	if err != nil {
		fmt.Println(err)
		return models.DomainList{}, err
	}
	RecordList := models.DomainList{
		PageNumber: info.PageNumber,
		PageSize:   info.PageSize,
		DnsFrom:    DNSFromTag,
	}

	for _, domain := range response.Response.DomainList {
		RecordList.Domains = append(RecordList.Domains, models.DomainInfo{
			Id:         strconv.FormatUint(*domain.DomainId, 10),
			DomainName: tea.StringValue(domain.Name),
			GroupId:    strconv.FormatUint(*domain.GroupId, 10),
			DnsFrom:    DNSFromTag,
		})
	}
	return RecordList, nil
}

// GetRecordList 获取域名解析列表
func (c *TencentDNSClient) GetRecordList(info models.DNSSearch) ([]models.RecordInfo, error) {
	request := dnspod.NewDescribeRecordListRequest()
	request.Offset = common.Uint64Ptr(uint64((info.PageNumber - 1) * info.PageSize))
	request.Limit = common.Uint64Ptr(uint64(info.PageSize))
	info.DomainIdTC, _ = strconv.ParseUint(info.DomainId, 10, 64)
	utils.SetRequestFieldsWithTag(&info, request, DNSFromTag)
	response, err := c.client.DescribeRecordList(request)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	var RecordList []models.RecordInfo
	for _, record := range response.Response.RecordList {
		RecordInfo := models.RecordInfo{
			Id:            strconv.FormatUint(*record.RecordId, 10),
			DomainId:      info.DomainId,
			DomainName:    info.DomainName,
			Line:          tea.StringValue(record.Line),
			RecordName:    tea.StringValue(record.Name),
			RecordType:    tea.StringValue(record.Type),
			RecordContent: tea.StringValue(record.Value),
			Status:        tea.StringValue(record.Status),
			Weight:        int32(tea.Uint64Value(record.Weight)),
			Ttl:           int64(tea.Uint64Value(record.TTL)),
			DnsFrom:       DNSFromTag,
		}
		RecordListData[strconv.FormatUint(*record.RecordId, 10)] = RecordInfo
		RecordList = append(RecordList, RecordInfo)
	}
	return RecordList, nil
}

// AddRecord 添加记录
func (c *TencentDNSClient) AddRecord(info models.RecordInfo) (models.RecordInfo, error) {
	request := dnspod.NewCreateRecordRequest()
	info.ToTencent()
	utils.SetRequestFieldsWithTag(&info, request, DNSFromTag)
	_, err := c.client.CreateRecord(request)
	if err != nil {
		fmt.Println(err)
		return models.RecordInfo{}, err
	}
	return info, nil
}

// UpdateRecord 修改记录
func (c *TencentDNSClient) UpdateRecord(info models.RecordInfo) (models.RecordInfo, error) {
	request := dnspod.NewModifyRecordRequest()
	info.ToTencent()
	utils.SetRequestFieldsWithTag(&info, request, DNSFromTag)
	_, err := c.client.ModifyRecord(request)
	if err != nil {
		fmt.Println(err)
		return models.RecordInfo{}, err
	}
	return info, nil
}

// DeleteRecord 删除记录
func (c *TencentDNSClient) DeleteRecord(DomainName string, RecordIdStr string) (models.RecordInfo, error) {
	request := dnspod.NewDeleteRecordRequest()
	RecordId, err := strconv.Atoi(RecordIdStr)
	if err != nil {
		return models.RecordInfo{}, err
	}
	request.RecordId = common.Uint64Ptr(uint64(RecordId))
	request.Domain = common.StringPtr(DomainName)
	_, err = c.client.DeleteRecord(request)
	if err != nil {
		fmt.Println(err)
		return models.RecordInfo{}, err
	}
	return models.RecordInfo{
		Id: RecordIdStr,
	}, nil
}

// SetRecordStatus 设置记录状态
func (c *TencentDNSClient) SetRecordStatus(DomainName string, RecordIdStr string, Status string) (models.RecordInfo, error) {
	request := dnspod.NewModifyRecordStatusRequest()
	RecordId, err := strconv.Atoi(RecordIdStr)
	if err != nil {
		return models.RecordInfo{}, err
	}
	request.RecordId = common.Uint64Ptr(uint64(RecordId))
	request.Status = common.StringPtr(strings.ToUpper(Status))
	request.Domain = common.StringPtr(DomainName)
	_, err = c.client.ModifyRecordStatus(request)
	if err != nil {
		fmt.Println(err)
		return models.RecordInfo{}, err
	}
	return models.RecordInfo{
		Id:     RecordIdStr,
		Status: Status,
	}, nil
}

// GetRecordInfo 获取记录信息
func (c *TencentDNSClient) GetRecordInfo(DomainName string, RecordIdStr string) (models.RecordInfo, error) {
	// 先获取缓存，如果有就直接返回
	RecordInfo := RecordListData[RecordIdStr]
	if RecordInfo.Id != "" {
		return RecordInfo, nil
	}
	// 没有直接查询
	request := dnspod.NewDescribeRecordRequest()
	RecordId, err := strconv.Atoi(RecordIdStr)
	if err != nil {
		return models.RecordInfo{}, err
	}
	request.RecordId = common.Uint64Ptr(uint64(RecordId))
	request.Domain = common.StringPtr(DomainName)
	// 查询
	response, err := c.client.DescribeRecord(request)
	if err != nil {
		fmt.Println(err)
		return models.RecordInfo{}, err
	}
	// 因为要统一化，所以需要转换
	var Status string
	if response.Response.RecordInfo.Enabled != nil && *response.Response.RecordInfo.Enabled == 1 {
		Status = "true"
	} else {
		Status = "false"
	}
	RecordInfo = models.RecordInfo{
		Id:            strconv.FormatUint(*response.Response.RecordInfo.Id, 10),
		DomainId:      strconv.FormatUint(*response.Response.RecordInfo.DomainId, 10),
		DomainName:    DomainName,
		Line:          tea.StringValue(response.Response.RecordInfo.RecordLine),
		RecordName:    tea.StringValue(response.Response.RecordInfo.SubDomain),
		RecordType:    tea.StringValue(response.Response.RecordInfo.RecordType),
		RecordContent: tea.StringValue(response.Response.RecordInfo.Value),
		Status:        Status,
		Ttl:           int64(tea.Uint64Value(response.Response.RecordInfo.TTL)),
		Weight:        int32(tea.Uint64Value(response.Response.RecordInfo.Weight)),
		DnsFrom:       DNSFromTag,
	}
	// 缓存记录
	RecordListData[RecordIdStr] = RecordInfo
	return RecordInfo, nil
}

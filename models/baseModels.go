package models

import (
	"time"
)

// DomainInfo 域名信息
type DomainInfo struct {
	Domains
	Paused              bool         `json:"paused"`              // 域名是否暂停
	NameServers         []string     `json:"nameServers"`         // 域名解析服务器
	OriginalNameServers []string     `json:"originalNameServers"` // 域名原始解析服务器
	RecordList          []RecordInfo `json:"recordList"`          // 域名记录列表
}

// DomainList 域名列表
type DomainList struct {
	Domains    []DomainInfo `json:"domains"`    // 域名列表
	PageNumber int64        `json:"pageNumber"` // 页码
	PageSize   int64        `json:"pageSize"`   // 每页条数
	DnsFrom    string       `json:"dnsFrom"`    // 域名来源
}

// RecordInfo 记录信息
type RecordInfo struct {
	Id            string    `form:"id" json:"id"`                                                                      // 记录ID
	DomainId      string    `form:"domainId" binding:"required" json:"domainId" Ali:""`                                // 域名ID
	DomainName    string    `form:"domainName" binding:"required" json:"domainName" Ali:"DomainName" Tencent:"Domain"` // 域名
	Line          string    `form:"line" json:"line" Ali:"Line" Tencent:"RecordLine"`                                  // 解析线路
	RecordName    string    `form:"recordName" json:"recordName" Ali:"RR" Tencent:"SubDomain"`                         // 记录名称
	RecordType    string    `form:"recordType" json:"recordType" Ali:"Type" Tencent:"RecordType"`                      // 记录类型
	RecordContent string    `form:"recordContent" json:"recordContent" Ali:"Value" Tencent:"Value"`                    // 记录值
	Status        string    `form:"status" json:"status" Ali:"" Tencent:""`                                            // 记录状态
	Locked        bool      `form:"locked" json:"locked"`                                                              // 是否锁定
	Proxied       bool      `form:"proxied" json:"proxied"`                                                            // 是否启用代理
	Ttl           int64     `form:"ttl" json:"ttl" Ali:"TTL"`                                                          // TTL
	Weight        int32     `form:"weight" json:"weight" Ali:"Priority"`                                               // 权重
	Settings      string    `form:"settings" json:"settings"`                                                          // 设置
	Meta          string    `form:"meta" json:"meta"`                                                                  // 元数据
	Comment       string    `form:"comment" json:"comment"`                                                            // 备注
	Tags          []string  `form:"tags" json:"tags" Ali:"" Tencent:""`                                                // 标签
	CreateTime    time.Time `form:"createTime" json:"createTime"`                                                      // 创建时间
	UpdateTime    time.Time `form:"updateTime" json:"updateTime"`                                                      // 更新时间
	DnsFrom       string    `form:"dnsFrom" json:"dnsFrom"`                                                            // 域名解析来源
	// Tencent 适配
	IdTC       uint64 `json:"idTC" Tencent:"RecordId"`
	DomainIdTC uint64 `json:"domainIdTC" Tencent:"DomainId"`
	TtlTC      uint64 `json:"ttlTC" Tencent:"TTL"`
	WeightTC   uint64 `json:"weightTC" Tencent:"Weight"`
}

type RecordProvider interface {
	// GetDomainList 获取域名列表
	GetDomainList(info DomainsSearch) (result DomainList, _err error)
	// GetRecordList 获取域名解析列表
	GetRecordList(info DNSSearch) (result []RecordInfo, _err error)
	// AddRecord 添加记录
	AddRecord(info RecordInfo) (result RecordInfo, _err error)
	// UpdateRecord 修改记录
	UpdateRecord(info RecordInfo) (result RecordInfo, _err error)
	// DeleteRecord 删除记录
	DeleteRecord(DomainName string, RecordId string) (result RecordInfo, _err error)
	// SetRecordStatus 设置记录状态
	SetRecordStatus(DomainName string, RecordId string, Status string) (result RecordInfo, _err error)
	// GetRecordInfo 获取记录信息
	GetRecordInfo(DomainName string, RecordId string) (result RecordInfo, _err error)
}

// DomainsSearch 域名搜索结构体
type DomainsSearch struct {
	KeyWord         string `form:"keyWord" Ali:"KeyWord" Tencent:"Keyword"` // 关键字
	PageNumber      int64  `form:"pageNumber" Ali:"PageNumber"`             // 页码
	PageSize        int64  `form:"pageSize" Ali:"PageSize" Tencent:"Limit"` // 每页条数
	GroupId         string `form:"groupId" Ali:"GroupId" Tencent:"GroupId"` // 分组ID
	SearchMode      string `form:"searchMode" Ali:"SearchMode"`             // 搜索模式 LIKE | EXACT
	ResourceGroupId string `form:"resourceGroupId" Ali:"ResourceGroupId"`   // 资源组ID
	Starmark        *bool  `form:"starmark" Ali:"Starmark"`                 // 是否星标域名
}

// DNSSearch DNS记录搜索结构体
type DNSSearch struct {
	DomainId     string `form:"domainId" binding:"required" json:"domain_id" Ali:""`                                // 域名ID
	DomainName   string `form:"domainName" binding:"required" json:"domain_name" Ali:"DomainName" Tencent:"Domain"` // 域名
	PageNumber   int64  `form:"pageNumber" json:"page_number" Ali:"PageNumber" Tencent:""`                          // 页码
	PageSize     int64  `form:"pageSize" json:"page_size" Ali:"PageSize"`                                           // 每页条数
	KeyWord      string `form:"keyWord" json:"key_word" Ali:"KeyWord" Tencent:"Keyword"`                            // 全部关键字搜索
	RRKeyWord    string `form:"rrKeyWord" json:"rr_key_word" Ali:"RRKeyWord" Tencent:"SubDomain"`                   // 主机记录关键字搜索
	TypeKeyWord  string `form:"typeKeyWord" json:"type_key_word" Ali:"TypeKeyWord" Tencent:"RecordType"`            // 解析类型关键字搜索
	ValueKeyWord string `form:"valueKeyWord" json:"value_key_word" Ali:"ValueKeyWord" Tencent:""`                   // 记录值的关键字搜索
	OrderBy      string `form:"orderBy" json:"order_by" Ali:"" Tencent:"SortField"`                                 // 排序方式
	Direction    string `form:"direction" json:"direction" Ali:"" Tencent:"SortType"`                               // 排序方向 DESC | ASC
	GroupId      int64  `form:"groupId" json:"group_id" Ali:"GroupId" Tencent:"GroupId"`                            // 查询的分组ID
	Line         string `form:"line" json:"line" Ali:"Line" Tencent:"RecordLine"`                                   // 线路类型
	Status       string `form:"status" json:"status" Ali:""`                                                        // 解析状态  Enable | Disable
	// Tencent 适配
	DomainIdTC uint64 `Tencent:"DomainId"`
}

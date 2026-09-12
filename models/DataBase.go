package models

import (
	"strings"
	"time"
)

type Domains struct {
	DBID          uint      `gorm:"primaryKey" json:"-"`
	Id            string    `gorm:"not null;uniqueIndex:idx_domain_identity" json:"id"` // 云厂商域名ID
	DomainName    string    `gorm:"not null" json:"domainName"`                         // 域名
	GroupId       string    `gorm:"null" json:"groupId"`                                // 域名组ID
	GroupName     string    `gorm:"null" json:"groupName"`                              // 域名组名称
	Status        string    `gorm:"null" json:"status"`                                 // 域名状态
	Type          string    `gorm:"null" json:"type"`                                   // 域名类型
	CertificateId int       `gorm:"null" json:"certificateId"`
	CreateTime    time.Time `gorm:"null" json:"createTime"`                                      // 域名创建时间
	UpdateTime    time.Time `gorm:"null" json:"updateTime"`                                      // 域名更新时间
	DnsFrom       string    `gorm:"not null;uniqueIndex:idx_domain_identity" json:"dnsFrom"`     // 域名解析来源
	AccountName   string    `gorm:"not null;uniqueIndex:idx_domain_identity" json:"accountName"` // 域名所属账号名称
	DNSAccountID  uint      `gorm:"index" json:"dnsAccountId"`
}

type Certificate struct {
	Id                 int        `gorm:"primaryKey" json:"id"`
	State              string     `gorm:"index;default:'wait'" json:"state"`
	TaskId             string     `gorm:"uniqueIndex" json:"taskId"`
	SavePath           string     `gorm:"null" json:"savePath"`
	Issuer             string     `gorm:"null" json:"issuer"`    // 颁发者
	Subject            string     `gorm:"null" json:"subject"`   // 主题
	NotBefore          time.Time  `gorm:"null" json:"notBefore"` // 有效期开始时间
	NotAfter           time.Time  `gorm:"index" json:"notAfter"`
	DNSNames           string     `gorm:"null" json:"DNSNames"`   // SAN中的DNS名称
	CommonName         string     `gorm:"null" json:"commonName"` // 主题中的Common Name
	DomainList         string     `gorm:"null" json:"domainList"` // 允许的域名列表
	ParentID           uint       `gorm:"index" json:"parentId"`
	LineageID          string     `gorm:"index" json:"lineageId"`
	Stage              string     `gorm:"index;default:'wait'" json:"stage"`
	DomainsFingerprint string     `gorm:"index" json:"domainsFingerprint"`
	IssuedAt           *time.Time `json:"issuedAt"`
	ActivatedAt        *time.Time `json:"activatedAt"`
	RetiredAt          *time.Time `gorm:"index" json:"retiredAt"`
	FilesDeletedAt     *time.Time `json:"filesDeletedAt"`
	LastError          string     `json:"lastError"`
}

type CertificateTask struct {
	Id              int        `gorm:"primaryKey" json:"id"`
	CertId          int        `gorm:"index" json:"certId"`
	TaskId          string     `gorm:"index" json:"taskId"`
	LogPath         string     `gorm:"null" json:"logPath"`
	State           string     `gorm:"index;default:'wait'" json:"state"`
	Result          string     `gorm:"null" json:"result"`
	CreateTime      time.Time  `gorm:"null" json:"createTime"`
	Kind            string     `gorm:"index;default:'issue'" json:"kind"`
	Attempt         int        `json:"attempt"`
	ActiveKey       *string    `gorm:"uniqueIndex" json:"activeKey,omitempty"`
	NodeID          uint       `gorm:"index" json:"nodeId"`
	NodeGroupID     uint       `gorm:"index" json:"nodeGroupId"`
	LeaseID         string     `gorm:"index" json:"leaseId"`
	RemoteStage     string     `gorm:"index" json:"remoteStage"`
	AssignedAt      *time.Time `json:"assignedAt"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
}

type CertificateDomain struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	CertificateID    int    `gorm:"index;uniqueIndex:idx_cert_domain" json:"certificateId"`
	DomainName       string `gorm:"uniqueIndex:idx_cert_domain" json:"domainName"`
	AccountName      string `gorm:"index" json:"accountName"`
	ProviderType     string `json:"providerType"`
	ProviderDomainID string `json:"providerDomainId"`
	ChallengeMode    string `gorm:"index" json:"challengeMode"`
}

type ChallengeCleanup struct {
	ID          uint   `gorm:"primaryKey"`
	TaskID      string `gorm:"index"`
	AccountName string
	DomainName  string
	RecordID    string
	RecordName  string
	RecordValue string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
}

// MatchesDomain 方法检查给定的域名是否与证书匹配
func (c *Certificate) MatchesDomain(domain string) bool {
	// 检查 Common Name 是否匹配
	if matchesDomain(domain, c.CommonName) {
		return true
	}
	// 检查 SAN 中的 DNSNames 是否匹配
	for _, dnsName := range strings.Split(c.DNSNames, ",") {
		if matchesDomain(domain, dnsName) {
			return true
		}
	}
	return false
}

// matchesDomain 辅助函数，检查域名是否与证书中的名称（包括通配符）匹配
func matchesDomain(domain, certName string) bool {
	// 完全匹配
	if certName == domain {
		return true
	}
	// 处理通配符匹配，例如 *.example.com
	if strings.HasPrefix(certName, "*.") {
		suffix := certName[2:] // 去掉 *.
		if strings.HasSuffix(domain, "."+suffix) {
			prefix := domain[:len(domain)-len(suffix)-1]
			// 前缀不为空且不含点，表示匹配一级子域名
			if prefix != "" && !strings.Contains(prefix, ".") {
				return true
			}
		}
	}
	return false
}

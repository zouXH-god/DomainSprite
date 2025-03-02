package models

import (
	"strings"
	"time"
)

type Domains struct {
	Id            string    `gorm:"primaryKey" json:"id"`       // 域名ID
	DomainName    string    `gorm:"not null" json:"domainName"` // 域名
	GroupId       string    `gorm:"null" json:"groupId"`        // 域名组ID
	GroupName     string    `gorm:"null" json:"groupName"`      // 域名组名称
	Status        string    `gorm:"null" json:"status"`         // 域名状态
	Type          string    `gorm:"null" json:"type"`           // 域名类型
	CertificateId string    `gorm:"null" json:"certificateId"`
	CreateTime    time.Time `gorm:"null" json:"createTime"`  // 域名创建时间
	UpdateTime    time.Time `gorm:"null" json:"updateTime"`  // 域名更新时间
	DnsFrom       string    `gorm:"not null" json:"dnsFrom"` // 域名解析来源
}

type Certificate struct {
	Id         string    `gorm:"primaryKey" json:"id"`
	SavePath   string    `gorm:"null" json:"savePath"`
	Issuer     string    `gorm:"null" json:"issuer"`     // 颁发者
	Subject    string    `gorm:"null" json:"subject"`    // 主题
	NotBefore  time.Time `gorm:"null" json:"notBefore"`  // 有效期开始时间
	NotAfter   time.Time `gorm:"null" json:"notAfter"`   // 有效期结束时间
	DNSNames   string    `gorm:"null" json:"DNSNames"`   // SAN中的DNS名称
	CommonName string    `gorm:"null" json:"commonName"` // 主题中的Common Name
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

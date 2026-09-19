package models

import "time"

// FastDDNSRecord is the durable representation of a quick-DDNS record. Tokens
// are one-way hashed; the plaintext is returned only when a record is created.
type FastDDNSRecord struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	DNSAccountName   string    `gorm:"not null;index;uniqueIndex:idx_fast_provider_record" json:"dnsAccountName"`
	ProviderDomainID string    `gorm:"not null" json:"domainId"`
	DomainName       string    `gorm:"not null;index" json:"domainName"`
	ProviderRecordID string    `gorm:"not null;uniqueIndex:idx_fast_provider_record" json:"providerRecordId"`
	RecordName       string    `gorm:"not null;index" json:"recordName"`
	RecordType       string    `gorm:"not null;default:A" json:"recordType"`
	RecordContent    string    `gorm:"not null;index" json:"recordContent"`
	Line             string    `json:"line"`
	Status           string    `json:"status"`
	TTL              int64     `json:"ttl"`
	DNSFrom          string    `json:"dnsFrom"`
	TokenHash        string    `gorm:"size:64;not null;uniqueIndex" json:"-"`
	TokenEncrypted   string    `gorm:"not null" json:"-"`
	Revision         uint64    `gorm:"not null;default:1" json:"revision"`
	LastError        string    `json:"lastError,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

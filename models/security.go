package models

import "time"

type User struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	Username           string     `gorm:"uniqueIndex;size:128;not null" json:"username"`
	PasswordHash       string     `gorm:"not null" json:"-"`
	Role               string     `gorm:"index;not null" json:"role"`
	Enabled            bool       `gorm:"index;not null;default:true" json:"enabled"`
	MustChangePassword bool       `gorm:"not null;default:false" json:"mustChangePassword"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
}

type WebSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"userId"`
	TokenHash    string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	CSRFHash     string    `gorm:"size:64;not null" json:"-"`
	ExpiresAt    time.Time `gorm:"index;not null" json:"expiresAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type AccessKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	Name       string     `gorm:"not null" json:"name"`
	KeyID      string     `gorm:"uniqueIndex;size:64;not null" json:"keyId"`
	SecretHash string     `gorm:"size:64;not null" json:"-"`
	SecretTail string     `gorm:"size:8" json:"secretTail"`
	Scopes     string     `gorm:"not null" json:"scopes"`
	Enabled    bool       `gorm:"index;not null;default:true" json:"enabled"`
	ExpiresAt  *time.Time `gorm:"index" json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

type DNSAccount struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	Name                 string    `gorm:"uniqueIndex;not null" json:"name"`
	ProviderType         string    `gorm:"index;not null" json:"providerType"`
	AccessKeyIDEncrypted string    `gorm:"not null" json:"-"`
	SecretEncrypted      string    `gorm:"not null" json:"-"`
	CredentialTail       string    `json:"credentialTail"`
	Enabled              bool      `gorm:"index;not null;default:true" json:"enabled"`
	Version              uint      `gorm:"not null;default:1" json:"version"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type ACMEProfile struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Name                  string    `gorm:"uniqueIndex;not null" json:"name"`
	Email                 string    `gorm:"uniqueIndex;not null" json:"email"`
	CADirURL              string    `json:"caDirUrl"`
	AccountKeyEncrypted   string    `json:"-"`
	RegistrationEncrypted string    `json:"-"`
	IsDefault             bool      `gorm:"index" json:"isDefault"`
	Enabled               bool      `gorm:"index;not null;default:true" json:"enabled"`
	IssuedCount           uint64    `json:"issuedCount"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type DomainScope struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	DNSAccountID     uint      `gorm:"index;uniqueIndex:idx_scope_identity;not null" json:"dnsAccountId"`
	ProviderDomainID string    `gorm:"index;not null" json:"providerDomainId"`
	ZoneName         string    `gorm:"index;not null" json:"zoneName"`
	FQDN             string    `gorm:"uniqueIndex:idx_scope_identity;not null" json:"fqdn"`
	InheritChildren  bool      `gorm:"not null;default:true" json:"inheritChildren"`
	CreatedAt        time.Time `json:"createdAt"`
}

type DomainGrant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_scope;not null" json:"userId"`
	ScopeID   uint      `gorm:"uniqueIndex:idx_user_scope;not null" json:"scopeId"`
	Level     string    `gorm:"index;not null" json:"level"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:128" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	Sensitive bool      `gorm:"not null;default:false" json:"sensitive"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"userId"`
	AuthMethod string    `gorm:"index" json:"authMethod"`
	Action     string    `gorm:"index;not null" json:"action"`
	Resource   string    `gorm:"index" json:"resource"`
	Result     string    `gorm:"index" json:"result"`
	RequestID  string    `gorm:"index" json:"requestId"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

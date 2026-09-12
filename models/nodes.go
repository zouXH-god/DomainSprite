package models

import "time"

type NodeGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null" json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `gorm:"index;default:true" json:"enabled"`
	IsDefault   bool      `gorm:"index" json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
type CertificateNode struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	Name                 string     `gorm:"uniqueIndex;not null" json:"name"`
	Remark               string     `json:"remark"`
	GroupID              uint       `gorm:"index;not null" json:"groupId"`
	IdentitySerial       string     `gorm:"uniqueIndex" json:"identitySerial"`
	Status               string     `gorm:"index" json:"status"`
	Enabled              bool       `gorm:"index;default:true" json:"enabled"`
	Draining             bool       `gorm:"index" json:"draining"`
	Version              string     `json:"version"`
	ProtocolVersion      string     `json:"protocolVersion"`
	Capacity             int        `json:"capacity"`
	Running              int        `json:"running"`
	Providers            string     `json:"providers"`
	LastHeartbeatAt      *time.Time `gorm:"index" json:"lastHeartbeatAt"`
	LastAssignedAt       *time.Time `gorm:"index" json:"lastAssignedAt"`
	CertificateExpiresAt *time.Time `json:"certificateExpiresAt"`
	RevokedAt            *time.Time `json:"revokedAt"`
	LastError            string     `json:"lastError"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}
type NodeRegistrationToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	TokenTail string     `json:"tokenTail"`
	GroupID   uint       `gorm:"index;not null" json:"groupId"`
	CreatedBy uint       `gorm:"index" json:"createdBy"`
	ExpiresAt time.Time  `gorm:"index" json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt"`
	RevokedAt *time.Time `json:"revokedAt"`
	CreatedAt time.Time  `json:"createdAt"`
}
type UserNodeGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_node_group;not null" json:"userId"`
	GroupID   uint      `gorm:"uniqueIndex:idx_user_node_group;not null" json:"groupId"`
	CreatedAt time.Time `json:"createdAt"`
}
type NodeACMEPolicy struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	NodeID          uint      `gorm:"uniqueIndex:idx_node_acme;not null" json:"nodeId"`
	ACMEProfileID   uint      `gorm:"uniqueIndex:idx_node_acme;not null" json:"acmeProfileId"`
	HourlyLimit     int       `json:"hourlyLimit"`
	DailyLimit      int       `json:"dailyLimit"`
	CooldownSeconds int       `json:"cooldownSeconds"`
	Enabled         bool      `gorm:"index;default:true" json:"enabled"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
type NodeRateUsage struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	NodeID        uint      `gorm:"uniqueIndex:idx_node_rate_bucket;not null" json:"nodeId"`
	ACMEProfileID uint      `gorm:"uniqueIndex:idx_node_rate_bucket;not null" json:"acmeProfileId"`
	Bucket        string    `gorm:"uniqueIndex:idx_node_rate_bucket;not null" json:"bucket"`
	Reserved      int       `json:"reserved"`
	Issued        int       `json:"issued"`
	Failed        int       `json:"failed"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type NodeTaskLease struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TaskID        string     `gorm:"uniqueIndex;not null" json:"taskId"`
	NodeID        uint       `gorm:"index;not null" json:"nodeId"`
	GroupID       uint       `gorm:"index;not null" json:"groupId"`
	ACMEProfileID uint       `gorm:"index;not null" json:"acmeProfileId"`
	LeaseID       string     `gorm:"uniqueIndex;not null" json:"leaseId"`
	Stage         string     `gorm:"index" json:"stage"`
	ExpiresAt     time.Time  `gorm:"index" json:"expiresAt"`
	LastAckAt     *time.Time `json:"lastAckAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

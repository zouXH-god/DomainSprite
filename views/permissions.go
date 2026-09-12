package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"strings"

	"github.com/gin-gonic/gin"
)

func canUseDomain(c *gin.Context, accountName, fqdn string, required int) bool {
	user, ok := currentUser(c)
	if !ok {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	fqdn, err := normalizeFQDN(fqdn)
	if err != nil {
		return false
	}
	var account models.DNSAccount
	if db.DB.Where("name = ? AND enabled = ?", accountName, true).First(&account).Error != nil {
		return false
	}
	var grants []struct {
		Level           string
		FQDN            string
		InheritChildren bool
	}
	db.DB.Table("domain_grants").Select("domain_grants.level, domain_scopes.fqdn, domain_scopes.inherit_children").Joins("JOIN domain_scopes ON domain_scopes.id = domain_grants.scope_id").Where("domain_grants.user_id = ? AND domain_scopes.dns_account_id = ?", user.ID, account.ID).Scan(&grants)
	for _, grant := range grants {
		if permissionRank(grant.Level) < required {
			continue
		}
		if fqdn == grant.FQDN || (grant.InheritChildren && strings.HasSuffix(fqdn, "."+grant.FQDN)) {
			return true
		}
	}
	return false
}

func canUseAccount(c *gin.Context, accountName string, required int) bool {
	user, ok := currentUser(c)
	if !ok {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	var count int64
	db.DB.Table("domain_grants").Joins("JOIN domain_scopes ON domain_scopes.id = domain_grants.scope_id").Joins("JOIN dns_accounts ON dns_accounts.id = domain_scopes.dns_account_id").Where("domain_grants.user_id = ? AND dns_accounts.name = ?", user.ID, accountName).Where("CASE domain_grants.level WHEN 'owner' THEN 4 WHEN 'certificate_manager' THEN 3 WHEN 'dns_editor' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END >= ?", required).Count(&count)
	return count > 0
}

func canViewZone(c *gin.Context, accountName, zone string) bool {
	user, ok := currentUser(c)
	if !ok {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	var count int64
	db.DB.Table("domain_grants").Joins("JOIN domain_scopes ON domain_scopes.id = domain_grants.scope_id").Joins("JOIN dns_accounts ON dns_accounts.id = domain_scopes.dns_account_id").Where("domain_grants.user_id = ? AND dns_accounts.name = ? AND domain_scopes.zone_name = ?", user.ID, accountName, strings.ToLower(strings.TrimSuffix(zone, "."))).Count(&count)
	return count > 0
}

func recordFQDN(name, zone string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return zone
	}
	if strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(zone)) {
		return name
	}
	return name + "." + zone
}

func canUseCertificate(c *gin.Context, certificateID int, required int) bool {
	user, ok := currentUser(c)
	if !ok {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	var domains []models.CertificateDomain
	if db.DB.Where("certificate_id = ?", certificateID).Find(&domains).Error != nil || len(domains) == 0 {
		return false
	}
	for _, domain := range domains {
		if !canUseDomain(c, domain.AccountName, domain.DomainName, required) {
			return false
		}
	}
	return true
}

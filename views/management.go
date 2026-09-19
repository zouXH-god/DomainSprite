package views

import (
	"DDNSServer/DDNS"
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Register(c *gin.Context) {
	var setting models.SystemSetting
	if db.DB.First(&setting, "key = ?", "registration.open").Error != nil || setting.Value != "true" {
		requestModel.Forbidden(c, "注册未开放")
		return
	}
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	user := models.User{Username: strings.TrimSpace(req.Username), PasswordHash: hash, Role: "user", Enabled: true}
	if err = db.DB.Create(&user).Error; err != nil {
		requestModel.Error(c, 409, "用户名已存在", nil)
		return
	}
	audit(c, user.ID, "register", "user:"+user.Username, "success")
	requestModel.Success(c, user)
}

func Users(c *gin.Context) {
	var users []models.User
	if err := db.DB.Order("id").Find(&users).Error; err != nil {
		requestModel.Error(c, 500, "查询用户失败", nil)
		return
	}
	requestModel.Success(c, users)
}
func CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=3,max=128"`
		Password string `json:"password" binding:"required,min=10,max=256"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	if req.Role != "user" && req.Role != "admin" {
		requestModel.BadRequest(c, "role 无效")
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	u := models.User{Username: strings.TrimSpace(req.Username), PasswordHash: hash, Role: req.Role, Enabled: true, MustChangePassword: true}
	if err = db.DB.Create(&u).Error; err != nil {
		requestModel.Error(c, 409, "用户名已存在", nil)
		return
	}
	actor, _ := currentUser(c)
	audit(c, actor.ID, "user.create", fmt.Sprint(u.ID), "success")
	requestModel.Success(c, u)
}
func UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		requestModel.BadRequest(c, "用户 ID 无效")
		return
	}
	var req struct {
		Enabled  *bool  `json:"enabled"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	updates := map[string]any{}
	var target models.User
	if db.DB.First(&target, id).Error != nil {
		requestModel.NotFound(c, "用户不存在")
		return
	}
	if target.Role == "admin" && ((req.Enabled != nil && !*req.Enabled) || (req.Role != "" && req.Role != "admin")) {
		var admins int64
		db.DB.Model(&models.User{}).Where("role = ? AND enabled = ?", "admin", true).Count(&admins)
		if admins <= 1 {
			requestModel.Error(c, 409, "不能停用或降级最后一个管理员", nil)
			return
		}
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Role != "" {
		if req.Role != "user" && req.Role != "admin" {
			requestModel.BadRequest(c, "role 无效")
			return
		}
		updates["role"] = req.Role
	}
	if req.Password != "" {
		hash, e := db.HashPassword(req.Password)
		if e != nil {
			requestModel.BadRequest(c, e.Error())
			return
		}
		updates["password_hash"] = hash
		updates["must_change_password"] = true
	}
	if err = db.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		requestModel.Error(c, 500, "更新用户失败", nil)
		return
	}
	if req.Enabled != nil && !*req.Enabled {
		db.DB.Where("user_id = ?", id).Delete(&models.WebSession{})
	}
	if req.Password != "" {
		db.DB.Where("user_id = ?", id).Delete(&models.WebSession{})
	}
	actor, _ := currentUser(c)
	audit(c, actor.ID, "user.update", fmt.Sprint(id), "success")
	requestModel.Success(c, "ok")
}
func RevokeUserSessions(c *gin.Context) {
	result := db.DB.Where("user_id = ?", c.Param("id")).Delete(&models.WebSession{})
	if result.Error != nil {
		requestModel.Error(c, 500, "撤销会话失败", nil)
		return
	}
	requestModel.Success(c, gin.H{"revoked": result.RowsAffected})
}

var validScopes = map[string]bool{"dns:read": true, "dns:write": true, "certificate:issue": true, "certificate:renew": true, "certificate:download": true}

func AccessKeys(c *gin.Context) {
	u, _ := currentUser(c)
	var items []models.AccessKey
	db.DB.Where("user_id = ?", u.ID).Order("id desc").Find(&items)
	requestModel.Success(c, items)
}
func CreateAccessKey(c *gin.Context) {
	u, _ := currentUser(c)
	var req struct {
		Name      string     `json:"name" binding:"required"`
		Scopes    []string   `json:"scopes" binding:"required,min=1"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	for _, s := range req.Scopes {
		if !validScopes[s] {
			requestModel.BadRequest(c, "scope 无效: "+s)
			return
		}
	}
	idSecret, _ := db.RandomToken(18)
	secret, _ := db.RandomToken(32)
	keyID := "ds_" + idSecret
	tail := secret
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	row := models.AccessKey{UserID: u.ID, Name: req.Name, KeyID: keyID, SecretHash: db.HashToken(secret), SecretTail: tail, Scopes: strings.Join(req.Scopes, ","), Enabled: true, ExpiresAt: req.ExpiresAt}
	if err := db.DB.Create(&row).Error; err != nil {
		requestModel.Error(c, 500, "创建 AccessKey 失败", nil)
		return
	}
	audit(c, u.ID, "access_key.create", fmt.Sprint(row.ID), "success")
	requestModel.Success(c, gin.H{"accessKey": row, "secret": secret})
}
func RevokeAccessKey(c *gin.Context) {
	u, _ := currentUser(c)
	now := time.Now()
	result := db.DB.Model(&models.AccessKey{}).Where("id = ? AND user_id = ?", c.Param("id"), u.ID).Updates(map[string]any{"enabled": false, "revoked_at": now})
	if result.RowsAffected == 0 {
		requestModel.NotFound(c, "AccessKey 不存在")
		return
	}
	audit(c, u.ID, "access_key.revoke", c.Param("id"), "success")
	requestModel.Success(c, "ok")
}
func RotateAccessKey(c *gin.Context) {
	u, _ := currentUser(c)
	var old models.AccessKey
	if err := db.DB.Where("id = ? AND user_id = ? AND revoked_at IS NULL", c.Param("id"), u.ID).First(&old).Error; err != nil {
		requestModel.NotFound(c, "AccessKey 不存在")
		return
	}
	idPart, _ := db.RandomToken(18)
	secret, _ := db.RandomToken(32)
	tail := secret
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	next := models.AccessKey{UserID: u.ID, Name: old.Name + " (rotated)", KeyID: "ds_" + idPart, SecretHash: db.HashToken(secret), SecretTail: tail, Scopes: old.Scopes, Enabled: true, ExpiresAt: old.ExpiresAt}
	now := time.Now()
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&next).Error; e != nil {
			return e
		}
		return tx.Model(&old).Updates(map[string]any{"enabled": false, "revoked_at": now}).Error
	})
	if err != nil {
		requestModel.Error(c, 500, "轮换失败", nil)
		return
	}
	requestModel.Success(c, gin.H{"accessKey": next, "secret": secret})
}

func DNSAccounts(c *gin.Context) {
	var rows []models.DNSAccount
	db.DB.Order("name").Find(&rows)
	for i := range rows {
		rows[i].AccessKeyIDEncrypted = ""
		rows[i].SecretEncrypted = ""
	}
	requestModel.Success(c, rows)
}
func SaveDNSAccount(c *gin.Context) {
	var req struct {
		ID           uint   `json:"id"`
		Name         string `json:"name" binding:"required"`
		ProviderType string `json:"providerType" binding:"required"`
		AccessKeyID  string `json:"accessKeyId"`
		Secret       string `json:"secret"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if req.ID == 0 && c.Param("id") != "" {
		parsed, e := strconv.ParseUint(c.Param("id"), 10, 64)
		if e != nil {
			requestModel.BadRequest(c, "DNS 账号 ID 无效")
			return
		}
		req.ID = uint(parsed)
	}
	if req.ProviderType != "Ali" && req.ProviderType != "Tencent" && req.ProviderType != "Cloudflare" {
		requestModel.BadRequest(c, "providerType 无效")
		return
	}
	var row models.DNSAccount
	if req.ID > 0 {
		if err := db.DB.First(&row, req.ID).Error; err != nil {
			requestModel.NotFound(c, "DNS 账号不存在")
			return
		}
	} else {
		row.Name = req.Name
		row.ProviderType = req.ProviderType
		row.Enabled = true
	}
	if req.AccessKeyID != "" {
		row.AccessKeyIDEncrypted, _ = db.EncryptSecret(req.AccessKeyID)
		tail := req.AccessKeyID
		if len(tail) > 4 {
			tail = tail[len(tail)-4:]
		}
		row.CredentialTail = tail
	}
	if req.Secret != "" {
		row.SecretEncrypted, _ = db.EncryptSecret(req.Secret)
	}
	if row.AccessKeyIDEncrypted == "" || row.SecretEncrypted == "" {
		requestModel.BadRequest(c, "新账号必须提供完整凭据")
		return
	}
	row.Name = req.Name
	row.ProviderType = req.ProviderType
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	row.Version++
	if err := db.DB.Save(&row).Error; err != nil {
		requestModel.Error(c, 409, "保存 DNS 账号失败", nil)
		return
	}
	if err := db.LoadRuntimeConfig(); err != nil {
		requestModel.Error(c, 500, "刷新运行配置失败", nil)
		return
	}
	u, _ := currentUser(c)
	audit(c, u.ID, "dns_account.save", fmt.Sprint(row.ID), "success")
	row.AccessKeyIDEncrypted = ""
	row.SecretEncrypted = ""
	requestModel.Success(c, row)
}
func DeleteDNSAccount(c *gin.Context) {
	var count int64
	db.DB.Model(&models.Domains{}).Where("dns_account_id = ? OR account_name = (SELECT name FROM dns_accounts WHERE id = ?)", c.Param("id"), c.Param("id")).Count(&count)
	if count == 0 {
		db.DB.Model(&models.FastDDNSRecord{}).Where("dns_account_name = (SELECT name FROM dns_accounts WHERE id = ?)", c.Param("id")).Count(&count)
	}
	if count > 0 {
		requestModel.Error(c, 409, "账号仍被域名或快速解析记录引用，请先迁移相关数据", nil)
		return
	}
	if err := db.DB.Delete(&models.DNSAccount{}, c.Param("id")).Error; err != nil {
		requestModel.Error(c, 500, "删除失败", nil)
		return
	}
	requestModel.Success(c, "ok")
}
func TestDNSAccount(c *gin.Context) {
	account, err := db.GetDNSAccountByID(c.Param("id"))
	if err != nil {
		requestModel.NotFound(c, "DNS 账号不存在")
		return
	}
	provider, err := DDNS.NewBaseProvider(account)
	if err == nil {
		_, err = provider.GetDomainList(models.DomainsSearch{PageNumber: 1, PageSize: 1})
	}
	if err != nil {
		requestModel.Error(c, 502, "DNS 厂商连接失败", nil)
		return
	}
	requestModel.Success(c, gin.H{"reachable": true})
}
func SyncDNSAccount(c *gin.Context) {
	account, err := db.GetDNSAccountByID(c.Param("id"))
	if err != nil {
		requestModel.NotFound(c, "DNS 账号不存在")
		return
	}
	provider, err := DDNS.NewBaseProvider(account)
	if err != nil {
		requestModel.Error(c, 502, err.Error(), nil)
		return
	}
	list, err := provider.GetDomainList(models.DomainsSearch{PageNumber: 1, PageSize: 100})
	if err != nil {
		requestModel.Error(c, 502, "同步域名失败", nil)
		return
	}
	var row models.DNSAccount
	db.DB.Where("name = ?", account.Name).First(&row)
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		for _, domain := range list.Domains {
			domain.AccountName = account.Name
			domain.DnsFrom = account.Type
			domain.DNSAccountID = row.ID
			if e := tx.Where("account_name = ? AND dns_from = ? AND id = ?", domain.AccountName, domain.DnsFrom, domain.Id).Assign(domain).FirstOrCreate(&models.Domains{}).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		requestModel.Error(c, 500, "保存同步结果失败", nil)
		return
	}
	requestModel.Success(c, gin.H{"synced": len(list.Domains)})
}

func ACMEProfiles(c *gin.Context) {
	var rows []models.ACMEProfile
	db.DB.Order("is_default desc,id").Find(&rows)
	requestModel.Success(c, rows)
}
func SaveACMEProfile(c *gin.Context) {
	var row models.ACMEProfile
	if err := c.ShouldBindJSON(&row); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if row.ID == 0 && c.Param("id") != "" {
		parsed, e := strconv.ParseUint(c.Param("id"), 10, 64)
		if e != nil {
			requestModel.BadRequest(c, "ACME Profile ID 无效")
			return
		}
		row.ID = uint(parsed)
	}
	if _, err := mail.ParseAddress(row.Email); err != nil {
		requestModel.BadRequest(c, "邮箱无效")
		return
	}
	if row.Name == "" {
		row.Name = row.Email
	}
	if row.ID == 0 {
		row.Enabled = true
	}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if row.IsDefault {
			if e := tx.Model(&models.ACMEProfile{}).Where("id <> ?", row.ID).Update("is_default", false).Error; e != nil {
				return e
			}
		}
		return tx.Save(&row).Error
	})
	if err != nil {
		requestModel.Error(c, 409, "保存 ACME Profile 失败", nil)
		return
	}
	_ = db.LoadRuntimeConfig()
	u, _ := currentUser(c)
	audit(c, u.ID, "acme_profile.save", fmt.Sprint(row.ID), "success")
	requestModel.Success(c, row)
}
func DeleteACMEProfile(c *gin.Context) {
	var count int64
	db.DB.Model(&models.Certificate{}).Where("stage IN ?", []string{"wait", "challenging", "issued", "persisted"}).Count(&count)
	if count > 0 {
		requestModel.Error(c, 409, "存在运行中的证书任务", nil)
		return
	}
	db.DB.Delete(&models.ACMEProfile{}, c.Param("id"))
	_ = db.LoadRuntimeConfig()
	requestModel.Success(c, "ok")
}

func Settings(c *gin.Context) {
	group := c.Param("group")
	if group == "fast-ddns" {
		group = "fast"
	}
	prefix := group + ".%"
	var rows []models.SystemSetting
	db.DB.Where("key LIKE ?", prefix).Order("key").Find(&rows)
	for i := range rows {
		if rows[i].Sensitive {
			rows[i].Value = ""
		}
	}
	requestModel.Success(c, rows)
}
func SaveSettings(c *gin.Context) {
	group := c.Param("group")
	if group == "fast-ddns" {
		group = "fast"
	}
	if group != "certificate" && group != "fast" && group != "registration" {
		requestModel.BadRequest(c, "配置组无效")
		return
	}
	var values map[string]any
	if err := c.ShouldBindJSON(&values); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if group == "certificate" {
		if prefix, ok := values["apply_prefix"]; ok {
			normalizedPrefix, prefixErr := certificate.NormalizeDelegationPrefix(fmt.Sprint(prefix))
			if prefixErr != nil {
				requestModel.BadRequest(c, prefixErr.Error())
				return
			}
			values["apply_prefix"] = normalizedPrefix
		}
		account, hasAccount := values["apply_account"]
		domainID, hasDomainID := values["apply_domain_id"]
		domainName, hasDomainName := values["apply_domain_name"]
		if hasAccount || hasDomainID || hasDomainName {
			accountName := strings.TrimSpace(fmt.Sprint(account))
			providerID := strings.TrimSpace(fmt.Sprint(domainID))
			fqdn, normalizeErr := normalizeFQDN(fmt.Sprint(domainName))
			if accountName == "" || providerID == "" || normalizeErr != nil {
				requestModel.BadRequest(c, "委托 DNS 账号、DomainId 和承载域名必须完整且有效")
				return
			}
			var count int64
			if err := db.DB.Model(&models.DNSAccount{}).Where("name = ? AND enabled = ?", accountName, true).Count(&count).Error; err != nil || count != 1 {
				requestModel.BadRequest(c, "委托 DNS 账号不存在或已停用")
				return
			}
			values["apply_account"] = accountName
			values["apply_domain_id"] = providerID
			values["apply_domain_name"] = fqdn
		}
	}
	if group == "fast" {
		if salt, ok := values["access_salt"]; ok {
			normalized := strings.TrimSpace(fmt.Sprint(salt))
			if len(normalized) < 16 {
				requestModel.BadRequest(c, "AccessSalt 至少需要 16 个字符")
				return
			}
			values["access_salt"] = normalized
		}
		allowed := map[string]bool{"access_salt": true, "use_account": true, "domain_id": true, "domain_name": true, "name_strata": true, "id_length": true, "start_id": true}
		for key := range values {
			if !allowed[key] {
				requestModel.BadRequest(c, "未知的快速解析配置项: "+key)
				return
			}
		}
		hasFastConfig := false
		for _, key := range []string{"use_account", "domain_id", "domain_name", "name_strata", "id_length", "start_id"} {
			if _, ok := values[key]; ok {
				hasFastConfig = true
			}
		}
		if hasFastConfig {
			accountName := strings.TrimSpace(fmt.Sprint(values["use_account"]))
			domainID := strings.TrimSpace(fmt.Sprint(values["domain_id"]))
			domainName, domainErr := normalizeFQDN(fmt.Sprint(values["domain_name"]))
			if accountName == "" || domainID == "" || domainErr != nil {
				requestModel.BadRequest(c, "快速解析 DNS 账号、DomainId 和承载域名必须完整且有效")
				return
			}
			var count int64
			if err := db.DB.Model(&models.DNSAccount{}).Where("name = ? AND enabled = ?", accountName, true).Count(&count).Error; err != nil || count != 1 {
				requestModel.BadRequest(c, "快速解析 DNS 账号不存在或已停用")
				return
			}
			prefix := strings.TrimSpace(fmt.Sprint(values["name_strata"]))
			if prefix == "" || !validFastRecordName(prefix) || strings.Contains(prefix, ".") {
				requestModel.BadRequest(c, "快速解析记录前缀格式不正确")
				return
			}
			idLength, idErr := strconv.Atoi(fmt.Sprint(values["id_length"]))
			startID, startErr := strconv.Atoi(fmt.Sprint(values["start_id"]))
			if idErr != nil || idLength < 1 || idLength > 12 || startErr != nil || startID < 0 {
				requestModel.BadRequest(c, "编号长度必须为 1-12，起始编号不能为负数")
				return
			}
			values["use_account"], values["domain_id"], values["domain_name"] = accountName, domainID, domainName
			values["name_strata"], values["id_length"], values["start_id"] = prefix, idLength, startID
		}
	}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			full := group + "." + key
			raw := fmt.Sprint(value)
			sensitive := full == "fast.access_salt"
			if sensitive {
				var e error
				raw, e = db.EncryptSecret(raw)
				if e != nil {
					return e
				}
			}
			if e := tx.Where(models.SystemSetting{Key: full}).Assign(models.SystemSetting{Value: raw, Sensitive: sensitive}).FirstOrCreate(&models.SystemSetting{}).Error; e != nil {
				return e
			}
		}
		if group == "fast" {
			if start, ok := values["start_id"]; ok {
				var recordCount int64
				if e := tx.Model(&models.FastDDNSRecord{}).Count(&recordCount).Error; e != nil {
					return e
				}
				if recordCount == 0 {
					if e := tx.Where(models.SystemSetting{Key: "fast.next_id"}).Assign(models.SystemSetting{Value: fmt.Sprint(start)}).FirstOrCreate(&models.SystemSetting{}).Error; e != nil {
						return e
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		requestModel.Error(c, 500, "保存设置失败", nil)
		return
	}
	if err = db.LoadRuntimeConfig(); err != nil {
		requestModel.Error(c, 500, "刷新运行配置失败", nil)
		return
	}
	if group == "certificate" {
		if err = certificate.ReloadTaskProcessor(); err != nil {
			requestModel.Error(c, 503, "设置已保存，但 worker 重载失败", nil)
			return
		}
	}
	u, _ := currentUser(c)
	audit(c, u.ID, "settings.save", group, "success")
	requestModel.Success(c, "ok")
}

func DomainScopes(c *gin.Context) {
	var scopes []models.DomainScope
	u, _ := currentUser(c)
	query := db.DB.Order("zone_name,fqdn")
	if u.Role != "admin" {
		query = query.Where("id IN (SELECT scope_id FROM domain_grants WHERE user_id = ?)", u.ID)
	}
	query.Find(&scopes)
	requestModel.Success(c, scopes)
}
func SaveDomainScope(c *gin.Context) {
	var scope models.DomainScope
	if err := c.ShouldBindJSON(&scope); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	fqdn, err := normalizeFQDN(scope.FQDN)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	zone, err := normalizeFQDN(scope.ZoneName)
	if err != nil || !(fqdn == zone || strings.HasSuffix(fqdn, "."+zone)) {
		requestModel.BadRequest(c, "授权节点必须位于真实 Zone 内")
		return
	}
	scope.FQDN = fqdn
	scope.ZoneName = zone
	if scope.ID == 0 {
		scope.InheritChildren = true
	}
	if err = db.DB.Save(&scope).Error; err != nil {
		requestModel.Error(c, 409, "保存授权节点失败", nil)
		return
	}
	requestModel.Success(c, scope)
}
func DeleteDomainScope(c *gin.Context) {
	var grants int64
	if err := db.DB.Model(&models.DomainGrant{}).Where("scope_id = ?", c.Param("id")).Count(&grants).Error; err != nil {
		requestModel.Error(c, 500, "检查节点引用失败", nil)
		return
	}
	if grants > 0 {
		requestModel.Error(c, 409, "域名节点仍有用户授权，请先撤销授权", nil)
		return
	}
	result := db.DB.Delete(&models.DomainScope{}, c.Param("id"))
	if result.Error != nil {
		requestModel.Error(c, 500, "删除域名节点失败", nil)
		return
	}
	if result.RowsAffected == 0 {
		requestModel.NotFound(c, "域名节点不存在")
		return
	}
	user, _ := currentUser(c)
	audit(c, user.ID, "domain_scope.delete", c.Param("id"), "success")
	requestModel.Success(c, "ok")
}
func DomainGrants(c *gin.Context) {
	var grants []models.DomainGrant
	u, _ := currentUser(c)
	query := db.DB.Order("id")
	if u.Role != "admin" {
		query = query.Where("user_id = ? OR scope_id IN (SELECT scope_id FROM domain_grants WHERE user_id = ? AND level = 'owner')", u.ID, u.ID)
	}
	query.Find(&grants)
	requestModel.Success(c, grants)
}
func SaveDomainGrant(c *gin.Context) {
	var grant models.DomainGrant
	if err := c.ShouldBindJSON(&grant); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if permissionRank(grant.Level) < 1 {
		requestModel.BadRequest(c, "权限级别无效")
		return
	}
	u, _ := currentUser(c)
	if u.Role != "admin" {
		var n int64
		db.DB.Model(&models.DomainGrant{}).Where("user_id = ? AND scope_id = ? AND level = 'owner'", u.ID, grant.ScopeID).Count(&n)
		if n == 0 {
			requestModel.Forbidden(c, "需要该域名节点的 owner 权限")
			return
		}
	}
	if err := db.DB.Where(models.DomainGrant{UserID: grant.UserID, ScopeID: grant.ScopeID}).Assign(models.DomainGrant{Level: grant.Level}).FirstOrCreate(&grant).Error; err != nil {
		requestModel.Error(c, 409, "保存授权失败", nil)
		return
	}
	audit(c, u.ID, "domain_grant.save", fmt.Sprint(grant.ID), "success")
	requestModel.Success(c, grant)
}
func DeleteDomainGrant(c *gin.Context) {
	u, _ := currentUser(c)
	var grant models.DomainGrant
	if db.DB.First(&grant, c.Param("id")).Error != nil {
		requestModel.NotFound(c, "授权不存在")
		return
	}
	if u.Role != "admin" {
		var n int64
		db.DB.Model(&models.DomainGrant{}).Where("user_id = ? AND scope_id = ? AND level = 'owner'", u.ID, grant.ScopeID).Count(&n)
		if n == 0 {
			requestModel.Forbidden(c, "需要该域名节点的 owner 权限")
			return
		}
	}
	db.DB.Delete(&grant)
	audit(c, u.ID, "domain_grant.delete", fmt.Sprint(grant.ID), "success")
	requestModel.Success(c, "ok")
}

func normalizeFQDN(v string) (string, error) {
	return certificate.NormalizeDomain(v)
}
func permissionRank(v string) int {
	return map[string]int{"viewer": 1, "dns_editor": 2, "certificate_manager": 3, "owner": 4}[v]
}

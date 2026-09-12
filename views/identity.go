package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const sessionCookie = "domainsprite_session"

var bootstrap struct {
	sync.Mutex
	hash    string
	expires time.Time
}

func InitIdentity() error {
	has, err := db.HasUsers()
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	token, err := db.RandomToken(32)
	if err != nil {
		return err
	}
	bootstrap.Lock()
	bootstrap.hash = db.HashToken(token)
	bootstrap.expires = time.Now().Add(30 * time.Minute)
	bootstrap.Unlock()
	log.Printf("首次初始化 Setup Token（30 分钟有效，仅使用一次）: %s", token)
	return nil
}

func SetupStatus(c *gin.Context) {
	has, err := db.HasUsers()
	if err != nil {
		requestModel.Error(c, 500, "读取初始化状态失败", nil)
		return
	}
	requestModel.Success(c, gin.H{"required": !has})
}

type credentialsRequest struct {
	Username   string `json:"username" binding:"required,min=3,max=128"`
	Password   string `json:"password" binding:"required,min=10,max=256"`
	SetupToken string `json:"setupToken"`
}

func Setup(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	has, _ := db.HasUsers()
	if has {
		requestModel.Error(c, 409, "系统已完成初始化", nil)
		return
	}
	bootstrap.Lock()
	valid := time.Now().Before(bootstrap.expires) && constantEqual(db.HashToken(req.SetupToken), bootstrap.hash)
	if valid {
		bootstrap.hash = ""
	}
	bootstrap.Unlock()
	if !valid {
		requestModel.Forbidden(c, "Setup Token 无效或已过期")
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	user := models.User{Username: strings.TrimSpace(req.Username), PasswordHash: hash, Role: "admin", Enabled: true}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&user).Error; e != nil {
			return e
		}
		if models.AccountConfig.BaseConfig.AccessKeyId != "" && models.AccountConfig.BaseConfig.AccessKeySecret != "" {
			return tx.Create(&models.AccessKey{UserID: user.ID, Name: "Legacy migrated key", KeyID: models.AccountConfig.BaseConfig.AccessKeyId, SecretHash: db.HashToken(models.AccountConfig.BaseConfig.AccessKeySecret), Scopes: "dns:read,dns:write,certificate:issue,certificate:renew,certificate:download", Enabled: true}).Error
		}
		return nil
	})
	if err != nil {
		requestModel.Error(c, 500, "创建管理员失败", nil)
		return
	}
	audit(c, user.ID, "setup", "user:"+req.Username, "success")
	requestModel.Success(c, gin.H{"user": user})
}

func Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	var user models.User
	if err := db.DB.Where("username = ?", strings.TrimSpace(req.Username)).First(&user).Error; err != nil || !user.Enabled || !db.VerifyPassword(user.PasswordHash, req.Password) {
		requestModel.Unauthorized(c, "用户名或密码错误")
		return
	}
	token, _ := db.RandomToken(32)
	csrf, _ := db.RandomToken(32)
	now := time.Now()
	session := models.WebSession{UserID: user.ID, TokenHash: db.HashToken(token), CSRFHash: db.HashToken(csrf), ExpiresAt: now.Add(12 * time.Hour), LastActiveAt: now}
	if err := db.DB.Create(&session).Error; err != nil {
		requestModel.Error(c, 500, "创建会话失败", nil)
		return
	}
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, token, int((12 * time.Hour).Seconds()), "/", "", secure, true)
	db.DB.Model(&user).Update("last_login_at", now)
	audit(c, user.ID, "login", "session", "success")
	requestModel.Success(c, gin.H{"user": user, "csrfToken": csrf})
}
func SessionInfo(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		requestModel.Unauthorized(c, "会话无效")
		return
	}
	csrf, err := db.RandomToken(32)
	if err != nil {
		requestModel.Error(c, 500, "刷新 CSRF token 失败", nil)
		return
	}
	raw, _ := c.Cookie(sessionCookie)
	db.DB.Model(&models.WebSession{}).Where("token_hash = ?", db.HashToken(raw)).Update("csrf_hash", db.HashToken(csrf))
	requestModel.Success(c, gin.H{"user": user, "csrfToken": csrf})
}
func Logout(c *gin.Context) {
	if raw, e := c.Cookie(sessionCookie); e == nil {
		db.DB.Where("token_hash = ?", db.HashToken(raw)).Delete(&models.WebSession{})
	}
	c.SetCookie(sessionCookie, "", -1, "/", "", c.Request.TLS != nil, true)
	requestModel.Success(c, "ok")
}
func ChangePassword(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		requestModel.Unauthorized(c, "会话无效")
		return
	}
	var req struct {
		Current  string `json:"current" binding:"required"`
		Password string `json:"password" binding:"required,min=10,max=256"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if !db.VerifyPassword(user.PasswordHash, req.Current) {
		requestModel.Forbidden(c, "当前密码错误")
		return
	}
	hash, err := db.HashPassword(req.Password)
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	db.DB.Model(&user).Updates(map[string]any{"password_hash": hash, "must_change_password": false})
	db.DB.Where("user_id = ?", user.ID).Delete(&models.WebSession{})
	c.SetCookie(sessionCookie, "", -1, "/", "", c.Request.TLS != nil, true)
	audit(c, user.ID, "password.change", "user", "success")
	requestModel.Success(c, "ok")
}

func currentUser(c *gin.Context) (models.User, bool) {
	if value, ok := c.Get("current_user"); ok {
		return value.(models.User), true
	}
	raw, err := c.Cookie(sessionCookie)
	if err != nil {
		return models.User{}, false
	}
	var session models.WebSession
	if err = db.DB.Where("token_hash = ? AND expires_at > ?", db.HashToken(raw), time.Now()).First(&session).Error; err != nil {
		return models.User{}, false
	}
	var user models.User
	if err = db.DB.First(&user, session.UserID).Error; err != nil || !user.Enabled {
		return models.User{}, false
	}
	db.DB.Model(&session).Update("last_active_at", time.Now())
	c.Set("current_user", user)
	c.Set("auth_method", "session")
	return user, true
}
func constantEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func parseScopes(value string) map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Split(value, ",") {
		m[strings.TrimSpace(s)] = true
	}
	return m
}
func authenticateAccessKey(c *gin.Context) (models.User, bool) {
	id, secret := c.GetHeader("AccessKeyId"), c.GetHeader("AccessKeySecret")
	if id == "" || secret == "" {
		return models.User{}, false
	}
	var key models.AccessKey
	if err := db.DB.Where("key_id = ? AND enabled = ? AND revoked_at IS NULL", id, true).First(&key).Error; err != nil {
		return models.User{}, false
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) || !constantEqual(key.SecretHash, db.HashToken(secret)) {
		return models.User{}, false
	}
	var user models.User
	if err := db.DB.First(&user, key.UserID).Error; err != nil || !user.Enabled {
		return models.User{}, false
	}
	now := time.Now()
	db.DB.Model(&key).Update("last_used_at", now)
	c.Set("current_user", user)
	c.Set("access_key", key)
	c.Set("auth_method", "access_key")
	return user, true
}

func IdentityAuthentication(c *gin.Context) {
	user, ok := authenticateAccessKey(c)
	if !ok {
		user, ok = currentUser(c)
	}
	if !ok {
		requestModel.Unauthorized(c, "认证失败")
		c.Abort()
		return
	}
	if c.Request.Method != "GET" && c.GetString("auth_method") == "session" {
		csrf := c.GetHeader("X-CSRF-Token")
		raw, _ := c.Cookie(sessionCookie)
		var session models.WebSession
		if csrf == "" || db.DB.Where("token_hash = ? AND csrf_hash = ?", db.HashToken(raw), db.HashToken(csrf)).First(&session).Error != nil {
			requestModel.Forbidden(c, "CSRF token 无效")
			c.Abort()
			return
		}
	}
	c.Set("current_user", user)
	c.Next()
}
func RequireAdmin(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user.Role != "admin" {
		requestModel.Forbidden(c, "需要管理员权限")
		c.Abort()
		return
	}
	c.Next()
}
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if value, ok := c.Get("access_key"); ok && !parseScopes(value.(models.AccessKey).Scopes)[scope] {
			requestModel.Forbidden(c, "AccessKey 缺少权限: "+scope)
			c.Abort()
			return
		}
		c.Next()
	}
}
func audit(c *gin.Context, userID uint, action, resource, result string) {
	requestID, _ := c.Get("request_id")
	_ = db.DB.Create(&models.AuditLog{UserID: userID, AuthMethod: c.GetString("auth_method"), Action: action, Resource: resource, Result: result, RequestID: toString(requestID)}).Error
}
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = errors.Is

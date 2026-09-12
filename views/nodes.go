package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Nodes(c *gin.Context) {
	user, _ := currentUser(c)
	q := db.DB.Order("status desc,name")
	if user.Role != "admin" {
		q = q.Where("group_id IN (SELECT group_id FROM user_node_groups WHERE user_id = ?)", user.ID)
	}
	var nodes []models.CertificateNode
	q.Find(&nodes)
	requestModel.Success(c, nodes)
}
func UpdateNode(c *gin.Context) {
	var req struct {
		Remark  *string `json:"remark"`
		GroupID *uint   `json:"groupId"`
		Enabled *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	updates := map[string]any{}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}
	if req.GroupID != nil {
		updates["group_id"] = *req.GroupID
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	result := db.DB.Model(&models.CertificateNode{}).Where("id = ?", c.Param("id")).Updates(updates)
	if result.RowsAffected == 0 {
		requestModel.NotFound(c, "节点不存在")
		return
	}
	requestModel.Success(c, "ok")
}
func NodeAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		updates := map[string]any{}
		switch action {
		case "drain":
			updates["draining"] = true
		case "resume":
			updates["draining"] = false
		case "revoke":
			now := time.Now()
			updates["enabled"] = false
			updates["revoked_at"] = now
			updates["status"] = "revoked"
		}
		result := db.DB.Model(&models.CertificateNode{}).Where("id = ?", c.Param("id")).Updates(updates)
		if result.RowsAffected == 0 {
			requestModel.NotFound(c, "节点不存在")
			return
		}
		requestModel.Success(c, "ok")
	}
}
func DeleteNode(c *gin.Context) {
	var active int64
	db.DB.Model(&models.NodeTaskLease{}).Where("node_id = ? AND stage NOT IN ?", c.Param("id"), []string{"success", "fail"}).Count(&active)
	if active > 0 {
		requestModel.Error(c, 409, "节点仍有活动任务", nil)
		return
	}
	db.DB.Delete(&models.CertificateNode{}, c.Param("id"))
	requestModel.Success(c, "ok")
}

func RegistrationTokens(c *gin.Context) {
	var rows []models.NodeRegistrationToken
	db.DB.Order("id desc").Limit(100).Find(&rows)
	requestModel.Success(c, rows)
}
func CreateRegistrationToken(c *gin.Context) {
	var req struct {
		GroupID        uint `json:"groupId" binding:"required"`
		ExpiresMinutes int  `json:"expiresMinutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if req.ExpiresMinutes < 1 {
		req.ExpiresMinutes = 10
	}
	if req.ExpiresMinutes > 1440 {
		requestModel.BadRequest(c, "有效期不能超过 24 小时")
		return
	}
	var group models.NodeGroup
	if db.DB.First(&group, req.GroupID).Error != nil || !group.Enabled {
		requestModel.BadRequest(c, "节点组不可用")
		return
	}
	token, _ := db.RandomToken(32)
	tail := token
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	user, _ := currentUser(c)
	row := models.NodeRegistrationToken{TokenHash: db.HashToken(token), TokenTail: tail, GroupID: req.GroupID, CreatedBy: user.ID, ExpiresAt: time.Now().Add(time.Duration(req.ExpiresMinutes) * time.Minute)}
	if err := db.DB.Create(&row).Error; err != nil {
		requestModel.Error(c, 500, "创建注册凭证失败", nil)
		return
	}
	controller := models.AccountConfig.GRPC.PublicURL
	quoted := url.QueryEscape(controller)
	linux := fmt.Sprintf("sudo ./DomainSpriteNode install --name '<name>' --remark '<remark>' --controller '%s' --token '%s'", controller, token)
	windows := fmt.Sprintf(".\\DomainSpriteNode.exe install --name \"<name>\" --remark \"<remark>\" --controller \"%s\" --token \"%s\"", controller, token)
	requestModel.Success(c, gin.H{"registrationToken": row, "token": token, "commands": gin.H{"linux": linux, "windows": windows}, "controllerEncoded": quoted})
}
func RevokeRegistrationToken(c *gin.Context) {
	now := time.Now()
	db.DB.Model(&models.NodeRegistrationToken{}).Where("id = ? AND used_at IS NULL", c.Param("id")).Update("revoked_at", now)
	requestModel.Success(c, "ok")
}

func NodeGroups(c *gin.Context) {
	var groups []models.NodeGroup
	db.DB.Order("name").Find(&groups)
	requestModel.Success(c, groups)
}
func SaveNodeGroup(c *gin.Context) {
	var row models.NodeGroup
	if err := c.ShouldBindJSON(&row); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	if row.ID == 0 && c.Param("id") != "" {
		v, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		row.ID = uint(v)
	}
	if strings.TrimSpace(row.Name) == "" {
		requestModel.BadRequest(c, "名称不能为空")
		return
	}
	if row.ID == 0 {
		row.Enabled = true
	}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if row.IsDefault {
			if e := tx.Model(&models.NodeGroup{}).Where("id <> ?", row.ID).Update("is_default", false).Error; e != nil {
				return e
			}
		}
		return tx.Save(&row).Error
	})
	if err != nil {
		requestModel.Error(c, 409, "保存节点组失败", nil)
		return
	}
	requestModel.Success(c, row)
}
func DeleteNodeGroup(c *gin.Context) {
	var n int64
	db.DB.Model(&models.CertificateNode{}).Where("group_id = ?", c.Param("id")).Count(&n)
	if n > 0 {
		requestModel.Error(c, 409, "节点组仍包含节点", nil)
		return
	}
	db.DB.Delete(&models.NodeGroup{}, c.Param("id"))
	requestModel.Success(c, "ok")
}
func UserNodeGroups(c *gin.Context) {
	var rows []models.UserNodeGroup
	db.DB.Where("user_id = ?", c.Param("id")).Find(&rows)
	requestModel.Success(c, rows)
}
func SaveUserNodeGroups(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		requestModel.BadRequest(c, "用户 ID 无效")
		return
	}
	var req struct {
		GroupIDs []uint `json:"groupIds"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("user_id = ?", userID).Delete(&models.UserNodeGroup{}).Error; e != nil {
			return e
		}
		for _, id := range req.GroupIDs {
			if e := tx.Create(&models.UserNodeGroup{UserID: uint(userID), GroupID: id}).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		requestModel.Error(c, 409, "保存用户节点组失败", nil)
		return
	}
	requestModel.Success(c, "ok")
}
func NodeACMEPolicies(c *gin.Context) {
	var rows []models.NodeACMEPolicy
	db.DB.Where("node_id = ?", c.Param("id")).Find(&rows)
	requestModel.Success(c, rows)
}
func SaveNodeACMEPolicies(c *gin.Context) {
	nodeID, vErr := strconv.ParseUint(c.Param("id"), 10, 64)
	if vErr != nil {
		requestModel.BadRequest(c, "节点 ID 无效")
		return
	}
	var rows []models.NodeACMEPolicy
	if err := c.ShouldBindJSON(&rows); err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("node_id = ?", nodeID).Delete(&models.NodeACMEPolicy{}).Error; e != nil {
			return e
		}
		for _, row := range rows {
			row.ID = 0
			row.NodeID = uint(nodeID)
			if row.HourlyLimit < 0 || row.DailyLimit < 0 {
				return fmt.Errorf("频率限制不能为负数")
			}
			if e := tx.Create(&row).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		requestModel.BadRequest(c, err.Error())
		return
	}
	requestModel.Success(c, "ok")
}

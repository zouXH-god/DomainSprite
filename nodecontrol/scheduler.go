package nodecontrol

import (
	nodev1 "DDNSServer/api/node/v1"
	"DDNSServer/db"
	"DDNSServer/models"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNoEligibleNode = errors.New("没有可用的证书节点")
	ErrNodeQuota      = errors.New("证书节点邮箱额度已用尽")
)

type ScheduleRequest struct {
	TaskID        string
	UserID        uint
	GroupID       uint
	ACMEProfileID uint
	Providers     []string
	Payload       []byte
	LeaseDuration time.Duration
}

type Assignment struct {
	NodeID  uint
	GroupID uint
	LeaseID string
}

// Schedule selects an online node, atomically creates a lease and reserves
// hourly/daily CA quota before sending the task. A send failure rolls the
// reservation back so it cannot strand capacity.
func Schedule(req ScheduleRequest) (Assignment, error) {
	if req.TaskID == "" || req.UserID == 0 || req.ACMEProfileID == 0 {
		return Assignment{}, errors.New("task, user and ACME profile are required")
	}
	if req.LeaseDuration <= 0 {
		req.LeaseDuration = 2 * time.Minute
	}
	if err := ReapExpiredLeases(time.Now()); err != nil {
		return Assignment{}, err
	}
	groupIDs, err := allowedGroups(req.UserID, req.GroupID)
	if err != nil {
		return Assignment{}, err
	}
	cutoff := time.Now().Add(-30 * time.Second)
	var nodes []models.CertificateNode
	if err = db.DB.Where("group_id IN ? AND enabled = ? AND draining = ? AND status = ? AND last_heartbeat_at >= ?", groupIDs, true, false, "online", cutoff).Find(&nodes).Error; err != nil {
		return Assignment{}, err
	}
	wanted := make(map[string]bool, len(req.Providers))
	for _, provider := range req.Providers {
		wanted[strings.ToLower(provider)] = true
	}
	eligible := nodes[:0]
	for _, node := range nodes {
		if node.Running >= node.Capacity || !supports(node.Providers, wanted) {
			continue
		}
		eligible = append(eligible, node)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		left := float64(eligible[i].Running) / float64(max(eligible[i].Capacity, 1))
		right := float64(eligible[j].Running) / float64(max(eligible[j].Capacity, 1))
		if left != right {
			return left < right
		}
		if eligible[i].LastAssignedAt == nil {
			return true
		}
		if eligible[j].LastAssignedAt == nil {
			return false
		}
		return eligible[i].LastAssignedAt.Before(*eligible[j].LastAssignedAt)
	})
	if len(eligible) == 0 {
		return Assignment{}, ErrNoEligibleNode
	}

	var selected Assignment
	var lastErr error
	for _, candidate := range eligible {
		leaseID := uuid.NewString()
		now := time.Now()
		err = db.DB.Transaction(func(tx *gorm.DB) error {
			if e := reserveQuota(tx, candidate.ID, req.ACMEProfileID, now); e != nil {
				return e
			}
			lease := models.NodeTaskLease{TaskID: req.TaskID, NodeID: candidate.ID, GroupID: candidate.GroupID, ACMEProfileID: req.ACMEProfileID, LeaseID: leaseID, Stage: "assigned", ExpiresAt: now.Add(req.LeaseDuration)}
			if e := tx.Create(&lease).Error; e != nil {
				return e
			}
			result := tx.Model(&models.CertificateTask{}).Where("task_id = ? AND (lease_id = '' OR lease_id IS NULL)", req.TaskID).Updates(map[string]any{"node_id": candidate.ID, "node_group_id": candidate.GroupID, "lease_id": leaseID, "remote_stage": "assigned", "assigned_at": &now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("任务不存在或已有有效租约")
			}
			return tx.Model(&models.CertificateNode{}).Where("id = ?", candidate.ID).Update("last_assigned_at", &now).Error
		})
		if err != nil {
			lastErr = err
			continue
		}
		message := &nodev1.ControllerMessage{ProtocolVersion: ProtocolVersion, NodeId: uint64(candidate.ID), TaskId: req.TaskID, LeaseId: leaseID, TimestampUnix: now.Unix(), Payload: &nodev1.ControllerMessage_Assignment{Assignment: &nodev1.Assignment{EncryptedPayload: req.Payload, LeaseExpiresUnix: now.Add(req.LeaseDuration).Unix(), Stage: "assigned"}}}
		if err = DefaultHub.Send(uint64(candidate.ID), message); err != nil {
			_ = rollbackAssignment(req.TaskID, leaseID, candidate.ID, req.ACMEProfileID, now)
			lastErr = err
			continue
		}
		selected = Assignment{NodeID: candidate.ID, GroupID: candidate.GroupID, LeaseID: leaseID}
		return selected, nil
	}
	if lastErr != nil {
		return Assignment{}, fmt.Errorf("%w: %v", ErrNoEligibleNode, lastErr)
	}
	return Assignment{}, ErrNoEligibleNode
}

// ReapExpiredLeases only releases work that has not begun the CA flow.
// challenging/issued leases deliberately remain pinned to their original node.
func ReapExpiredLeases(now time.Time) error {
	var leases []models.NodeTaskLease
	if err := db.DB.Where("expires_at < ? AND stage IN ?", now, []string{"wait", "assigned", "rejected"}).Find(&leases).Error; err != nil {
		return err
	}
	for _, lease := range leases {
		if err := rollbackAssignment(lease.TaskID, lease.LeaseID, lease.NodeID, lease.ACMEProfileID, lease.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func allowedGroups(userID, requested uint) ([]uint, error) {
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.Role == "admin" {
		var ids []uint
		q := db.DB.Model(&models.NodeGroup{}).Where("enabled = ?", true)
		if requested != 0 {
			q = q.Where("id = ?", requested)
		}
		if err := q.Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, ErrNoEligibleNode
		}
		return ids, nil
	}
	var ids []uint
	q := db.DB.Model(&models.UserNodeGroup{}).Where("user_id = ?", userID)
	if requested != 0 {
		q = q.Where("group_id = ?", requested)
	}
	if err := q.Pluck("group_id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("用户无权使用该节点组")
	}
	return ids, nil
}

func reserveQuota(tx *gorm.DB, nodeID, profileID uint, now time.Time) error {
	var policy models.NodeACMEPolicy
	if err := tx.Where("node_id = ? AND acme_profile_id = ? AND enabled = ?", nodeID, profileID, true).First(&policy).Error; err != nil {
		return err
	}
	for _, bucket := range []struct {
		key   string
		limit int
	}{{"h:" + now.UTC().Format("2006010215"), policy.HourlyLimit}, {"d:" + now.UTC().Format("20060102"), policy.DailyLimit}} {
		if bucket.limit <= 0 {
			continue
		}
		var usage models.NodeRateUsage
		err := tx.Where("node_id = ? AND acme_profile_id = ? AND bucket = ?", nodeID, profileID, bucket.key).First(&usage).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			usage = models.NodeRateUsage{NodeID: nodeID, ACMEProfileID: profileID, Bucket: bucket.key}
			if err = tx.Create(&usage).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		result := tx.Model(&models.NodeRateUsage{}).Where("id = ? AND reserved + issued < ?", usage.ID, bucket.limit).Update("reserved", gorm.Expr("reserved + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNodeQuota
		}
	}
	return nil
}

func rollbackAssignment(taskID, leaseID string, nodeID, profileID uint, now time.Time) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ? AND lease_id = ?", taskID, leaseID).Delete(&models.NodeTaskLease{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.CertificateTask{}).Where("task_id = ? AND lease_id = ?", taskID, leaseID).Updates(map[string]any{"node_id": 0, "node_group_id": 0, "lease_id": "", "remote_stage": "wait", "assigned_at": nil}).Error; err != nil {
			return err
		}
		for _, key := range []string{"h:" + now.UTC().Format("2006010215"), "d:" + now.UTC().Format("20060102")} {
			if err := tx.Model(&models.NodeRateUsage{}).Where("node_id = ? AND acme_profile_id = ? AND bucket = ? AND reserved > 0", nodeID, profileID, key).Update("reserved", gorm.Expr("reserved - 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func supports(csv string, wanted map[string]bool) bool {
	available := map[string]bool{}
	for _, item := range strings.Split(csv, ",") {
		available[strings.ToLower(strings.TrimSpace(item))] = true
	}
	for item := range wanted {
		if !available[item] {
			return false
		}
	}
	return true
}

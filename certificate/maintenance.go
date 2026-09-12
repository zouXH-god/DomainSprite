package certificate

import (
	"DDNSServer/DDNS"
	"DDNSServer/db"
	"DDNSServer/models"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var maintenance struct {
	sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func StartMaintenance() {
	maintenance.Lock()
	defer maintenance.Unlock()
	if maintenance.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	maintenance.cancel = cancel
	maintenance.done = make(chan struct{})
	go func() {
		defer close(maintenance.done)
		runMaintenance()
		dailyTicker := time.NewTicker(24 * time.Hour)
		cleanupTicker := time.NewTicker(5 * time.Minute)
		defer dailyTicker.Stop()
		defer cleanupTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-dailyTicker.C:
				runMaintenance()
			case <-cleanupTicker.C:
				if err := RetryChallengeCleanup(); err != nil {
					slog.Error("DNS challenge 延迟清理失败", "error", err)
				}
			}
		}
	}()
}
func StopMaintenance() {
	maintenance.Lock()
	cancel, done := maintenance.cancel, maintenance.done
	maintenance.cancel = nil
	maintenance.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func runMaintenance() {
	if err := ScanRenewals(time.Now()); err != nil {
		slog.Error("自动续期扫描失败", "error", err)
	}
	if err := CleanupRetired(time.Now()); err != nil {
		slog.Error("旧证书清理失败", "error", err)
	}
	if err := CleanupStaging(time.Now()); err != nil {
		slog.Error("staging 清理失败", "error", err)
	}
	if err := RetryChallengeCleanup(); err != nil {
		slog.Error("DNS challenge 清理重试失败", "error", err)
	}
}

func CleanupStaging(now time.Time) error {
	root := filepath.Join(models.AccountConfig.Certificate.SavePath, "staging")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if now.Sub(info.ModTime()) < 24*time.Hour {
			continue
		}
		var task models.CertificateTask
		err = db.DB.Where("task_id = ?", entry.Name()).First(&task).Error
		if err == nil && task.State != "success" && task.State != "fail" {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func RetryChallengeCleanup() error {
	var rows []models.ChallengeCleanup
	if err := db.DB.Order("id").Limit(100).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		account, err := DDNS.GetAccount(row.AccountName)
		if err == nil {
			var provider models.RecordProvider
			provider, err = DDNS.NewBaseProvider(account)
			if err == nil {
				_, err = provider.DeleteRecord(row.DomainName, row.RecordID)
			}
		}
		if err == nil {
			if e := db.DB.Delete(&models.ChallengeCleanup{}, row.ID).Error; e != nil {
				return e
			}
		} else {
			if e := db.DB.Model(&models.ChallengeCleanup{}).Where("id = ?", row.ID).Updates(map[string]any{"attempts": row.Attempts + 1, "last_error": err.Error()}).Error; e != nil {
				return e
			}
		}
	}
	return nil
}

func ScanRenewals(now time.Time) error {
	var certs []models.Certificate
	deadline := now.Add(30 * 24 * time.Hour)
	if err := db.DB.Where("stage = ? AND retired_at IS NULL AND not_after <= ? AND EXISTS (SELECT 1 FROM certificate_domains cd WHERE cd.certificate_id = certificates.id)", "success", deadline).Find(&certs).Error; err != nil {
		return err
	}
	for _, cert := range certs {
		if _, err := EnqueueRenewal(cert.Id); err != nil && !strings.Contains(err.Error(), ErrRenewalActive.Error()) {
			slog.Error("自动续期入队失败", "certificate_id", cert.Id, "error", err)
		}
	}
	return nil
}

func CleanupRetired(now time.Time) error {
	var certs []models.Certificate
	cutoff := now.Add(-90 * 24 * time.Hour)
	if err := db.DB.Where("retired_at IS NOT NULL AND retired_at <= ? AND files_deleted_at IS NULL", cutoff).Find(&certs).Error; err != nil {
		return err
	}
	root, err := filepath.Abs(models.AccountConfig.Certificate.SavePath)
	if err != nil {
		return err
	}
	for _, cert := range certs {
		var refs int64
		if err := db.DB.Model(&models.Domains{}).Where("certificate_id = ?", cert.Id).Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 {
			continue
		}
		path, err := filepath.Abs(cert.SavePath)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("拒绝清理越界证书路径 %q", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		deleted := now
		if err := db.DB.Model(&models.Certificate{}).Where("id = ?", cert.Id).Update("files_deleted_at", &deleted).Error; err != nil {
			return err
		}
	}
	return nil
}

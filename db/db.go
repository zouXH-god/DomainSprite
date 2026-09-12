package db

import (
	"DDNSServer/models"
	"fmt"
	"github.com/glebarez/sqlite" // 替换为新的 SQLite 驱动
	"gorm.io/gorm"
	"io"
	"os"
	"time"
)

var DB = &gorm.DB{}

func InitDB() error {
	// 连接 SQLite 数据库
	db, err := gorm.Open(sqlite.Open("database.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	needsBackup := db.Migrator().HasTable(&models.Domains{}) && !db.Migrator().HasColumn(&models.Domains{}, "DBID")
	needsBackup = needsBackup || (db.Migrator().HasTable(&models.Certificate{}) && !db.Migrator().HasColumn(&models.Certificate{}, "LineageID"))
	if needsBackup {
		if err := backupDatabase("database.db"); err != nil {
			return err
		}
	}
	// 自动迁移（创建/更新表结构）
	err = db.AutoMigrate(&models.Domains{}, &models.Certificate{}, &models.CertificateTask{}, &models.CertificateDomain{}, &models.ChallengeCleanup{}, &models.User{}, &models.WebSession{}, &models.AccessKey{}, &models.DNSAccount{}, &models.ACMEProfile{}, &models.DomainScope{}, &models.DomainGrant{}, &models.SystemSetting{}, &models.AuditLog{}, &models.NodeGroup{}, &models.CertificateNode{}, &models.NodeRegistrationToken{}, &models.UserNodeGroup{}, &models.NodeACMEPolicy{}, &models.NodeRateUsage{}, &models.NodeTaskLease{})
	if err != nil {
		return err
	}
	if err = db.Exec("UPDATE certificates SET lineage_id = 'legacy-' || id WHERE lineage_id IS NULL OR lineage_id = ''").Error; err != nil {
		return err
	}
	if err = db.Exec("UPDATE certificates SET stage = state WHERE (stage IS NULL OR stage = '' OR stage = 'wait') AND state IN ('apply','success','fail')").Error; err != nil {
		return err
	}
	DB = db
	return nil
}

func backupDatabase(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开数据库备份源: %w", err)
	}
	defer src.Close()
	dstPath := path + ".bak." + time.Now().Format("20060102-150405")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("创建数据库备份: %w", err)
	}
	if _, err = io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err = dst.Sync(); err != nil {
		_ = dst.Close()
		return err
	}
	return dst.Close()
}

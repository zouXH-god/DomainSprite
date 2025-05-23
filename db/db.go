package db

import (
	"DDNSServer/models"
	"github.com/glebarez/sqlite" // 替换为新的 SQLite 驱动
	"gorm.io/gorm"
)

var DB = &gorm.DB{}

func InitDB() error {
	// 连接 SQLite 数据库
	db, err := gorm.Open(sqlite.Open("database.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	// 自动迁移（创建/更新表结构）
	err = db.AutoMigrate(&models.Domains{}, &models.Certificate{}, &models.CertificateTask{})
	if err != nil {
		return err
	}
	DB = db
	return nil
}

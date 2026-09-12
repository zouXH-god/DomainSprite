package main

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/nodecontrol"
	"DDNSServer/utils"
	"DDNSServer/views"
	"DDNSServer/webui"
	"context"
	_ "embed"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

//go:embed config.toml.example
var config string

func main() {
	created, err := utils.EnsureConfig("config.toml", config)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		keyCreated, keyPath, keyErr := utils.EnsureMasterKey("config.toml", "DOMAINSPRITE_MASTER_KEY")
		if keyErr != nil {
			log.Fatal(keyErr)
		}
		if keyCreated {
			log.Printf("已生成数据库主密钥并安全保存到 %s", keyPath)
		}
		log.Print("config.toml 已生成，请检查基础设施配置后重新启动")
		return
	}
	if err := models.LoadConfig("config.toml"); err != nil {
		log.Fatal(err)
	}
	keyCreated, keyPath, err := utils.EnsureMasterKey("config.toml", models.AccountConfig.BaseConfig.EncryptionKeyEnv)
	if err != nil {
		log.Fatal(err)
	}
	if keyCreated {
		log.Printf("环境变量 %s 未设置，已生成数据库主密钥并安全保存到 %s", models.AccountConfig.BaseConfig.EncryptionKeyEnv, keyPath)
	}
	if err := db.InitMasterKey(models.AccountConfig.BaseConfig.EncryptionKeyEnv); err != nil {
		log.Fatal(err)
	}
	if err := models.InitDataDirectories(); err != nil {
		log.Fatal(err)
	}
	if err := db.InitDB(); err != nil {
		log.Fatal(err)
	}
	legacyConfig := models.AccountConfig
	imported, err := db.ImportLegacyConfig(legacyConfig)
	if err != nil {
		log.Fatal(err)
	}
	if imported {
		if err := utils.SanitizeLegacyConfig("config.toml", legacyConfig); err != nil {
			log.Fatal(err)
		}
	}
	if err := db.LoadRuntimeConfig(); err != nil {
		log.Fatal(err)
	}
	db.InstallACMEStorage()
	if err := views.InitFastStore(); err != nil {
		log.Fatal(err)
	}
	if err := views.InitIdentity(); err != nil {
		log.Fatal(err)
	}
	if err := certificate.InitTaskProcessor(); err != nil {
		log.Fatal(err)
	}
	grpcServer, err := nodecontrol.NewServer(nodecontrol.DefaultHub)
	if err != nil {
		log.Fatal(err)
	}
	if err = grpcServer.Start(models.AccountConfig.GRPC); err != nil {
		log.Fatal(err)
	}
	certificate.StartMaintenance()
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})
	corsCfg := cors.Config{AllowOrigins: models.AccountConfig.BaseConfig.AllowedOrigins, AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization", "AccessKeyId", "AccessKeySecret", "AccessSalt", "X-Request-ID", "X-CSRF-Token"}, ExposeHeaders: []string{"Content-Length", "Content-Disposition", "Deprecation", "Sunset", "Link", "X-Request-ID"}, AllowCredentials: true, MaxAge: 12 * time.Hour}
	r.Use(cors.New(corsCfg))
	registerRoutes(r)
	webui.Register(r)
	srv := &http.Server{Addr: models.AccountConfig.BaseConfig.Host + ":" + models.AccountConfig.BaseConfig.Port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP 服务失败: %v", err)
		}
	case <-sigCh:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	certificate.StopMaintenance()
	certificate.ShutdownTaskProcessor()
	grpcServer.Stop()
	if sqlDB, err := db.DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

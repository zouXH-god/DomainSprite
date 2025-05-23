package main

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/utils"
	_ "embed"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

//go:embed config.toml.example
var config string

func main() {
	gin.Logger()
	if utils.InitConfig(config) {
		return
	}
	// 启用证书任务
	go certificate.StartTaskProcessor()
	// 初始化数据库
	err := db.InitDB()
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()

	// 定义CORS配置
	CORSConfig := cors.Config{
		AllowAllOrigins:  true, // 允许所有源
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(CORSConfig))

	// 注册路由
	registerRoutes(r)

	// 启动服务器
	err = r.Run(models.AccountConfig.BaseConfig.Host + ":" + models.AccountConfig.BaseConfig.Port)
	if err != nil {
		log.Fatal(err)
		return
	}
}

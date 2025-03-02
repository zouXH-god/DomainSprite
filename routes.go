package main

import (
	"DDNSServer/views"
	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine) {
	api := r.Group("/api", views.ApiAuthentication)
	{
		// 获取账户列表
		api.GET("/accounts", views.GetAccounts)
		// 获取域名列表
		api.GET("/:accountName/domains", views.GetDomains)
		// 获取域名解析记录列表
		api.GET("/:accountName/records", views.GetRecords)
		// 获取域名解析记录信息
		api.GET("/:accountName/record", views.GetRecordInfo)
		// 添加域名解析记录
		api.POST("/:accountName/record", views.AddRecord)
		// 修改域名解析记录
		api.PUT("/:accountName/record", views.UpdateRecord)
		// 删除域名解析记录
		api.DELETE("/:accountName/record", views.DeleteRecord)
		// 修改域名解析记录状态
		api.PUT("/:accountName/record/status", views.SetRecordStatus)
		// 为域名申请通配符证书
		api.POST("/:accountName/certificate", views.CreateCertificateView)
	}
	// 快速请求
	fastRequest := r.Group("/fast")
	{
		// 创建一个A解析并返回快速解析token
		fastRequest.GET("/ip2a", views.FastAuthentication, views.IpToDomainRecord)
		// 对指定的解析进行更新
		fastRequest.GET("/updateRecord", views.UpdateForToken)
	}
}

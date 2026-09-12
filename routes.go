package main

import (
	"DDNSServer/views"
	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine) {
	r.GET("/health", views.PublicHealth)
	r.GET("/auth/setup-status", views.SetupStatus)
	r.POST("/auth/setup", views.Setup)
	r.POST("/auth/register", views.Register)
	r.POST("/auth/login", views.Login)
	r.GET("/auth/session", views.IdentityAuthentication, views.SessionInfo)
	r.POST("/auth/logout", views.IdentityAuthentication, views.Logout)
	r.PUT("/auth/password", views.IdentityAuthentication, views.ChangePassword)
	api := r.Group("/api", views.ApiAuthentication)
	{
		api.GET("/access-keys", views.AccessKeys)
		api.POST("/access-keys", views.CreateAccessKey)
		api.DELETE("/access-keys/:id", views.RevokeAccessKey)
		api.POST("/access-keys/:id/rotate", views.RotateAccessKey)
		admin := api.Group("/admin", views.RequireAdmin)
		admin.GET("/users", views.Users)
		admin.POST("/users", views.CreateUser)
		admin.PUT("/users/:id", views.UpdateUser)
		admin.DELETE("/users/:id/sessions", views.RevokeUserSessions)
		api.GET("/nodes", views.Nodes)
		api.PUT("/nodes/:id", views.RequireAdmin, views.UpdateNode)
		api.DELETE("/nodes/:id", views.RequireAdmin, views.DeleteNode)
		api.POST("/nodes/:id/drain", views.RequireAdmin, views.NodeAction("drain"))
		api.POST("/nodes/:id/resume", views.RequireAdmin, views.NodeAction("resume"))
		api.POST("/nodes/:id/revoke", views.RequireAdmin, views.NodeAction("revoke"))
		api.GET("/nodes/registration-tokens", views.RequireAdmin, views.RegistrationTokens)
		api.POST("/nodes/registration-tokens", views.RequireAdmin, views.CreateRegistrationToken)
		api.DELETE("/nodes/registration-tokens/:id", views.RequireAdmin, views.RevokeRegistrationToken)
		api.GET("/node-groups", views.NodeGroups)
		api.POST("/node-groups", views.RequireAdmin, views.SaveNodeGroup)
		api.PUT("/node-groups/:id", views.RequireAdmin, views.SaveNodeGroup)
		api.DELETE("/node-groups/:id", views.RequireAdmin, views.DeleteNodeGroup)
		api.GET("/users/:id/node-groups", views.RequireAdmin, views.UserNodeGroups)
		api.PUT("/users/:id/node-groups", views.RequireAdmin, views.SaveUserNodeGroups)
		api.GET("/nodes/:id/acme-policies", views.RequireAdmin, views.NodeACMEPolicies)
		api.PUT("/nodes/:id/acme-policies", views.RequireAdmin, views.SaveNodeACMEPolicies)
		api.GET("/dns-accounts", views.RequireAdmin, views.DNSAccounts)
		api.POST("/dns-accounts", views.RequireAdmin, views.SaveDNSAccount)
		api.PUT("/dns-accounts/:id", views.RequireAdmin, views.SaveDNSAccount)
		api.DELETE("/dns-accounts/:id", views.RequireAdmin, views.DeleteDNSAccount)
		api.POST("/dns-accounts/:id/test", views.RequireAdmin, views.TestDNSAccount)
		api.POST("/dns-accounts/:id/sync", views.RequireAdmin, views.SyncDNSAccount)
		api.GET("/acme-profiles", views.RequireAdmin, views.ACMEProfiles)
		api.POST("/acme-profiles", views.RequireAdmin, views.SaveACMEProfile)
		api.PUT("/acme-profiles/:id", views.RequireAdmin, views.SaveACMEProfile)
		api.DELETE("/acme-profiles/:id", views.RequireAdmin, views.DeleteACMEProfile)
		api.GET("/settings/:group", views.RequireAdmin, views.Settings)
		api.PUT("/settings/:group", views.RequireAdmin, views.SaveSettings)
		api.GET("/domain-scopes", views.DomainScopes)
		api.POST("/domain-scopes", views.RequireAdmin, views.SaveDomainScope)
		api.PUT("/domain-scopes/:id", views.RequireAdmin, views.SaveDomainScope)
		api.DELETE("/domain-scopes/:id", views.RequireAdmin, views.DeleteDomainScope)
		api.GET("/domain-grants", views.DomainGrants)
		api.POST("/domain-grants", views.SaveDomainGrant)
		api.DELETE("/domain-grants/:id", views.DeleteDomainGrant)
		// 获取账户列表
		api.GET("/accounts", views.GetAccounts)
		api.GET("/meta", views.GetMeta)
		api.GET("/dashboard", views.GetDashboard)
		api.GET("/health", views.PrivateHealth)
		// 获取域名列表
		api.GET("/:accountName/domains", views.RequireScope("dns:read"), views.GetDomains)
		// 获取域名解析记录列表
		api.GET("/:accountName/records", views.RequireScope("dns:read"), views.GetRecords)
		// 获取域名解析记录信息
		api.GET("/:accountName/record", views.RequireScope("dns:read"), views.GetRecordInfo)
		// 添加域名解析记录
		api.POST("/:accountName/record", views.RequireScope("dns:write"), views.AddRecord)
		// 修改域名解析记录
		api.PUT("/:accountName/record", views.RequireScope("dns:write"), views.UpdateRecord)
		// 删除域名解析记录
		api.DELETE("/:accountName/record", views.RequireScope("dns:write"), views.DeleteRecord)
		// 修改域名解析记录状态
		api.PUT("/:accountName/record/status", views.RequireScope("dns:write"), views.SetRecordStatus)
		// 为域名申请通配符证书
		api.POST("/:accountName/certificate", views.RequireScope("certificate:issue"), views.CreateCertificateView)
	}
	// 证书管理
	certificate := r.Group("/certificate", views.ApiAuthentication)
	{
		// 获取证书列表
		certificate.GET("/list", views.GetCertificateListView)
		certificate.GET("/page", views.GetCertificatePage)
		certificate.GET("/tasks", views.GetCertificateTasks)
		certificate.GET("/task/:taskId", views.GetCertificateTask)
		certificate.POST("/apply/check-cname", views.RequireScope("certificate:issue"), views.CheckCNAME)
		// 前置申请证书
		certificate.GET("/apply", views.RequireScope("certificate:issue"), views.GetCertificateViewWithDomainInfo)
		// 提交申请证书
		certificate.POST("/apply", views.RequireScope("certificate:issue"), views.CreateCertificateViewWithDomainInfo)
		certificate.POST("/apply/accounts", views.RequireScope("certificate:issue"), views.CreateMultiAccountCertificateView)
		certificate.POST("/:id/renew", views.RequireScope("certificate:renew"), views.RenewCertificateView)
		certificate.GET("/:id/content", views.RequireScope("certificate:download"), views.CertificateContent)
		certificate.DELETE("/:id", views.RequireScope("certificate:renew"), views.DeleteCertificate)
		certificate.GET("/:id", views.GetCertificateDetail)
		// 获取证书信息
		certificate.GET("/info", views.GetCertificateViewWithId)
		// 下载证书
		certificate.GET("/download", views.RequireScope("certificate:download"), views.DownloadCertificateViewWithId)
		// 获取证书任务列表
		certificate.GET("/task", views.GetCertificateTaskInfoByCertificateId)
		// 获取证书任务日志
		certificate.GET("/task/log", views.GetTaskLog)
		certificate.GET("/task/:taskId/log", views.GetTaskLogByTaskID)
	}
	// 快速请求
	fastRequest := r.Group("/fast")
	{
		// 创建一个A解析并返回快速解析token
		fastRequest.POST("/ip2a", views.FastAuthentication, views.IpToDomainRecord)
		fastRequest.PUT("/record", views.UpdateForToken)
		fastRequest.GET("/ip2a", views.FastAuthentication, views.LegacyIpToDomainRecord)
		// 对指定的解析进行更新
		fastRequest.GET("/updateRecord", views.LegacyUpdateForToken)
	}
}

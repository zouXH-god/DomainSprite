package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"github.com/gin-gonic/gin"
)

// CreateCertificateView 申请证书
func CreateCertificateView(c *gin.Context) {
	domainId := c.PostForm("domainId")
	renew := c.PostForm("renew") == "true"
	domain, err := db.GetDomainForId(domainId)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	provider, err := getProvider(c)
	if err != nil {
		return
	}
	// 申请证书
	if renew {
		// 查询原证书
		certificateDB, err := db.GetCertificateForId(domain.CertificateId)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "查询证书记录异常：" + err.Error(),
			})
			return
		}
		certificatePrivate := models.CertificatePrivate{SavePath: certificateDB.SavePath}
		// 读取证书信息
		resource, err := certificatePrivate.LoadResource()
		if err != nil {
			c.JSON(400, gin.H{
				"message": "证书历史读取失败：" + err.Error(),
			})
			return
		}
		// 续期
		certificateData, err := certificate.RenewCertificate(provider, db.DomainToDomainInfo(domain), &resource.Resource)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "证书续期异常：" + err.Error(),
			})
			return
		}
		// 查询新证书信息并保存
		certificateNewDB, err := certificate.ParseCertificateAndSaveDb(certificateData)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "证书保存异常：" + err.Error(),
			})
			return
		}
		domain.CertificateId = certificateNewDB.Id
		err = db.UpdateDomain(domain)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "数据库更新异常：" + err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "ok",
			"data":    certificateData,
		})
	} else {
		certificateData, err := certificate.CreateCertificate(provider, db.DomainToDomainInfo(domain))
		if err != nil {
			c.JSON(400, gin.H{
				"message": err.Error(),
			})
			return
		}
		// 查询新证书信息并保存
		certificateNewDB, err := certificate.ParseCertificateAndSaveDb(certificateData)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "证书保存异常：" + err.Error(),
			})
			return
		}
		domain.CertificateId = certificateNewDB.Id
		err = db.UpdateDomain(domain)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "数据库更新异常：" + err.Error(),
			})
			return
		}
		c.JSON(200, gin.H{
			"message": "ok",
			"data":    certificateData,
		})
	}
}

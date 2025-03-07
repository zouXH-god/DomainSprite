package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/utils"
	"github.com/gin-gonic/gin"
	"strconv"
)

// CreateCertificateView 申请证书(基于数据库一键申请)
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

// GetCertificateListView 获取证书列表
func GetCertificateListView(c *gin.Context) {
	page := c.Query("page")
	pageSize := c.Query("pageSize")
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		pageInt = 0
	}
	pageSizeInt, err := strconv.Atoi(pageSize)
	if err != nil {
		pageSizeInt = 10
	}
	certificateList, err := db.GetCertificateList(pageInt, pageSizeInt)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "ok",
		"data":    certificateList,
	})
}

// GetCertificateViewWithDomainInfo 基于域名信息获取需要解析的内容
func GetCertificateViewWithDomainInfo(c *gin.Context) {
	domainName := c.Query("domainName")
	fullDomainName := "_acme-challenge" + "." + domainName
	rr := utils.HashString(domainName)
	value := rr + "." + models.AccountConfig.Certificate.ApplyDomainName
	c.JSON(200, gin.H{
		"message": "ok",
		"data": gin.H{
			"name":           "_acme-challenge",
			"type":           "cname",
			"domain":         domainName,
			"fullDomainName": fullDomainName,
			"value":          value,
		},
	})
}

// CreateCertificateViewWithDomainInfo 申请证书(基于域名信息)
func CreateCertificateViewWithDomainInfo(c *gin.Context) {
	domainName := c.PostForm("domainName")
	userDomain := "_acme-challenge" + "." + domainName
	selfDomian := utils.HashString(domainName) + "." + models.AccountConfig.Certificate.ApplyDomainName
	if !utils.IsCNAMEEqual(userDomain, selfDomian) {
		c.JSON(400, gin.H{
			"message": "域名未正确解析，请将域名就行cname解析到指定域名",
			"data": gin.H{
				"name":           "_acme-challenge",
				"type":           "cname",
				"domain":         domainName,
				"fullDomainName": userDomain,
				"value":          selfDomian,
			},
		})
		return
	}
	domainInfo := models.DomainInfo{}
	domainInfo.DomainName = domainName
	provider, err := getProviderForAccountName(models.AccountConfig.Certificate.ApplyAccount)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	certificateData, err := certificate.CreateCertificate(provider, domainInfo)
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
	c.JSON(200, gin.H{
		"message": "ok",
		"data":    certificateNewDB,
	})
}

// GetCertificateViewWithId 根据id查询证书消息
func GetCertificateViewWithId(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	certificateDB, err := db.GetCertificateForId(id)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "ok",
		"data":    certificateDB,
	})
}

// DownloadCertificateViewWithId 根据id下载证书
func DownloadCertificateViewWithId(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	downType := c.Query("type")
	certificateDB, err := db.GetCertificateForId(id)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
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
	}
	switch downType {
	case "cert":
		c.File(resource.CertificatePath)
	case "key":
		c.File(resource.PrivateKeyPath)
	}
	return
}

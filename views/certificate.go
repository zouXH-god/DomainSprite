package views

import (
	"DDNSServer/certificate"
	"DDNSServer/db"
	"DDNSServer/models"
	"DDNSServer/utils"
	"github.com/gin-gonic/gin"
	"log/slog"
	"strconv"
	"strings"
)

type CnameInfo struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Domain         string `json:"domain"`
	FullDomainName string `json:"fullDomainName"`
	Value          string `json:"value"`
}

func analyzeDomainForId(domainId string, domainList *[]models.Domains, domainInfoList *[]models.DomainInfo) error {
	domain, err := db.GetDomainForId(domainId)
	if err != nil {
		return err
	}
	*domainList = append(*domainList, domain)
	domainInfo := db.DomainToDomainInfo(domain)
	*domainInfoList = append(*domainInfoList, domainInfo)
	return nil
}

func GetCnameInfoForDomain(domainNameList []string) (cnameInfoList []CnameInfo) {
	name := "_acme-challenge"
	for _, domainName := range domainNameList {
		fullDomainName := name + "." + domainName
		rr := utils.HashString(domainName)
		value := rr + "." + models.AccountConfig.Certificate.ApplyDomainName
		cnameInfo := CnameInfo{
			Name:           name,
			Type:           "cname",
			Domain:         domainName,
			FullDomainName: fullDomainName,
			Value:          value,
		}
		cnameInfoList = append(cnameInfoList, cnameInfo)
	}
	return cnameInfoList
}

// CreateCertificateView 申请证书(基于数据库一键申请)
func CreateCertificateView(c *gin.Context) {
	domainId := c.PostForm("domainId")
	domainIdList := strings.Split(c.PostForm("domainIdList"), ",")
	renew := c.PostForm("renew") == "true"
	// 获取域名信息
	var domainInfoList []models.DomainInfo
	var domainList []models.Domains
	var err error
	if len(domainIdList) > 0 {
		for _, dId := range domainIdList {
			err = analyzeDomainForId(dId, &domainList, &domainInfoList)
			if err != nil {
				break
			}
		}
	} else if domainId != "" {
		err = analyzeDomainForId(domainId, &domainList, &domainInfoList)
	} else {
		c.JSON(400, gin.H{
			"message": "请选择域名",
		})
		return
	}
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
		certificateDB, err := db.GetCertificateForId(domainList[0].CertificateId)
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
		certificateData, err := certificate.RenewCertificate(provider, domainInfoList, &resource.Resource)
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
		for _, domain := range domainList {
			domain.CertificateId = certificateNewDB.Id
			err = db.UpdateDomain(domain)
			if err != nil {
				slog.Log(c, slog.LevelError, "更新域名信息失败", "domain", domain.DomainName)
			}
		}
		c.JSON(200, gin.H{
			"message": "ok",
			"data":    certificateData,
		})
	} else {
		certificateData, err := certificate.CreateCertificate(provider, domainInfoList)
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
		for _, domain := range domainList {
			domain.CertificateId = certificateNewDB.Id
			err = db.UpdateDomain(domain)
			if err != nil {
				slog.Log(c, slog.LevelError, "更新域名信息失败", "domain", domain.DomainName)
			}
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
	domainNameList := strings.Split(c.Query("domainNameList"), ",")
	cnameInfoList := GetCnameInfoForDomain(domainNameList)
	c.JSON(200, gin.H{
		"message": "ok",
		"data":    cnameInfoList,
	})
}

// CreateCertificateViewWithDomainInfo 申请证书(基于域名信息)
func CreateCertificateViewWithDomainInfo(c *gin.Context) {
	domainNameList := strings.Split(c.PostForm("domainNameList"), ",")
	cnameInfoList := GetCnameInfoForDomain(domainNameList)
	// 判断是否所有域名CNAME解析到指定域名
	var errorCnameInfo []CnameInfo
	for _, cnameInfo := range cnameInfoList {
		if !utils.IsCNAMEEqual(cnameInfo.FullDomainName, cnameInfo.Value) {
			errorCnameInfo = append(errorCnameInfo, cnameInfo)
		}
	}
	if len(errorCnameInfo) > 0 {
		c.JSON(400, gin.H{
			"message": "未将所有域名CNAME解析到指定域名",
			"data":    errorCnameInfo,
		})
		return
	}
	// 申请证书
	var domainInfoList []models.DomainInfo
	for _, domainName := range domainNameList {
		domainInfo := models.DomainInfo{}
		domainInfo.DomainName = domainName
		domainInfoList = append(domainInfoList, domainInfo)
	}
	provider, err := getProviderForAccountName(models.AccountConfig.Certificate.ApplyAccount)
	if err != nil {
		c.JSON(400, gin.H{
			"message": err.Error(),
		})
		return
	}
	certificateData, err := certificate.CreateCertificate(provider, domainInfoList)
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

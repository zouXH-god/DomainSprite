package db

import (
	"DDNSServer/models"
	"errors"
)

func mapDomainFields(domainInfo models.DomainInfo, domain models.Domains) (models.Domains, models.DomainInfo) {
	return domainInfo.Domains, models.DomainInfo{
		Domains: models.Domains{
			Id:         domain.Id,
			DomainName: domain.DomainName,
			DnsFrom:    domain.DnsFrom,
			GroupId:    domain.GroupId,
			GroupName:  domain.GroupName,
			Status:     domain.Status,
			Type:       domain.Type,
			CreateTime: domain.CreateTime,
			UpdateTime: domain.UpdateTime,
		},
	}
}

func DomainInfoToDomain(domainInfo models.DomainInfo) models.Domains {
	domain, _ := mapDomainFields(domainInfo, models.Domains{})
	return domain
}

func DomainToDomainInfo(domain models.Domains) models.DomainInfo {
	_, domainInfo := mapDomainFields(models.DomainInfo{}, domain)
	return domainInfo
}

// GetDomainForId 根据id获取域名信息
func GetDomainForId(id string) (models.Domains, error) {
	if id == "" {
		return models.Domains{}, errors.New("domainId is empty")
	}
	var domain models.Domains
	if err := DB.Model(&domain).Where("id = ?", id).First(&domain).Error; err != nil {
		return domain, err
	}
	return domain, nil
}

// IsDomainExist 根据域名判断是否存在
func IsDomainExist(domainName string) bool {
	var domain models.Domains
	if err := DB.Model(&domain).Where("domain_name = ?", domainName).First(&domain).Error; err == nil {
		return true
	}
	return false
}

// AddDomainInfo 添加域名信息,不存在则创建
func AddDomainInfo(domainInfo models.DomainInfo) error {
	domain := DomainInfoToDomain(domainInfo)
	// 判断是否已经存在,存在则更新,不存在则创建
	if err := DB.Model(&domain).Where("id = ?", domain.Id).First(&domain).Error; err == nil {
		if err := DB.Model(&domain).Updates(&domain).Error; err != nil {
			return err
		}
		return nil
	}
	if err := DB.Model(&domain).Create(&domain).Error; err != nil {
		return err
	}
	return nil
}

// UpdateDomain 更新域名信息
func UpdateDomain(domain models.Domains) error {
	if err := DB.Model(&domain).Updates(&domain).Error; err != nil {
		return err
	}
	return nil
}

// GetCertificateList 获取证书列表
func GetCertificateList(page, pageSize int) ([]models.Certificate, error) {
	var certificates []models.Certificate
	if err := DB.Model(&certificates).Limit(pageSize).Offset((page - 1) * pageSize).Find(&certificates).Error; err != nil {
		return certificates, err
	}
	return certificates, nil
}

// GetCertificateForId 根据Id获取证书
func GetCertificateForId(id int) (models.Certificate, error) {
	if id == 0 {
		return models.Certificate{}, errors.New("certificateId is empty")
	}
	var certificate models.Certificate
	if err := DB.Model(&certificate).Where("id = ?", id).First(&certificate).Error; err != nil {
		return certificate, err
	}
	return certificate, nil
}

// AddCertificateInfo 添加证书信息,不存在则创建
func AddCertificateInfo(certificateInfo *models.Certificate) error {
	// 判断是否已经存在,存在则更新,不存在则创建
	if err := DB.Model(&certificateInfo).Where("id = ?", certificateInfo.Id).First(&certificateInfo).Error; err == nil {
		if err := DB.Model(&certificateInfo).Updates(&certificateInfo).Error; err != nil {
			return err
		}
		return nil
	}
	if err := DB.Model(&certificateInfo).Create(&certificateInfo).Error; err != nil {
	}
	return nil
}

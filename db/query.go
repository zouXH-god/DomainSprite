package db

import (
	"DDNSServer/models"
	"errors"
	"gorm.io/gorm"
)

func mapDomainFields(domainInfo models.DomainInfo, domain models.Domains) (models.Domains, models.DomainInfo) {
	return domainInfo.Domains, models.DomainInfo{
		Domains: models.Domains{
			DBID:          domain.DBID,
			Id:            domain.Id,
			DomainName:    domain.DomainName,
			DnsFrom:       domain.DnsFrom,
			GroupId:       domain.GroupId,
			GroupName:     domain.GroupName,
			Status:        domain.Status,
			Type:          domain.Type,
			AccountName:   domain.AccountName,
			CertificateId: domain.CertificateId,
			CreateTime:    domain.CreateTime,
			UpdateTime:    domain.UpdateTime,
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
func GetDomainForId(accountName, id string) (models.Domains, error) {
	if id == "" {
		return models.Domains{}, errors.New("domainId is empty")
	}
	var domain models.Domains
	if err := DB.Model(&domain).Where("id = ? AND account_name = ?", id, accountName).First(&domain).Error; err != nil {
		return domain, err
	}
	return domain, nil
}

// IsDomainExist 根据域名判断是否存在
func IsDomainExist(accountName, dnsFrom, domainName string) (bool, error) {
	var domain models.Domains
	err := DB.Model(&domain).Where("domain_name = ? AND account_name = ? AND dns_from = ?", domainName, accountName, dnsFrom).First(&domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// AddDomainInfo 添加域名信息,不存在则创建
func AddDomainInfo(domainInfo models.DomainInfo) error {
	domain := DomainInfoToDomain(domainInfo)
	var existing models.Domains
	err := DB.Where("id = ? AND account_name = ? AND dns_from = ?", domain.Id, domain.AccountName, domain.DnsFrom).First(&existing).Error
	if err == nil {
		domain.DBID = existing.DBID
		return DB.Model(&existing).Select("*").Updates(&domain).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return DB.Create(&domain).Error
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
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
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
	if certificateInfo == nil {
		return errors.New("certificateInfo is nil")
	}
	return DB.Save(certificateInfo).Error
}

// GetTaskInfoList 获取任务日志列表
func GetTaskInfoList(taskId string) ([]models.CertificateTask, error) {
	if taskId == "" {
		return []models.CertificateTask{}, errors.New("taskId is empty")
	}
	var taskList []models.CertificateTask
	if err := DB.Model(&models.CertificateTask{}).Where("task_id = ?", taskId).Order("id desc").Find(&taskList).Error; err != nil {
		return taskList, err
	}
	return taskList, nil
}

// GetTaskInfoById 获取任务具体信息
func GetTaskInfoById(id int) (models.CertificateTask, error) {
	var task models.CertificateTask
	if err := DB.Model(&models.CertificateTask{}).Where("id = ?", id).First(&task).Error; err != nil {
		return task, err
	}
	return task, nil
}

func GetTaskInfoByTaskID(taskID string) (models.CertificateTask, error) {
	if taskID == "" {
		return models.CertificateTask{}, errors.New("taskId is empty")
	}
	var task models.CertificateTask
	err := DB.Where("task_id = ?", taskID).First(&task).Error
	return task, err
}

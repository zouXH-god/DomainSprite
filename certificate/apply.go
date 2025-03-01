package certificate

import (
	"DDNSServer/models"
	"fmt"
	"github.com/go-acme/lego/v4/certificate"
)

// CreateCertificate 申请证书
func CreateCertificate(recordProvider models.RecordProvider, domain models.DomainInfo) (*certificate.Resource, error) {
	// 初始化 ClientManager
	manager := models.ClientManager{}

	// 获取客户端
	client, err := manager.GetClient()
	if err != nil {
		return &certificate.Resource{}, err
	}

	// 配置 DNS-01 挑战
	provider := models.NewProvider(recordProvider, domain)
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		return &certificate.Resource{}, fmt.Errorf("设置 DNS-01 挑战失败: %v", err)
	}

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: []string{"*." + domain.DomainName, domain.DomainName}, // 通配符和主域名
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return &certificate.Resource{}, fmt.Errorf("申请证书失败: %v", err)
	}

	// 增加请求计数
	manager.RequestCount++

	// 保存新证书
	err = provider.SaveCertificate(certificates)
	if err != nil {
		return certificates, err
	}

	fmt.Println("证书申请成功！")
	return certificates, nil
}

// RenewCertificate 续期证书
func RenewCertificate(recordProvider models.RecordProvider, domain models.DomainInfo, existingCert *certificate.Resource) (*certificate.Resource, error) {
	manager := models.ClientManager{}
	client, err := manager.GetClient()
	if err != nil {
		return nil, err
	}

	provider := models.NewProvider(recordProvider, domain)
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		return nil, fmt.Errorf("设置 DNS-01 挑战失败: %v", err)
	}

	// 续期证书
	renewedCert, err := client.Certificate.RenewWithOptions(*existingCert, &certificate.RenewOptions{})
	if err != nil {
		return nil, fmt.Errorf("续期证书失败: %v", err)
	}

	manager.RequestCount++

	// 保存新证书
	err = provider.SaveCertificate(renewedCert)
	if err != nil {
		return renewedCert, err
	}

	fmt.Println("证书续期成功！")
	return renewedCert, nil
}

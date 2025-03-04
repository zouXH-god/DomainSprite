package certificate

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/go-acme/lego/v4/certificate"
	"strings"
)

// CreateCertificate 全自动申请证书
func CreateCertificate(recordProvider models.RecordProvider, domain models.DomainInfo) (*models.Resource, error) {
	// 初始化 ClientManager
	manager := models.ClientManager{}

	// 获取客户端
	client, err := manager.GetClient()
	if err != nil {
		return &models.Resource{}, err
	}

	// 配置 DNS-01 挑战
	provider := models.NewProvider(recordProvider, domain)
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		return &models.Resource{}, fmt.Errorf("设置 DNS-01 挑战失败: %v", err)
	}

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: []string{"*." + domain.DomainName, domain.DomainName}, // 通配符和主域名
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return &models.Resource{}, fmt.Errorf("申请证书失败: %v", err)
	}

	// 增加请求计数
	manager.RequestCount++

	// 保存新证书
	resource, err := provider.SaveCertificate(certificates)
	if err != nil {
		return resource, err
	}

	fmt.Println("证书申请成功！")
	return resource, nil
}

// RenewCertificate 全自动续期证书
func RenewCertificate(recordProvider models.RecordProvider, domain models.DomainInfo, existingCert *certificate.Resource) (*models.Resource, error) {
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
	resource, err := provider.SaveCertificate(renewedCert)
	if err != nil {
		return resource, err
	}

	fmt.Println("证书续期成功！")
	return resource, nil
}

// ParseCertificate 函数解析PEM格式的证书并返回CertificateInfo结构体
func ParseCertificate(resource *models.Resource) (*models.Certificate, error) {
	var certDER []byte
	// 解码PEM数据，提取第一个证书
	for {
		block, _ := pem.Decode(resource.Certificate)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certDER = block.Bytes
			break
		}
	}
	// 如果没有找到证书，返回错误
	if certDER == nil {
		return nil, errors.New("no CERTIFICATE found in PEM data")
	}
	// 解析DER格式的证书
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}
	// 填充 CertificateInfo 结构体
	info := &models.Certificate{
		SavePath:   resource.SavePath,
		Issuer:     cert.Issuer.String(),
		Subject:    cert.Subject.String(),
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		DNSNames:   strings.Join(cert.DNSNames, ","),
		CommonName: cert.Subject.CommonName,
	}
	return info, nil
}

// ParseCertificateAndSaveDb 函数解析PEM格式的证书并保存到数据库中
func ParseCertificateAndSaveDb(resource *models.Resource) (*models.Certificate, error) {
	info, err := ParseCertificate(resource)
	if err != nil {
		return info, err
	}
	err = db.AddCertificateInfo(info)
	if err != nil {
		return info, err
	}
	return info, nil
}

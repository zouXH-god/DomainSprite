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

// ParseCertificate 函数解析PEM格式的证书并返回CertificateInfo结构体
func ParseCertificate(pemData []byte) (*models.Certificate, error) {
	var certDER []byte
	// 解码PEM数据，提取第一个证书
	for {
		block, _ := pem.Decode(pemData)
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
		Issuer:     cert.Issuer.String(),
		Subject:    cert.Subject.String(),
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		DNSNames:   strings.Join(cert.DNSNames, ","),
		CommonName: cert.Subject.CommonName,
	}
	return info, nil
}

func ParseCertificateAndSaveDb(pemData []byte) error {
	info, err := ParseCertificate(pemData)
	if err != nil {
		return err
	}
	err = db.AddCertificateInfo(info)
	if err != nil {
		return err
	}
	return nil
}

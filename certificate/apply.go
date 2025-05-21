package certificate

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/go-acme/lego/v4/certificate"
	"log/slog"
	"strings"
)

func getLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value("logger").(*slog.Logger)
	if !ok {
		logger = slog.Default()
	}
	return logger
}

// CreateCertificate 全自动申请证书
func CreateCertificate(ctx context.Context, recordProvider models.RecordProvider, domains []models.DomainInfo) (*models.Resource, error) {
	logger := getLogger(ctx)
	// 初始化 ClientManager
	manager := models.ClientManager{}

	// 获取客户端
	client, err := manager.GetClient()
	if err != nil {
		logger.Error("创建 ACME 客户端失败", "err", err)
		return &models.Resource{}, err
	}

	// 配置 DNS-01 挑战
	provider := models.NewProvider(recordProvider, domains[0])
	provider.SelfDomain = db.IsDomainExist(domains[0].DomainName)
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		logger.Error("设置 DNS-01 挑战失败", "err", err)
		return &models.Resource{}, fmt.Errorf("设置 DNS-01 挑战失败: %v", err)
	}

	// 编辑域名信息
	var domainList []string
	for _, domain := range domains {
		logger.Debug("编辑域名信息", "domain", domain.DomainName)
		domainList = append(domainList, "*."+domain.DomainName)
		domainList = append(domainList, domain.DomainName)
	}

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: domainList, // 通配符和主域名
		Bundle:  true,
	}
	logger.Debug("申请证书", "domains", domainList)
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		logger.Error("申请证书失败", "err", err)
		return &models.Resource{}, fmt.Errorf("申请证书失败: %v", err)
	}

	// 增加请求计数
	manager.RequestCount++

	// 保存新证书
	logger.Debug("保存新证书", "certificate", certificates.Certificate)
	resource, err := provider.SaveCertificate(certificates)
	if err != nil {
		logger.Error("保存新证书失败", "err", err)
		return resource, err
	}

	logger.Info("证书申请成功！")
	return resource, nil
}

// RenewCertificate 全自动续期证书
func RenewCertificate(ctx context.Context, recordProvider models.RecordProvider, domain []models.DomainInfo, existingCert *certificate.Resource) (*models.Resource, error) {
	logger := getLogger(ctx)
	manager := models.ClientManager{}
	client, err := manager.GetClient()
	if err != nil {
		return nil, err
	}

	// 配置 DNS-01 挑战
	provider := models.NewProvider(recordProvider, domain[0])
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		logger.Error("设置 DNS-01 挑战失败", "err", err)
		return nil, fmt.Errorf("设置 DNS-01 挑战失败: %v", err)
	}

	// 续期证书
	logger.Debug("续期证书", "certificate", existingCert)
	renewedCert, err := client.Certificate.RenewWithOptions(*existingCert, &certificate.RenewOptions{})
	if err != nil {
		logger.Error("续期证书失败", "err", err)
		return nil, fmt.Errorf("续期证书失败: %v", err)
	}

	manager.RequestCount++

	// 保存新证书
	logger.Debug("保存新证书", "certificate", renewedCert)
	resource, err := provider.SaveCertificate(renewedCert)
	if err != nil {
		logger.Error("保存新证书失败", "err", err)
		return resource, err
	}

	logger.Info("证书续期成功！")
	return resource, nil
}

// ParseCertificate 函数解析PEM格式的证书并返回CertificateInfo结构体
func ParseCertificate(certificateInfo *models.Certificate, resource *models.Resource) (*models.Certificate, error) {
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

	// 构建 DomainList，包含 CommonName 和 DNSNames，去重
	domainMap := make(map[string]struct{}) // 使用 map 去重
	if cert.Subject.CommonName != "" {
		domainMap[cert.Subject.CommonName] = struct{}{}
	}
	for _, dnsName := range cert.DNSNames {
		domainMap[dnsName] = struct{}{}
	}
	// 将 map 转换为切片
	domainList := make([]string, 0, len(domainMap))
	for domain := range domainMap {
		domainList = append(domainList, domain)
	}

	// 填充 CertificateInfo 结构体
	certificateInfo.SavePath = resource.SavePath
	certificateInfo.Issuer = cert.Issuer.String()
	certificateInfo.Subject = cert.Subject.String()
	certificateInfo.NotBefore = cert.NotBefore
	certificateInfo.NotAfter = cert.NotAfter
	certificateInfo.DNSNames = strings.Join(cert.DNSNames, ",")
	certificateInfo.CommonName = cert.Subject.CommonName
	certificateInfo.DomainList = strings.Join(domainList, ",")
	return certificateInfo, nil
}

// ParseCertificateAndSaveDb 函数解析PEM格式的证书并保存到数据库中
func ParseCertificateAndSaveDb(ctx context.Context, resource *models.Resource, certificateInfo *models.Certificate) (*models.Certificate, error) {
	logger := getLogger(ctx)
	logger.Debug("解析证书", "resource", resource)
	info, err := ParseCertificate(certificateInfo, resource)
	if err != nil {
		logger.Error("解析证书失败", "err", err)
		return info, err
	}
	logger.Debug("保存证书信息", "info", info)
	err = db.AddCertificateInfo(info)
	if err != nil {
		logger.Error("保存证书信息失败", "err", err)
		return info, err
	}
	logger.Info("证书保存成功", "info", info)
	return info, nil
}

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
	"github.com/go-acme/lego/v4/challenge/dns01"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var acmeMu sync.Mutex
var acmeManager models.ClientManager

func getLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	if !ok {
		logger = slog.Default()
	}
	return logger
}

// CreateCertificate 全自动申请证书
func CreateCertificate(ctx context.Context, recordProvider models.RecordProvider, domains []models.DomainInfo) (*models.Resource, error) {
	logger := getLogger(ctx)
	if len(domains) == 0 {
		return nil, errors.New("域名列表不能为空")
	}
	acmeMu.Lock()
	defer acmeMu.Unlock()
	manager := &acmeManager

	// 获取客户端
	client, err := manager.GetClient()
	if err != nil {
		logger.Error("创建 ACME 客户端失败", "err", err)
		return &models.Resource{}, err
	}

	// 配置 DNS-01 挑战
	provider := models.NewProvider(recordProvider, domains[0])
	provider.SelfDomain, err = db.IsDomainExist(domains[0].AccountName, domains[0].DnsFrom, domains[0].DomainName)
	if err != nil {
		return nil, fmt.Errorf("查询域名归属: %w", err)
	}
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
	logger.Debug("certificate.persist_started")
	resource, err := provider.SaveCertificate(certificates)
	if err != nil {
		logger.Error("保存新证书失败", "err", err)
		return resource, err
	}

	logger.Info("证书申请成功！")
	return resource, nil
}

// IssueCertificate signs SANs but deliberately does not persist them. The worker
// records the issued artifact in staging before advancing the task state.
func IssueCertificate(ctx context.Context, provider *models.CertificatePrivate, sans []string) (*certificate.Resource, error) {
	if provider == nil || len(sans) == 0 {
		return nil, errors.New("证书域名不能为空")
	}
	acmeMu.Lock()
	defer acmeMu.Unlock()
	logger := getLogger(ctx)
	logger.Info("acme.obtain_started", "san_count", len(sans))
	client, err := acmeManager.GetClient()
	if err != nil {
		logger.Error("acme.client_failed", "error", err)
		return nil, err
	}
	if err := client.Challenge.SetDNS01Provider(provider, dns01.PropagationWait(30*time.Second, true)); err != nil {
		logger.Error("acme.provider_failed", "error", err)
		return nil, fmt.Errorf("设置 DNS-01 挑战: %w", err)
	}
	defer provider.CleanupPending()
	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{Domains: sans, Bundle: true})
	if err != nil {
		logger.Error("acme.obtain_failed", "error", err)
		return nil, fmt.Errorf("申请证书: %w", err)
	}
	acmeManager.RequestCount++
	logger.Info("acme.issued", "san_count", len(sans))
	return resource, nil
}

func IssueRenewedCertificate(ctx context.Context, provider *models.CertificatePrivate, existing *certificate.Resource, sans []string) (*certificate.Resource, error) {
	if provider == nil || existing == nil {
		return nil, errors.New("续期资源不能为空")
	}
	acmeMu.Lock()
	defer acmeMu.Unlock()
	logger := getLogger(ctx)
	logger.Info("acme.renew_started", "san_count", len(sans))
	client, err := acmeManager.GetClient()
	if err != nil {
		return nil, err
	}
	if err = client.Challenge.SetDNS01Provider(provider, dns01.PropagationWait(30*time.Second, true)); err != nil {
		return nil, fmt.Errorf("设置 DNS-01 挑战: %w", err)
	}
	defer provider.CleanupPending()
	resource, renewErr := client.Certificate.RenewWithOptions(*existing, &certificate.RenewOptions{})
	if renewErr != nil {
		logger.Warn("acme.renew_fallback", "reason", renewErr)
		resource, err = client.Certificate.Obtain(certificate.ObtainRequest{Domains: sans, Bundle: true})
		if err != nil {
			logger.Error("acme.renew_fallback_failed", "error", err)
			return nil, fmt.Errorf("续期失败 (%v)，重新签发也失败: %w", renewErr, err)
		}
	}
	acmeManager.RequestCount++
	logger.Info("acme.issued", "kind", "renew", "san_count", len(sans))
	return resource, nil
}

// RenewCertificate 全自动续期证书
func RenewCertificate(ctx context.Context, recordProvider models.RecordProvider, domain []models.DomainInfo, existingCert *certificate.Resource) (*models.Resource, error) {
	logger := getLogger(ctx)
	if len(domain) == 0 {
		return nil, errors.New("域名列表不能为空")
	}
	acmeMu.Lock()
	defer acmeMu.Unlock()
	manager := &acmeManager
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
	logger.Debug("certificate.renew_started")
	renewedCert, err := client.Certificate.RenewWithOptions(*existingCert, &certificate.RenewOptions{})
	if err != nil {
		logger.Error("续期证书失败", "err", err)
		return nil, fmt.Errorf("续期证书失败: %v", err)
	}

	manager.RequestCount++

	// 保存新证书
	logger.Debug("certificate.persist_started")
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
	logger.Debug("certificate.parse_started")
	info, err := ParseCertificate(certificateInfo, resource)
	if err != nil {
		logger.Error("解析证书失败", "err", err)
		return info, err
	}
	logger.Debug("certificate.database_save_started", "certificate_id", info.Id)
	err = db.AddCertificateInfo(info)
	if err != nil {
		logger.Error("保存证书信息失败", "err", err)
		return info, err
	}
	logger.Info("证书保存成功", "info", info)
	return info, nil
}

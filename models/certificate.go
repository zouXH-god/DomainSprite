package models

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var EmailIndex = 0
var LoadACMEIdentity func(email string) (keyPEM, registrationJSON []byte, err error)
var SaveACMEIdentity func(email string, keyPEM, registrationJSON []byte) error

type Resource struct {
	certificate.Resource
	PrivateKeyPath  string `json:"privateKeyPath"`  // 私钥文件路径
	CertificatePath string `json:"certificatePath"` // 证书文件路径
	IssuerCertPath  string `json:"issuerCertPath"`  // 发行者证书路径
	CSRPath         string `json:"csrPath"`         // CSR 文件路径
	SavePath        string `json:"savePath"`
}

type ClientManager struct {
	Client       *lego.Client
	Resource     *registration.Resource
	RequestCount int
	MaxRequests  int
	Email        string
	privateKey   crypto.PrivateKey // 新增字段存储私钥
}

func (m *ClientManager) GetEmail() string {
	return m.Email
}
func (m *ClientManager) GetRegistration() *registration.Resource {
	return m.Resource
}
func (m *ClientManager) GetPrivateKey() crypto.PrivateKey {
	if m.privateKey == nil {
		// 如果私钥不存在，生成一个新的 RSA 私钥
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			fmt.Println("生成私钥失败:", err)
		}
		m.privateKey = key
	}
	return m.privateKey
}

func (m *ClientManager) GetClient() (*lego.Client, error) {
	if len(AccountConfig.Certificate.EmailList) == 0 {
		return nil, fmt.Errorf("证书邮箱列表为空")
	}
	if m.Client == nil || m.RequestCount >= m.MaxRequests {
		// 获取一个邮箱
		m.Email = AccountConfig.Certificate.EmailList[EmailIndex]
		m.MaxRequests = AccountConfig.Certificate.MaxRequest
		if err := m.loadOrCreateAccountKey(); err != nil {
			return nil, err
		}
		// 创建新客户端
		config := lego.NewConfig(m)
		config.CADirURL = AccountConfig.Certificate.CADirURL
		if config.CADirURL == "" {
			config.CADirURL = lego.LEDirectoryProduction
		}
		config.Certificate.KeyType = certcrypto.RSA2048 // 设置默认密钥类型
		client, err := lego.NewClient(config)
		if err != nil {
			return nil, fmt.Errorf("创建 ACME 客户端失败: %v", err)
		}
		reg, err := client.Registration.ResolveAccountByKey()
		if err != nil {
			reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		}
		if err != nil {
			return nil, fmt.Errorf("注册用户失败: %w", err)
		}
		m.Resource = reg
		_ = m.persistRegistration(reg)
		// 邮箱迭代
		EmailIndex++
		if EmailIndex >= len(AccountConfig.Certificate.EmailList) {
			EmailIndex = 0
		}
		m.Client = client
		m.RequestCount = 0 // 重置计数
	}
	return m.Client, nil
}

func (m *ClientManager) accountDir() string {
	h := sha256.Sum256([]byte(strings.ToLower(m.Email)))
	return filepath.Join(AccountConfig.Certificate.SavePath, "acme", hex.EncodeToString(h[:16]))
}
func (m *ClientManager) loadOrCreateAccountKey() error {
	if LoadACMEIdentity != nil {
		keyPEM, _, err := LoadACMEIdentity(m.Email)
		if err == nil && len(keyPEM) > 0 {
			block, _ := pem.Decode(keyPEM)
			if block == nil {
				return errors.New("ACME 账户密钥格式错误")
			}
			key, e := x509.ParsePKCS1PrivateKey(block.Bytes)
			if e != nil {
				return e
			}
			m.privateKey = key
			return nil
		}
	}
	dir := m.accountDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "account.key")
	b, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return errors.New("ACME 账户密钥格式错误")
		}
		key, e := x509.ParsePKCS1PrivateKey(block.Bytes)
		if e != nil {
			return e
		}
		m.privateKey = key
		if SaveACMEIdentity != nil {
			if e := SaveACMEIdentity(m.Email, b, nil); e != nil {
				return e
			}
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return e
	}
	m.privateKey = key
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if SaveACMEIdentity != nil {
		return SaveACMEIdentity(m.Email, keyPEM, nil)
	}
	return atomicWriteFile(path, keyPEM, 0600)
}
func (m *ClientManager) persistRegistration(reg *registration.Resource) error {
	b, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	if SaveACMEIdentity != nil {
		var keyPEM []byte
		if key, ok := m.privateKey.(*rsa.PrivateKey); ok {
			keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		}
		return SaveACMEIdentity(m.Email, keyPEM, b)
	}
	return atomicWriteFile(filepath.Join(m.accountDir(), "registration.json"), b, 0600)
}

type CertificatePrivate struct {
	provider         RecordProvider
	domain           DomainInfo
	targets          map[string]ChallengeTarget
	SavePath         string
	SelfDomain       bool
	mu               sync.Mutex
	challengeRecords map[string]createdChallenge
	CleanupFailure   func(ChallengeTarget, RecordInfo, error)
	Logger           *slog.Logger
}

type createdChallenge struct {
	target ChallengeTarget
	record RecordInfo
}

type ChallengeTarget struct {
	Provider RecordProvider
	Domain   DomainInfo
	Mode     string
}

func (p *CertificatePrivate) targetFor(domain string) ChallengeTarget {
	base := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(domain), "."), "*.")
	if t, ok := p.targets[base]; ok {
		return t
	}
	return ChallengeTarget{Provider: p.provider, Domain: p.domain, Mode: map[bool]string{true: "direct", false: "delegated"}[p.SelfDomain]}
}

// Present 添加 TXT 记录以完成 DNS-01 挑战
func (p *CertificatePrivate) Present(domain, token, keyAuth string) error {
	// 解析挑战信息
	fqdn := dns01.GetChallengeInfo(domain, keyAuth)
	rr := strings.TrimSuffix(fqdn.FQDN, "."+domain+".") // _acme-challenge
	target := p.targetFor(domain)
	provider := target.Provider
	targetDomain := target.Domain
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("challenge.present_started", "domain", targetDomain.DomainName, "account", provider.GetAccountInfo().Name, "provider", provider.GetAccountInfo().Type, "zone_id", targetDomain.Id, "mode", target.Mode)
	// 构造 TXT 记录
	record := RecordInfo{}

	// 判断该域名是否存在数据库中，不存在按照第三方申请，获取其hash进行解析
	if target.Mode == "delegated" {
		record = RecordInfo{
			DomainId:      AccountConfig.Certificate.ApplyDomainId,
			DomainName:    AccountConfig.Certificate.ApplyDomainName,
			RecordName:    delegatedChallengeRecordName(domain),
			RecordType:    "TXT",
			RecordContent: fqdn.Value,
			Ttl:           60,
		}
	} else {
		record = RecordInfo{
			DomainId:      targetDomain.Id,
			DomainName:    domain,
			RecordName:    rr,
			RecordType:    "TXT",
			RecordContent: fqdn.Value,
			Ttl:           60,
		}
	}

	// 添加记录
	created, err := provider.AddRecord(record)
	if err != nil {
		logger.Error("challenge.present_failed", "domain", targetDomain.DomainName, "account", provider.GetAccountInfo().Name, "provider", provider.GetAccountInfo().Type, "error", err)
		return fmt.Errorf("添加 TXT 记录失败: %v", err)
	}
	p.mu.Lock()
	p.challengeRecords[fqdn.Value] = createdChallenge{target: target, record: created}
	p.mu.Unlock()
	logger.Info("challenge.present_succeeded", "domain", targetDomain.DomainName, "account", provider.GetAccountInfo().Name, "provider", provider.GetAccountInfo().Type, "record_id_available", created.Id != "")
	return nil
}

// CleanUp 删除 TXT 记录
func (p *CertificatePrivate) CleanUp(domain, token, keyAuth string) error {
	// 解析挑战信息
	fqdn := dns01.GetChallengeInfo(domain, keyAuth)
	rr := strings.TrimSuffix(fqdn.FQDN, "."+domain+".") // _acme-challenge
	p.mu.Lock()
	createdEntry, ok := p.challengeRecords[fqdn.Value]
	if ok {
		delete(p.challengeRecords, fqdn.Value)
	}
	p.mu.Unlock()
	created := createdEntry.record
	if ok && created.Id != "" {
		deleteDomain := created.DomainName
		if deleteDomain == "" {
			deleteDomain = domain
		}
		target := p.targetFor(domain)
		logger := p.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Info("challenge.cleanup_started", "domain", target.Domain.DomainName, "account", target.Provider.GetAccountInfo().Name, "provider", target.Provider.GetAccountInfo().Type, "by_record_id", true)
		err := deleteChallengeRecord(target.Provider, deleteDomain, created.Id)
		if err != nil {
			if p.CleanupFailure != nil {
				p.CleanupFailure(target, created, err)
			}
			logger.Warn("challenge.cleanup_deferred", "domain", target.Domain.DomainName, "account", target.Provider.GetAccountInfo().Name, "provider", target.Provider.GetAccountInfo().Type, "error", err)
			return fmt.Errorf("删除 TXT 记录失败: %w", err)
		}
		logger.Info("challenge.cleanup_succeeded", "domain", target.Domain.DomainName, "account", target.Provider.GetAccountInfo().Name, "provider", target.Provider.GetAccountInfo().Type)
		return nil
	}
	// 兼容未返回记录 ID 的厂商：精确匹配记录内容后清理
	search := DNSSearch{}

	// 判断该域名是否存在数据库中，不存在按照第三方申请，获取其hash进行解析
	target := p.targetFor(domain)
	provider := target.Provider
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("challenge.cleanup_fallback", "domain", target.Domain.DomainName, "account", provider.GetAccountInfo().Name, "provider", provider.GetAccountInfo().Type)
	if target.Mode == "delegated" {
		search = DNSSearch{
			DomainId:    AccountConfig.Certificate.ApplyDomainId,
			DomainName:  AccountConfig.Certificate.ApplyDomainName,
			RRKeyWord:   delegatedChallengeRecordName(domain),
			TypeKeyWord: "TXT",
		}
	} else {
		search = DNSSearch{
			DomainId:    target.Domain.Id,
			DomainName:  domain,
			RRKeyWord:   rr,
			TypeKeyWord: "TXT",
		}
	}

	records, err := provider.GetRecordList(search)
	if err != nil {
		logger.Error("challenge.cleanup_lookup_failed", "domain", target.Domain.DomainName, "error", err)
		return fmt.Errorf("获取记录列表失败: %v", err)
	}

	// 删除匹配的记录
	for _, record := range records.Records {
		if fqdn.Value == record.RecordContent {
			_, err := provider.DeleteRecord(search.DomainName, record.Id)
			if err != nil {
				if p.CleanupFailure != nil {
					p.CleanupFailure(target, record, err)
				}
				logger.Error("challenge.cleanup_failed", "domain", target.Domain.DomainName, "error", err)
				return fmt.Errorf("删除 TXT 记录失败: %v", err)
			}
			logger.Info("challenge.cleanup_succeeded", "domain", target.Domain.DomainName, "account", provider.GetAccountInfo().Name, "provider", provider.GetAccountInfo().Type)
		}
	}
	return nil
}

// CleanupPending removes records left behind when lego aborts before invoking
// its normal challenge cleanup (for example when propagation checking fails).
// Records are deleted only by the exact provider-returned RecordId.
func (p *CertificatePrivate) CleanupPending() {
	p.mu.Lock()
	pending := make([]createdChallenge, 0, len(p.challengeRecords))
	for key, item := range p.challengeRecords {
		pending = append(pending, item)
		delete(p.challengeRecords, key)
	}
	p.mu.Unlock()
	for _, item := range pending {
		if item.record.Id == "" {
			continue
		}
		domain := item.record.DomainName
		if domain == "" {
			domain = item.target.Domain.DomainName
		}
		if err := deleteChallengeRecord(item.target.Provider, domain, item.record.Id); err != nil {
			if p.CleanupFailure != nil {
				p.CleanupFailure(item.target, item.record, err)
			}
			logger := p.Logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("challenge.cleanup_deferred", "domain", domain, "record_id", item.record.Id, "error", err)
		}
	}
}

func deleteChallengeRecord(provider RecordProvider, domain, recordID string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * 500 * time.Millisecond)
		}
		if _, err := provider.DeleteRecord(domain, recordID); err == nil || recordAlreadyGone(err) {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func recordAlreadyGone(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "recordid.notfound") ||
		strings.Contains(message, "invalidrecordid.notfound") ||
		strings.Contains(message, "record not found") ||
		strings.Contains(message, "记录不存在")
}

func (p *CertificatePrivate) GetResourcePath() string {
	return filepath.Join(p.SavePath, "resource.json")
}

// SaveCertificate 保存证书
func (p *CertificatePrivate) SaveCertificate(certificates *certificate.Resource) (*Resource, error) {
	resource := Resource{}
	resource.Resource = *certificates
	resource.SavePath = p.SavePath
	resource.CertificatePath = filepath.Join(p.SavePath, "certificate.crt")
	resource.PrivateKeyPath = filepath.Join(p.SavePath, "private.key")
	resource.IssuerCertPath = filepath.Join(p.SavePath, "issuer.crt")
	resource.CSRPath = filepath.Join(p.SavePath, "csr.csr")
	// 创建保存路径
	err := os.MkdirAll(p.SavePath, 0755)
	if err != nil {
		return &resource, fmt.Errorf("创建保存路径失败: %v", err)
	}
	// 保存证书文件
	err = atomicWriteFile(resource.CertificatePath, certificates.Certificate, 0644)
	if err != nil {
		return &resource, fmt.Errorf("保存证书失败: %v", err)
	}
	err = atomicWriteFile(resource.PrivateKeyPath, certificates.PrivateKey, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	err = atomicWriteFile(resource.IssuerCertPath, certificates.IssuerCertificate, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	err = atomicWriteFile(resource.CSRPath, certificates.CSR, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	// 序列化 JSON（包含路径字段）
	jsonData, err := json.MarshalIndent(resource, "", "    ")
	if err != nil {
		return &resource, err
	}
	// 保存 JSON 文件
	if err = atomicWriteFile(p.GetResourcePath(), jsonData, 0600); err != nil {
		return &resource, err
	}
	return &resource, nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".cert-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (p *CertificatePrivate) LoadResource() (*Resource, error) {
	var resource Resource

	// 读取 JSON 文件
	jsonData, err := os.ReadFile(p.GetResourcePath())
	if err != nil {
		return &resource, err
	}

	// 反序列化 JSON 到结构体
	if err := json.Unmarshal(jsonData, &resource); err != nil {
		return &resource, err
	}

	// 根据路径读取 PrivateKey
	if resource.PrivateKeyPath != "" {
		if privateKeyData, err := os.ReadFile(resource.PrivateKeyPath); err == nil {
			resource.PrivateKey = privateKeyData
		} else if !os.IsNotExist(err) {
			return &resource, err
		}
	}

	// 根据路径读取 Certificate
	if resource.CertificatePath != "" {
		if certData, err := os.ReadFile(resource.CertificatePath); err == nil {
			resource.Certificate = certData
		} else if !os.IsNotExist(err) {
			return &resource, err
		}
	}

	// 根据路径读取 IssuerCertificate
	if resource.IssuerCertPath != "" {
		if issuerCertData, err := os.ReadFile(resource.IssuerCertPath); err == nil {
			resource.IssuerCertificate = issuerCertData
		} else if !os.IsNotExist(err) {
			return &resource, err
		}
	}

	// 根据路径读取 CSR
	if resource.CSRPath != "" {
		if csrData, err := os.ReadFile(resource.CSRPath); err == nil {
			resource.CSR = csrData
		} else if !os.IsNotExist(err) {
			return &resource, err
		}
	}

	return &resource, nil
}

func NewProvider(recordProvider RecordProvider, domain DomainInfo) *CertificatePrivate {
	nowTime := time.Now()
	SavePath := filepath.Join(
		AccountConfig.Certificate.SavePath,
		nowTime.Format("2006"),
		nowTime.Format("01-02"),
		domain.DomainName,
	)
	return &CertificatePrivate{
		provider:         recordProvider,
		domain:           domain,
		SavePath:         SavePath,
		challengeRecords: make(map[string]createdChallenge),
	}
}

func NewMultiProvider(targets map[string]ChallengeTarget, savePath string) *CertificatePrivate {
	return &CertificatePrivate{targets: targets, SavePath: savePath, challengeRecords: make(map[string]createdChallenge)}
}

func hashChallengeDomain(domain string) string {
	h := sha256.Sum256([]byte(strings.TrimPrefix(strings.ToLower(domain), "*.")))
	return hex.EncodeToString(h[:16])
}

func delegatedChallengeRecordName(domain string) string {
	name := hashChallengeDomain(domain)
	if prefix := strings.Trim(AccountConfig.Certificate.ApplyPrefix, ". "); prefix != "" {
		return prefix + "." + name
	}
	return name
}

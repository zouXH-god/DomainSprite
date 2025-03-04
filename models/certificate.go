package models

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var EmailIndex = 0

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
	if m.Client == nil || m.RequestCount >= m.MaxRequests {
		// 获取一个邮箱
		m.Email = AccountConfig.Certificate.EmailList[EmailIndex]
		m.MaxRequests = AccountConfig.Certificate.MaxRequest
		// 创建新客户端
		config := lego.NewConfig(m)
		config.CADirURL = lego.LEDirectoryProduction    // 使用 Let's Encrypt 生产环境
		config.Certificate.KeyType = certcrypto.RSA2048 // 设置默认密钥类型
		client, err := lego.NewClient(config)
		if err != nil {
			return nil, fmt.Errorf("创建 ACME 客户端失败: %v", err)
		}
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("注册用户失败: %v", err)
		}
		m.Resource = reg
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

type CertificatePrivate struct {
	provider RecordProvider
	domain   DomainInfo
	SavePath string
}

// Present 添加 TXT 记录以完成 DNS-01 挑战
func (p *CertificatePrivate) Present(domain, token, keyAuth string) error {
	// TODO 实现添加 TXT 记录提供 CNAME 的逻辑

	// 解析挑战信息
	fqdn := dns01.GetChallengeInfo(domain, keyAuth)
	rr := strings.TrimSuffix(fqdn.FQDN, "."+domain+".")

	// 构造 TXT 记录
	record := RecordInfo{
		DomainId:      p.domain.Id,
		DomainName:    domain,
		RecordName:    rr,
		RecordType:    "TXT",
		RecordContent: fqdn.Value,
	}

	// 添加记录
	_, err := p.provider.AddRecord(record)
	if err != nil {
		return fmt.Errorf("添加 TXT 记录失败: %v", err)
	}
	return nil
}

// CleanUp 删除 TXT 记录
func (p *CertificatePrivate) CleanUp(domain, token, keyAuth string) error {
	// 解析挑战信息
	fqdn := dns01.GetChallengeInfo(domain, keyAuth)
	rr := strings.TrimSuffix(fqdn.FQDN, "."+domain+".")

	// 搜索现有记录
	search := DNSSearch{
		DomainId:    p.domain.Id,
		DomainName:  domain,
		RRKeyWord:   rr,
		TypeKeyWord: "TXT",
	}
	records, err := p.provider.GetRecordList(search)
	if err != nil {
		return fmt.Errorf("获取记录列表失败: %v", err)
	}

	// 删除匹配的记录
	for _, record := range records {
		if fqdn.Value == keyAuth {
			_, err := p.provider.DeleteRecord(domain, record.Id)
			if err != nil {
				return fmt.Errorf("删除 TXT 记录失败: %v", err)
			}
		}
	}
	return nil
}

func (p *CertificatePrivate) GetResourcePath() string {
	return filepath.Join(p.SavePath, "resource.json")
}

// SaveCertificate 保存证书
func (p *CertificatePrivate) SaveCertificate(certificates *certificate.Resource) (*Resource, error) {
	resource := Resource{}
	resource.Resource = *certificates
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
	err = os.WriteFile(resource.CertificatePath, certificates.Certificate, 0644)
	if err != nil {
		return &resource, fmt.Errorf("保存证书失败: %v", err)
	}
	err = os.WriteFile(resource.PrivateKeyPath, certificates.PrivateKey, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	err = os.WriteFile(resource.IssuerCertPath, certificates.IssuerCertificate, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	err = os.WriteFile(resource.CSRPath, certificates.CSR, 0600)
	if err != nil {
		return &resource, fmt.Errorf("保存密钥失败: %v", err)
	}
	// 序列化 JSON（包含路径字段）
	jsonData, err := json.MarshalIndent(resource, "", "    ")
	if err != nil {
		return &resource, err
	}
	// 保存 JSON 文件
	if err = os.WriteFile(p.GetResourcePath(), jsonData, 0644); err != nil {
		return &resource, err
	}
	return &resource, nil
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
		provider: recordProvider,
		domain:   domain,
		SavePath: SavePath,
	}
}

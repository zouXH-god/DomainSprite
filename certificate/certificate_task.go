package certificate

import (
	"DDNSServer/DDNS"
	"DDNSServer/db"
	"DDNSServer/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	legocert "github.com/go-acme/lego/v4/certificate"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

const TypeCertificateCreate = "certificate:create"

var ErrQueueUnavailable = errors.New("证书任务队列不可用")
var ErrRenewalActive = errors.New("该证书已有续期任务")

type loggerKey struct{}
type CreatePayload struct {
	CertificateID int    `json:"certificateId"`
	TaskDataID    int    `json:"taskDataId"`
	TaskID        string `json:"taskId"`
}

func loadStagedResource(path string) (*models.Resource, error) {
	r, err := (&models.CertificatePrivate{SavePath: path}).LoadResource()
	if err == nil && len(r.Certificate) > 0 && len(r.PrivateKey) > 0 {
		return r, nil
	}
	cert, ce := os.ReadFile(filepath.Join(path, "certificate.crt"))
	key, ke := os.ReadFile(filepath.Join(path, "private.key"))
	if ce != nil || ke != nil || len(cert) == 0 || len(key) == 0 {
		return nil, errors.New("staging 签发结果不完整")
	}
	r = &models.Resource{SavePath: path, CertificatePath: filepath.Join(path, "certificate.crt"), PrivateKeyPath: filepath.Join(path, "private.key"), IssuerCertPath: filepath.Join(path, "issuer.crt"), CSRPath: filepath.Join(path, "csr.csr")}
	r.Certificate = cert
	r.PrivateKey = key
	r.IssuerCertificate, _ = os.ReadFile(r.IssuerCertPath)
	r.CSR, _ = os.ReadFile(r.CSRPath)
	return r, nil
}

var queue struct {
	sync.Mutex
	client *asynq.Client
	server *asynq.Server
}

func InitTaskProcessor() error {
	queue.Lock()
	defer queue.Unlock()
	if queue.client != nil {
		return nil
	}
	opt := asynq.RedisClientOpt{Addr: models.AccountConfig.BaseConfig.RedisPoint}
	client := asynq.NewClient(opt)
	if err := client.Ping(); err != nil {
		_ = client.Close()
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	server := asynq.NewServer(opt, asynq.Config{Concurrency: models.AccountConfig.Certificate.ConcurrencyTask, Queues: map[string]int{"default": 1}, ShutdownTimeout: 30 * time.Second, ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
		attrs := []any{"event", "task.handler_failed", "error", err, "task_type", task.Type()}
		var p CreatePayload
		if json.Unmarshal(task.Payload(), &p) == nil {
			attrs = append(attrs, "task_id", p.TaskID, "certificate_id", p.CertificateID)
		}
		if retry, ok := asynq.GetRetryCount(ctx); ok {
			attrs = append(attrs, "attempt", retry+1)
		}
		slog.Error("certificate task failed", attrs...)
	})})
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeCertificateCreate, HandleCertificateCreateTask)
	queue.client, queue.server = client, server
	go func() {
		if err := server.Run(mux); err != nil {
			slog.Error("任务处理器停止", "error", err)
		}
	}()
	return nil
}
func ShutdownTaskProcessor() {
	queue.Lock()
	defer queue.Unlock()
	if queue.server != nil {
		queue.server.Shutdown()
		queue.server = nil
	}
	if queue.client != nil {
		_ = queue.client.Close()
		queue.client = nil
	}
}

func ReloadTaskProcessor() error { ShutdownTaskProcessor(); return InitTaskProcessor() }

func TaskQueueHealthy() error {
	queue.Lock()
	client := queue.client
	queue.Unlock()
	if client == nil {
		return ErrQueueUnavailable
	}
	return client.Ping()
}

func enqueueTask(taskID string, payload CreatePayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	queue.Lock()
	client := queue.client
	queue.Unlock()
	if client == nil {
		return ErrQueueUnavailable
	}
	_, err = client.Enqueue(asynq.NewTask(TypeCertificateCreate, b), asynq.TaskID(taskID), asynq.Queue("default"), asynq.MaxRetry(models.AccountConfig.Certificate.TaskMaxRetry), asynq.Timeout(time.Duration(models.AccountConfig.Certificate.TaskTimeoutMinutes)*time.Minute))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQueueUnavailable, err)
	}
	return nil
}

// EnqueueCertificate preserves the legacy same-account API.
func EnqueueCertificate(accountName string, domains []models.DomainInfo, managed []models.Domains) (models.Certificate, error) {
	bindings := make([]models.CertificateDomain, 0, len(domains))
	byName := map[string]models.Domains{}
	for _, d := range managed {
		byName[d.DomainName] = d
	}
	for _, d := range domains {
		m := byName[d.DomainName]
		bindings = append(bindings, models.CertificateDomain{DomainName: d.DomainName, AccountName: accountName, ProviderType: d.DnsFrom, ProviderDomainID: d.Id, ChallengeMode: "direct"})
		if m.Id != "" {
			bindings[len(bindings)-1].ProviderDomainID = m.Id
			bindings[len(bindings)-1].ProviderType = m.DnsFrom
		}
	}
	return EnqueueCertificateDomains(bindings, 0, "issue")
}

func EnqueueCertificateDomains(input []models.CertificateDomain, parentID int, kind string) (models.Certificate, error) {
	domains, sans, fingerprint, err := NormalizeCertificateDomains(input)
	if err != nil {
		return models.Certificate{}, err
	}
	taskID, lineage := uuid.NewString(), uuid.NewString()
	var parent models.Certificate
	if parentID != 0 {
		if err := db.DB.First(&parent, parentID).Error; err != nil {
			return models.Certificate{}, err
		}
		lineage = parent.LineageID
		if lineage == "" {
			lineage = fmt.Sprint(parent.Id)
		}
	}
	cert := models.Certificate{ParentID: uint(parentID), LineageID: lineage, State: "wait", Stage: "wait", TaskId: taskID, DomainsFingerprint: fingerprint, DomainList: joinSANs(sans)}
	taskData := models.CertificateTask{}
	var active *string
	if kind == "renew" {
		k := "renew:" + lineage
		active = &k
	}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&cert).Error; err != nil {
			return err
		}
		for i := range domains {
			domains[i].CertificateID = cert.Id
		}
		if err := tx.Create(&domains).Error; err != nil {
			return err
		}
		taskData = models.CertificateTask{CertId: cert.Id, TaskId: taskID, CreateTime: time.Now(), State: "wait", Kind: kind, ActiveKey: active, LogPath: filepath.Join(models.AccountConfig.Certificate.SavePath, "logs", taskID+".log")}
		return tx.Create(&taskData).Error
	})
	if err != nil {
		if kind == "renew" {
			return models.Certificate{}, fmt.Errorf("%w: %v", ErrRenewalActive, err)
		}
		return models.Certificate{}, err
	}
	if err = enqueueTask(taskID, CreatePayload{CertificateID: cert.Id, TaskDataID: taskData.Id, TaskID: taskID}); err != nil {
		_ = db.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Delete(&models.CertificateDomain{}, "certificate_id = ?", cert.Id).Error; e != nil {
				return e
			}
			if e := tx.Delete(&models.CertificateTask{}, taskData.Id).Error; e != nil {
				return e
			}
			return tx.Delete(&models.Certificate{}, cert.Id).Error
		})
		return models.Certificate{}, err
	}
	if logger, closeLog, logErr := taskLogger(CreatePayload{CertificateID: cert.Id, TaskDataID: taskData.Id, TaskID: taskID}); logErr == nil {
		logger.Info("task.enqueued", "kind", kind, "lineage_id", lineage, "domain_count", len(domains), "san_count", len(sans))
		closeLog()
	}
	return cert, nil
}
func joinSANs(v []string) string { b, _ := json.Marshal(v); return string(b) }

func providersFor(domains []models.CertificateDomain) (map[string]models.ChallengeTarget, error) {
	providers := map[string]models.RecordProvider{}
	targets := map[string]models.ChallengeTarget{}
	for _, d := range domains {
		accountName := d.AccountName
		if d.ChallengeMode == "delegated" {
			accountName = models.AccountConfig.Certificate.ApplyAccount
		}
		p := providers[accountName]
		if p == nil {
			a, err := DDNS.GetAccount(accountName)
			if err != nil {
				return nil, err
			}
			p, err = DDNS.NewBaseProvider(a)
			if err != nil {
				return nil, err
			}
			providers[accountName] = p
		}
		targets[d.DomainName] = models.ChallengeTarget{Provider: p, Domain: models.DomainInfo{Domains: models.Domains{Id: d.ProviderDomainID, DomainName: d.DomainName, AccountName: d.AccountName, DnsFrom: d.ProviderType}}, Mode: d.ChallengeMode}
	}
	return targets, nil
}

func taskLogger(p CreatePayload) (*slog.Logger, func(), error) {
	path := filepath.Join(models.AccountConfig.Certificate.SavePath, "logs", p.TaskID+".log")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, f), nil)).With("task_id", p.TaskID, "certificate_id", p.CertificateID), func() { _ = f.Close() }, nil
}

func HandleCertificateCreateTask(ctx context.Context, task *asynq.Task) (err error) {
	started := time.Now()
	var p CreatePayload
	if err = json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("反序列化任务: %w", err)
	}
	logger, closeLog, err := taskLogger(p)
	if err != nil {
		return err
	}
	defer closeLog()
	ctx = context.WithValue(ctx, loggerKey{}, logger)
	retry, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	logger.Info("task.received", "attempt", retry+1, "max_retry", maxRetry+1, "task_type", task.Type())
	defer func() {
		if err != nil {
			logger.Error("task.attempt_failed", "attempt", retry+1, "stage", currentStage(p.CertificateID), "duration_ms", time.Since(started).Milliseconds(), "error", err)
		} else {
			logger.Info("task.attempt_completed", "attempt", retry+1, "duration_ms", time.Since(started).Milliseconds())
		}
	}()
	var cert models.Certificate
	var taskData models.CertificateTask
	if err = db.DB.First(&cert, p.CertificateID).Error; err != nil {
		logger.Error("task.certificate_load_failed", "error", err)
		return err
	}
	if err = db.DB.First(&taskData, p.TaskDataID).Error; err != nil {
		logger.Error("task.record_load_failed", "error", err)
		return err
	}
	logger = logger.With("task_kind", taskData.Kind, "lineage_id", cert.LineageID)
	ctx = context.WithValue(ctx, loggerKey{}, logger)
	logger.Info("task.context_loaded", "stage", cert.Stage, "state", cert.State)
	if err = db.DB.Model(&models.CertificateTask{}).Where("id = ?", taskData.Id).Updates(map[string]any{"attempt": gorm.Expr("attempt + 1"), "state": "apply"}).Error; err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		retry, rok := asynq.GetRetryCount(ctx)
		max, mok := asynq.GetMaxRetry(ctx)
		if rok && mok && retry < max {
			logger.Warn("task.retry_scheduled", "attempt", retry+1, "next_attempt", retry+2, "max_attempts", max+1, "stage", currentStage(cert.Id), "error", err)
			return
		}
		msg := err.Error()
		stateErr := db.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&models.Certificate{}).Where("id = ?", cert.Id).Updates(map[string]any{"state": "fail", "stage": "fail", "last_error": msg}).Error; e != nil {
				return e
			}
			return tx.Model(&models.CertificateTask{}).Where("id = ?", taskData.Id).Updates(map[string]any{"state": "fail", "result": msg, "active_key": nil}).Error
		})
		if stateErr != nil {
			logger.Error("task.failure_state_write_failed", "error", stateErr)
		} else {
			logger.Error("task.failed", "attempt", retry+1, "stage", "fail", "error", err)
		}
	}()
	if cert.Stage == "success" {
		logger.Info("task.already_completed", "stage", cert.Stage)
		return db.DB.Model(&models.CertificateTask{}).Where("id = ?", taskData.Id).Updates(map[string]any{"state": "success", "active_key": nil, "result": ""}).Error
	}
	var domains []models.CertificateDomain
	if err = db.DB.Where("certificate_id = ?", cert.Id).Find(&domains).Error; err != nil {
		logger.Error("task.domains_load_failed", "stage", cert.Stage, "error", err)
		return err
	}
	_, sans, fingerprint, err := NormalizeCertificateDomains(domains)
	if err != nil {
		return err
	}
	if fingerprint != cert.DomainsFingerprint {
		logger.Error("task.domain_fingerprint_mismatch", "stage", cert.Stage)
		return errors.New("证书域名指纹不一致")
	}
	staging := filepath.Join(models.AccountConfig.Certificate.SavePath, "staging", p.TaskID)
	final := filepath.Join(models.AccountConfig.Certificate.SavePath, "certificates", fmt.Sprint(cert.Id))
	if cert.Stage == "wait" || cert.Stage == "challenging" {
		if staged, loadErr := loadStagedResource(staging); loadErr == nil {
			if _, parseErr := ParseCertificate(&cert, staged); parseErr == nil {
				now := time.Now()
				if e := db.DB.Model(&models.Certificate{}).Where("id = ?", cert.Id).Updates(map[string]any{"stage": "issued", "issued_at": &now}).Error; e != nil {
					return e
				}
				cert.Stage = "issued"
				logger.Info("staging.recovered", "stage", "issued", "path", staging)
			}
		}
	}
	if cert.Stage == "wait" || cert.Stage == "challenging" {
		logger.Info("task.stage_started", "stage", "challenging", "domain_count", len(domains), "san_count", len(sans))
		if err = db.DB.Model(&models.Certificate{}).Where("id = ?", cert.Id).Updates(map[string]any{"stage": "challenging", "state": "apply"}).Error; err != nil {
			return err
		}
		targets, e := providersFor(domains)
		if e != nil {
			return e
		}
		challengeProvider := models.NewMultiProvider(targets, staging)
		challengeProvider.Logger = logger
		challengeProvider.CleanupFailure = func(target models.ChallengeTarget, record models.RecordInfo, cleanupErr error) {
			_ = db.DB.Create(&models.ChallengeCleanup{TaskID: p.TaskID, AccountName: target.Provider.GetAccountInfo().Name, DomainName: record.DomainName, RecordID: record.Id, RecordName: record.RecordName, RecordValue: record.RecordContent, LastError: cleanupErr.Error(), CreatedAt: time.Now()}).Error
		}
		var issued *legocert.Resource
		if cert.ParentID != 0 {
			var parent models.Certificate
			if e = db.DB.First(&parent, cert.ParentID).Error; e != nil {
				return e
			}
			existing, e := (&models.CertificatePrivate{SavePath: parent.SavePath}).LoadResource()
			if e != nil {
				return e
			}
			issued, e = IssueRenewedCertificate(ctx, challengeProvider, &existing.Resource, sans)
		} else {
			issued, e = IssueCertificate(ctx, challengeProvider, sans)
		}
		if e != nil {
			return e
		}
		if _, e = models.NewMultiProvider(nil, staging).SaveCertificate(issued); e != nil {
			logger.Error("staging.persist_failed", "stage", "issued", "error", e)
			return e
		}
		now := time.Now()
		if e = db.DB.Model(&models.Certificate{}).Where("id = ?", cert.Id).Updates(map[string]any{"stage": "issued", "issued_at": &now}).Error; e != nil {
			return e
		}
		cert.Stage = "issued"
		logger.Info("staging.persisted", "stage", "issued", "path", staging)
	}
	if cert.Stage == "issued" {
		logger.Info("task.stage_started", "stage", "persisted")
		stagingResource, e := loadStagedResource(staging)
		if e != nil {
			return e
		}
		finalResource, e := (&models.CertificatePrivate{SavePath: final}).SaveCertificate(&stagingResource.Resource)
		if e != nil {
			return e
		}
		parsed, e := ParseCertificate(&cert, finalResource)
		if e != nil {
			return e
		}
		parsed.Stage = "persisted"
		parsed.SavePath = final
		if e = db.DB.Save(parsed).Error; e != nil {
			return e
		}
		cert = *parsed
		logger.Info("certificate.persisted", "stage", "persisted", "path", final, "not_after", parsed.NotAfter)
	}
	if cert.Stage == "persisted" {
		logger.Info("database.commit_started", "stage", "persisted", "domain_count", len(domains))
		now := time.Now()
		err = db.DB.Transaction(func(tx *gorm.DB) error {
			updates := map[string]any{"stage": "success", "state": "success", "activated_at": &now, "last_error": ""}
			if e := tx.Model(&models.Certificate{}).Where("id = ?", cert.Id).Updates(updates).Error; e != nil {
				return e
			}
			if cert.ParentID != 0 {
				if e := tx.Model(&models.Certificate{}).Where("id = ?", cert.ParentID).Update("retired_at", &now).Error; e != nil {
					return e
				}
			}
			for _, d := range domains {
				if d.ChallengeMode == "direct" && d.ProviderDomainID != "" {
					if e := tx.Model(&models.Domains{}).Where("id = ? AND account_name = ?", d.ProviderDomainID, d.AccountName).Update("certificate_id", cert.Id).Error; e != nil {
						return e
					}
				}
			}
			return tx.Model(&models.CertificateTask{}).Where("id = ?", taskData.Id).Updates(map[string]any{"state": "success", "active_key": nil, "result": ""}).Error
		})
		if err == nil {
			logger.Info("certificate.completed", "stage", "success", "kind", taskData.Kind, "activated_at", now)
		}
		return err
	}
	return nil
}

func currentStage(certificateID int) string {
	if certificateID == 0 {
		return "unknown"
	}
	var c models.Certificate
	if db.DB.Select("stage").First(&c, certificateID).Error != nil {
		return "unknown"
	}
	return c.Stage
}

func EnqueueRenewal(certificateID int) (models.Certificate, error) {
	var parent models.Certificate
	if err := db.DB.First(&parent, certificateID).Error; err != nil {
		return models.Certificate{}, err
	}
	if parent.Stage != "success" {
		return models.Certificate{}, errors.New("只能续期成功证书")
	}
	var domains []models.CertificateDomain
	if err := db.DB.Where("certificate_id = ?", parent.Id).Find(&domains).Error; err != nil {
		return models.Certificate{}, err
	}
	return EnqueueCertificateDomains(domains, parent.Id, "renew")
}

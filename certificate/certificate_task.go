package certificate

import (
	"DDNSServer/models"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const TypeCertificateCreate = "certificate:create"

type CreatePayload struct {
	Provider       models.RecordProvider
	DomainInfoList []models.DomainInfo
	TaskID         string
	LogPath        string
	Certificate    models.Certificate
}

func NewCertificateCreateTask(provider models.RecordProvider, domains []models.DomainInfo, certificateInfo models.Certificate, TaskId string) (*asynq.Task, error) {
	// 创建日志文件路径
	logDir := filepath.Join(models.AccountConfig.Certificate.SavePath, "logs", time.Now().Format("2006-01-02"))
	logPath := filepath.Join(logDir, TaskId+".log")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		slog.Log(context.Background(), slog.LevelError, "创建日志目录失败", err)
	}
	// 创建任务负载
	payload, err := json.Marshal(CreatePayload{
		Provider:       provider,
		DomainInfoList: domains,
		TaskID:         TaskId,
		LogPath:        logPath,
		Certificate:    certificateInfo,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCertificateCreate, payload), nil
}

func HandleCertificateCreateTask(ctx context.Context, task *asynq.Task) error {
	var p CreatePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("反序列化任务负载失败: %w", err)
	}

	// 初始化日志文件输出
	logFile, err := os.OpenFile(p.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %w", err)
	}
	defer logFile.Close()

	// 创建多输出日志器: 同时输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// 设置slog以使用自定义输出
	logger := slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// 将logger存入context以便其他函数使用
	ctx = context.WithValue(ctx, "logger", logger)

	var domainNames []string
	for _, domain := range p.DomainInfoList {
		domainNames = append(domainNames, domain.DomainName)
	}
	// 记录任务开始
	logger.Info("开始处理证书创建任务",
		"task_id", p.TaskID,
		"domains", len(p.DomainInfoList),
		"provider", strings.Join(domainNames, ","))

	// 执行实际的证书创建
	certData, err := CreateCertificate(ctx, p.Provider, p.DomainInfoList)
	if err != nil {
		logger.Error("证书创建失败", "error", err)
		return fmt.Errorf("证书创建失败: %w", err)
	}

	// 保存证书
	_, err = ParseCertificateAndSaveDb(ctx, certData, &p.Certificate)
	if err != nil {
		logger.Error("证书保存失败", "error", err)
		return fmt.Errorf("证书保存失败: %w", err)
	}

	logger.Info("证书创建任务完成",
		"task_id", p.TaskID,
		"domains", len(p.DomainInfoList))

	return nil
}

func startTaskProcessor() {
	redisOpt := asynq.RedisClientOpt{
		Addr: models.AccountConfig.BaseConfig.RedisPoint,
	}

	// 任务优先级分配
	Concurrency := models.AccountConfig.Certificate.ConcurrencyTask
	ConcurrencyCritical := int(float32(models.AccountConfig.Certificate.ConcurrencyTask) * 0.6)
	ConcurrencyDefault := int(float32(models.AccountConfig.Certificate.ConcurrencyTask) * 0.3)
	ConcurrencyLow := Concurrency - ConcurrencyCritical - ConcurrencyDefault
	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: Concurrency, // 并发处理数
			Queues: map[string]int{
				"critical": ConcurrencyCritical,
				"default":  ConcurrencyDefault,
				"low":      ConcurrencyLow,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeCertificateCreate, HandleCertificateCreateTask)

	if err := srv.Run(mux); err != nil {
		slog.Log(context.Background(), slog.LevelError, "could not run server", err)
	}
}

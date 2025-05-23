package certificate

import (
	"DDNSServer/db"
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
	TaskDataId     int
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
	// 记录任务信息
	taskData := models.CertificateTask{
		TaskId:     TaskId,
		CreateTime: time.Now(),
		LogPath:    logPath,
		State:      "wait",
	}
	err = db.DB.Create(&taskData).Error
	if err != nil {
		slog.Log(context.Background(), slog.LevelError, "创建任务记录失败", err)
		return nil, err
	}
	// 创建任务负载
	payload, err := json.Marshal(CreatePayload{
		Provider:       provider,
		DomainInfoList: domains,
		TaskDataId:     taskData.Id,
		TaskID:         TaskId,
		LogPath:        logPath,
		Certificate:    certificateInfo,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCertificateCreate, payload), nil
}

func HandleCertificateCreateTask(ctx context.Context, task *asynq.Task) (err error) {
	var p CreatePayload
	if err = json.Unmarshal(task.Payload(), &p); err != nil {
		err = fmt.Errorf("反序列化任务负载失败: %w", err)
		return
	}
	// 更新任务状态
	defer func() {
		var taskData models.CertificateTask
		db.DB.Model(&models.CertificateTask{}).Where("id = ?", p.TaskDataId).First(&taskData)
		if err != nil {
			taskData.State = "fail"
			taskData.Result = err.Error()
		} else {
			taskData.State = "success"
		}
		err := db.DB.Model(&models.CertificateTask{}).Save(&taskData).Error
		if err != nil {
			slog.Log(ctx, slog.LevelError, "更新任务记录失败", err)
		}
	}()

	// 初始化日志文件输出
	logFile, err := os.OpenFile(p.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		err = fmt.Errorf("创建日志文件失败: %w", err)
		return
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
		err = fmt.Errorf("证书创建失败: %w", err)
		return
	}

	// 保存证书
	_, err = ParseCertificateAndSaveDb(ctx, certData, &p.Certificate)
	if err != nil {
		logger.Error("证书保存失败", "error", err)
		err = fmt.Errorf("证书保存失败: %w", err)
		return
	}

	logger.Info("证书创建任务完成",
		"task_id", p.TaskID,
		"domains", len(p.DomainInfoList))

	return nil
}

func StartTaskProcessor() {
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

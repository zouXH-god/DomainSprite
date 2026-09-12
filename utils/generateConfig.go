package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func EnsureConfig(configPath, config string) (bool, error) {
	if _, err := os.Stat(configPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return false, fmt.Errorf("生成配置文件: %w", err)
	}
	return true, nil
}

// EnsureMasterKey makes the configured database encryption key available to
// the current process. An explicitly supplied environment variable always
// wins. Otherwise the key is loaded from, or generated into, a sidecar file so
// encrypted database values remain readable after a restart.
func EnsureMasterKey(configPath, envName string) (bool, string, error) {
	if strings.TrimSpace(envName) == "" {
		envName = "DOMAINSPRITE_MASTER_KEY"
	}
	if strings.TrimSpace(os.Getenv(envName)) != "" {
		return false, "environment", nil
	}
	keyPath := configPath + ".master-key"
	if data, err := os.ReadFile(keyPath); err == nil {
		value := strings.TrimSpace(string(data))
		decoded, decodeErr := base64.StdEncoding.DecodeString(value)
		if decodeErr != nil || len(decoded) != 32 {
			return false, keyPath, fmt.Errorf("主密钥文件 %s 损坏", keyPath)
		}
		if err = os.Setenv(envName, value); err != nil {
			return false, keyPath, fmt.Errorf("设置环境变量 %s: %w", envName, err)
		}
		return false, keyPath, nil
	} else if !os.IsNotExist(err) {
		return false, keyPath, fmt.Errorf("读取主密钥文件: %w", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return false, keyPath, fmt.Errorf("生成数据库主密钥: %w", err)
	}
	value := base64.StdEncoding.EncodeToString(raw)
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, keyPath, fmt.Errorf("创建主密钥目录: %w", err)
	}
	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		// Another process may have won first-start initialization. Load its key
		// instead of overwriting it.
		if os.IsExist(err) {
			return EnsureMasterKey(configPath, envName)
		}
		return false, keyPath, fmt.Errorf("创建主密钥文件: %w", err)
	}
	_, writeErr := f.WriteString(value + "\n")
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(keyPath)
		return false, keyPath, fmt.Errorf("保存主密钥: %w", writeErr)
	}
	if err = restrictSecretFile(keyPath); err != nil {
		_ = os.Remove(keyPath)
		return false, keyPath, fmt.Errorf("限制主密钥文件权限: %w", err)
	}
	if err = os.Setenv(envName, value); err != nil {
		return false, keyPath, fmt.Errorf("设置环境变量 %s: %w", envName, err)
	}
	return true, keyPath, nil
}

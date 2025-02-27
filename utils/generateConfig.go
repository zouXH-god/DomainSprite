package utils

import (
	"fmt"
	"os"
)

func InitConfig(config string) bool {
	configPath := "config.toml"
	if _, err := os.ReadFile(configPath); err != nil {
		fmt.Println("未找到配置文件，正在为你生成配置文件...")
		os.WriteFile(configPath, []byte(config), 0644)
		println(configPath + " 配置文件已生成，请修改后重新启动程序")
		return true
	}
	return false
}

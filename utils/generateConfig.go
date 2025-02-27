package utils

import (
	"fmt"
	"os"
)

var config = `[baseConfig]
Host = "127.0.0.1"
Port = "2485"
AccessKeyId="P7yJRXMNBpeaCNAE47ZQ"
AccessKeySecret="wRfD7GLzPyyBTJJWGQKFWmKhkoAJDKki"

# 快速解析配置
[fastConfig]
UseAccount="account1"  # 要使用的账户
DomainId=""  # 要使用的域名id
DomainName=""  # 要使用的域名
NameStrata="server_a_" # 解析前缀
IdLength=5  # 解析的id长度
StartId=1  # 解析起始id
DataPath="./data/"  # 数据保存目录
AccessSalt="3Uq3nfRZemVnYhvcpFaufDmZxPCAz8ou"

[[account]]
Name="account1"  # 账户名称（自定义）
Type="Ali"  # 云服务商类型，目前仅支持 Ali | Tencent | Cloudflare
AccessKeyId="阿里云AKID"
AccessKeySecret="阿里云AKSecret"

[[account]]
Name="account2"
Type="Cloudflare"
AccessKeyId="Cloudflare账号ID"
AccessKeySecret="Cloudflare邮箱"

[[account]]
Name="account3"
Type="Tencent"
AccessKeyId="腾讯云AKID"
AccessKeySecret="腾讯云AKSecret"`

func InitConfig() bool {
	configPath := "config.yaml"
	if _, err := os.ReadFile(configPath); err != nil {
		fmt.Println("Error Read Config:", err)
		os.WriteFile(configPath, []byte(config), 0644)
		println(configPath + " 配置文件已生成，请修改后重新启动程序")
		return true
	}
	return false
}

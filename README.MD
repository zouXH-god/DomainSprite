# 🌟 DomainSprite README 🎮

欢迎来到 **DomainSprite** 的魔法世界！🌸✨

在这个世界中，你将能轻松管理你的 DNS 记录，快速管理多个云服务账户，甚至将域名和记录像魔法一样自由操控！🎩🔮

准备好了吗？让我们一起探索如何使用这个超强的工具吧！🚀

## 🏰 功能亮点

- **多云支持**：你可以同时管理 **阿里云**、**腾讯云** 和 **Cloudflare** 账户，就像拥有多个魔法卷轴，随时呼唤你的 DNS 记录！🌐
- **快速配置**：只需简单配置文件，就能一键开启魔法。📜✨
- **快捷 API**：通过强大的 API 接口，管理你的域名和记录，轻松如掌中宝！⚡
- **无缝衔接**: 轻松管理你的 DNS 记录，与多个云服务账户无缝衔接，让你像魔法师一样，轻松管理你的 DNS 记录！

## 🚀 快速开始

### 1. 安装依赖

首先，请确保你已经安装了 Go 和 Gin，开启你的魔法旅程：

```bash
go get github.com/gin-gonic/gin
```

### 2. 配置文件

> 复制 `config.yaml.example` 为 `config.yaml` 进行配置修改

打开 `baseConfig` 和 `fastConfig` 部分，配置你的魔法阵：

```toml
[baseConfig]
Host = "127.0.0.1"
Port = "2485"
AccessKeyId = "你的魔法钥匙ID"
AccessKeySecret = "你的魔法密钥"
```

在 `fastConfig` 部分，设置账户和数据路径等信息：

```toml
[fastConfig]
UseAccount = "account1"  # 使用的账户
DomainId = ""            # 要使用的域名ID
DomainName = ""          # 要使用的域名
NameStrata = "server_a_" # 解析前缀
IdLength = 5             # 解析ID的长度
StartId = 1              # 解析起始ID
DataPath = "./data/"     # 数据保存目录
AccessSalt = "你的快速请求盐"
```

### 3. 账户管理

你可以在文件中设置多个账户，像收集角色卡一样！🌟

```toml
[[account]]
Name = "account1"
Type = "Ali"
AccessKeyId = "阿里云AKID"
AccessKeySecret = "阿里云AKSecret"

[[account]]
Name = "account2"
Type = "Cloudflare"
AccessKeyId = "Cloudflare账号ID"
AccessKeySecret = "Cloudflare邮箱"

[[account]]
Name = "account3"
Type = "Tencent"
AccessKeyId = "腾讯云AKID"
AccessKeySecret = "腾讯云AKSecret"
```

### 4. API 使用 - 魔法操作！

通过下面的 API，你可以像召唤魔法一样，轻松操作你的 DNS 记录和域名：

#### 域名管理

- **获取账户列表**  
  `GET /api/accounts`  
  获取所有账户的列表。

- **获取账户域名列表**  
  `GET /api/:accountName/domains`  
  获取指定账户的域名列表。

- **获取账户的 DNS 记录列表**  
  `GET /api/:accountName/records`  
  获取指定账户的 DNS 记录列表。

- **添加 DNS 记录**  
  `POST /api/:accountName/record`  
  添加一条新的 DNS 记录。

- **更新 DNS 记录**  
  `PUT /api/:accountName/record`  
  修改一条 DNS 记录。

- **删除 DNS 记录**  
  `DELETE /api/:accountName/record`  
  删除一条 DNS 记录。

- **修改 DNS 记录状态**  
  `PUT /api/:accountName/record/status`  
  启用或禁用 DNS 记录。

#### 快速请求

如果你需要快速更新记录，只需动动手指：

- **创建 A 记录并返回 Token**  
  `GET /fast/ip2a`  
  创建一条 A 记录并返回快速验证 Token。

- **使用 Token 更新记录**  
  `GET /fast/updateRecord`  
  根据 Token 更新 DNS 记录。

## 🔒 鉴权说明 - 魔法钥匙🔑

为了保护你的魔法，所有 API 请求都需要进行 **鉴权**。当你发送请求时，需要传递 **AccessKeyId** 和 **AccessKeySecret**，这是你的魔法钥匙！⚔️

### API 鉴权逻辑

- **AccessKeyId** 和 **AccessKeySecret** 需要匹配系统配置中的值。如果这些值为空，或者不匹配，将会阻止请求继续进行。
- **FastAuthentication**：对于快速请求，需要提供 **AccessSalt** 进行验证。

#### 示例代码

```go
func ApiAuthentication(c *gin.Context) {
	accessKeyId := c.GetHeader("AccessKeyId")
	accessKeySecret := c.GetHeader("AccessKeySecret")
	if accessKeyId == "" || accessKeySecret == "" {
		c.JSON(400, gin.H{
			"message": "AccessKeyId 或 AccessKeySecret 为空",
		})
		c.Abort()
		return
	}
	if accessKeyId != models.AccountConfig.BaseConfig.AccessKeyId || accessKeySecret != models.AccountConfig.BaseConfig.AccessKeySecret {
		c.JSON(400, gin.H{
			"message": "AccessKeyId 或 AccessKeySecret 错误",
		})
		c.Abort()
		return
	}
}

func FastAuthentication(c *gin.Context) {
	accessSalt := c.GetHeader("AccessSalt")
	if accessSalt == "" || accessSalt != models.AccountConfig.FastConfig.AccessSalt {
		c.JSON(400, gin.H{
			"message": "AccessSalt 错误",
		})
		c.Abort()
		return
	}
}
```

### 请求示例

```bash
curl -H "AccessKeyId: 你的魔法钥匙ID" -H "AccessKeySecret: 你的魔法密钥" http://127.0.0.1:2485/api/accounts
```

## 🏆 最后，别忘了...

**DomainSprite** 是你管理 DNS 记录的最佳伙伴！💫 不管你是普通用户，还是云服务管理高手，这个工具都能帮你快速解决问题。

欢迎各路大佬为 **DomainSprite** 做出贡献，一起创造更多神奇的魔法！

```plaintext
           🌸
    ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
       🌼  (｡•̀ᴗ•́｡) 🎮       快速管理你的 DNS，保护你的虚拟世界
           🌻⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
``` 

享受魔法，享受世界！🌈
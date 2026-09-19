# DomainSprite

DomainSprite 是一个支持阿里云、腾讯云和 Cloudflare 的 DNS 与证书生命周期管理服务。发布二进制已内嵌 Vue 管理控制台，启动后直接访问后端地址即可使用，无需单独部署静态文件。

## 管理控制台

- `/`：概览、DNS 解析、证书申请/续期、任务日志与设置。
- `/quick`：独立快速 DDNS 页面，不读取管理 API 凭据。
- 首次进入需填写 `AccessKeyId` 与 `AccessKeySecret`；只有明确选择保存时才应长期保留凭据。
- 证书任务页面按 `taskId` 展示结构化 JSONL 时间线，运行中每 2 秒轮询，终态自动停止。

如果部署平台提供密钥管理系统，可以显式设置持久化主密钥，例如 PowerShell：

```powershell
$env:DOMAINSPRITE_MASTER_KEY = [Convert]::ToBase64String([byte[]](1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

未设置环境变量时，程序会自动生成并复用 `config.toml.master-key`，因此本地开发无需手工设置。生产环境建议将生成值放入安全的部署密钥管理系统；更换或丢失该值会导致数据库中的 DNS/ACME 凭据无法解密。空数据库启动时，日志会输出一个 30 分钟有效的一次性 Setup Token，管理页面用它创建首个管理员。

也可以不使用环境变量，直接在 `config.toml` 中设置 32 字节密钥的 Base64 或 Hex 值：

```toml
[baseConfig]
EncryptionKeyEnv=""
EncryptionKey="替换为32字节随机密钥的Base64或Hex值"
```

`EncryptionKey` 非空时优先使用；否则读取 `EncryptionKeyEnv` 指定的环境变量，再回退到自动生成的 `config.toml.master-key`。直接配置会将主密钥保存在 TOML 中，请确保文件权限为 `0600`、不提交到版本库，并与数据库一起安全备份。

DNS 厂商账户、ACME Profile、AccessKey、用户、域名授权节点和运行参数均保存在 SQLite。旧 TOML 业务配置只在升级后的第一次启动事务导入，此后数据库为唯一来源；确认导入成功后应删除 TOML 备份中的旧明文凭据。

域名权限支持 `example.com`、`dev.example.com`、`api.dev.example.com` 等节点。子域节点仍映射到真实厂商 Zone，默认授权整个子树，不会建立虚假的 DNS Zone。

本地开发可运行根目录 `start.bat`。脚本检查 Go、Node.js 与 npm，安装前端依赖，并在当前控制台同时运行后端 `127.0.0.1:2485` 和 Vite `127.0.0.1:5173`，不再创建额外窗口。任一服务退出后会清理另一服务；按 `R` 可原地重启两者，按回车退出，`Ctrl+C` 停止整个开发环境。生产构建顺序为：

```text
cd webui
npm ci
npm run typecheck
npm test
npm run build
cd ..
go build .
```

前端产物位于 `webui/dist`，由 Go `embed` 编译进单文件二进制。`/assets/*` 使用 immutable 缓存，SPA 的 `index.html` 不缓存；未命中的 API 路由始终返回 JSON 404。

**DomainSprite** 是一个 DNS 记录管理工具，可以方便的管理多平台的 DNS 记录，并且集成 SSL 证书申请，可以简单的为自己的域名或其他域名进行通配符证书申请

## 🏰 功能亮点

- **多云支持**：你可以同时管理 **阿里云**、**腾讯云** 和 **Cloudflare** 账户，就像拥有多个魔法卷轴，随时呼唤你的 DNS 记录！🌐
- **快速证书申请**：仅需一个请求，就能一键申请域名的通配符证书！📜
- **不限制的证书域名**：不论是自己的域名还是其他人的域名，你都能轻松地申请通配符证书！
- **快速配置**：只需简单配置文件，就能一键开启魔法。✨
- **快捷 API**：通过强大的 API 接口，管理你的域名和记录，轻松如掌中宝！⚡
- **快速DDNS**：通过简单的 API，你可以轻松地创建一个域名解析Token。通过这个Token，你可以轻松的在其他服务器上动态的更新你的域名解析！

## TODO

- [ ] 添加更多云平台支持
- [x] 修改当前的单域名SSl证书申请为多域名通配符模式
- [ ] 修改SSl证书申请为队列模式进行
- [ ] 添加web管理面板
- [ ] 添加证书快捷下载接口方便服务器进行动态更新
- [ ] 添加证书自动续期
- [ ] Provider 进行优化，允许多账户跨账户申请证书

## 🚀 快速开始

### SQLite 升级迁移

从旧版本升级且启动日志出现 `Cannot add a PRIMARY KEY column` 时，先停止重启循环，再运行仓库提供的迁移脚本：

```bash
sudo systemctl stop domainsprite
chmod +x ./update.sh
sudo ./update.sh --db /实际工作目录/database.db --service domainsprite.service
```

脚本会停止服务、执行数据库完整性检查、创建唯一且经过校验的 SQLite 备份、事务重建旧 `domains` 表、验证主键和记录数量，最后重新启动服务。脚本可以重复执行，已经包含 `db_id` 主键的数据库不会再次迁移。

只迁移数据库、不操作 systemd：

```bash
./update.sh --db ./database.db --no-service
```

迁移失败时不要删除原数据库。脚本会输出备份路径和恢复命令；应在服务停止状态下恢复。运行脚本需要系统安装 `sqlite3`。

### 1. 下载程序包

> 前往 `release` 获取程序包，下载对应版本的程序包。

### 2. 配置文件

> 对 `config.yaml` 进行配置修改

**baseConfig**： 基础的配置信息，包括端口号、域名、密钥等

```toml
[baseConfig]
Host = "127.0.0.1"
Port = "2485"
AccessKeyId = "你的魔法钥匙ID"
AccessKeySecret = "你的魔法密钥"
```

**certificateConfig**： SSL证书申请配置，包括邮箱列表、最大申请数量、证书的保存路径、使用的账号等

```toml
[certificateConfig]
EmailList=["email@mail.com", "email2@mail.com"]  # 申请证书使用的邮箱列表
MaxRequest=20  # 单邮箱单次最大申请证书数量
SavePath="./certificateData/"  # 证书保存路径
# 第三方域名申请使用的账号与域名
ApplyAccount="account1"  # 要使用的账户
ApplyDomainId=""  # 要使用的域名id
ApplyDomainName=""  # 要使用的域名
```

**fastConfig**： 快速请求配置，改部分用于快速更新记录接口。只需要一个Token，就能轻松的更新你的记录！

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

你可以在文件中设置多个账户，只要按照当前格式继续添加即可

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

### 5. 运行

- Linux 系统执行 `chmod +x ./DomainSprite* && ./DomainSprite*`
- Windows 系统双击 `DomainSprite.exe` 运行即可

### 4. API 使用 - 魔法操作！

> #### [API文档](https://50swcphpjw.apifox.cn) <- 在这儿

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

- **一键申请通配符证书**  
  `POST /api/:accountName/certificate`  
  为指定域名申请通配符证书。

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

#### 鉴权代码参考

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

## 运行可靠性与迁移说明

- 服务按“配置校验 → 数据目录 → SQLite 迁移 → Redis 检查 → worker → HTTP”的顺序启动；Redis 不可用时不会接受证书任务。
- 首次涉及新版域名身份模型的启动会在数据库旁生成带时间戳的 `database.db.bak.*` 备份。迁移失败时服务不会启动。
- 证书状态依次为 `wait`、`apply`、`success` 或 `fail`。入队失败返回 HTTP 503，并清理本次数据库记录。
- 本项目仍按单实例部署设计；快速 DDNS 使用加锁和原子替换保护本地 JSON，但不支持多个进程共享该文件。

新的快速 DDNS 写接口为 `POST /fast/ip2a` 和 `PUT /fast/record`。更新接口应在 JSON body 中传递 `{"token":"..."}`。旧的 `GET /fast/ip2a` 和 `GET /fast/updateRecord?token=...` 暂时兼容，并通过 `Deprecation`、`Sunset`、`Link` 响应头提示迁移。

浏览器调用时，应在 `baseConfig.AllowedOrigins` 中明确列出可信来源。不要使用通配来源携带凭据。

默认测试完全离线：

```bash
go test ./...
go test -race ./...
go vet ./...
```

首次启动时若当前目录不存在 `config.toml`，程序会从内嵌模板自动创建该文件。`EncryptionKey` 非空时直接使用该值；否则，如果 `EncryptionKeyEnv` 指定的环境变量尚未设置，会生成随机 256-bit 数据库主密钥，保存为 `config.toml.master-key` 并仅注入当前进程。后续启动会复用该文件，绝不会重新生成或覆盖已有环境变量。请将实际使用的主密钥与数据库一同备份，且不要提交到版本库；遗失主密钥将无法解密数据库中的 DNS/ACME 凭据。

真实云厂商测试使用 `integration` build tag，并且仅应配置专用测试账户：

```bash
go test -tags=integration ./...
```

### 多账户证书与续期

跨账户、跨厂商申请使用：

```http
POST /certificate/apply/accounts
Content-Type: application/json

{"domains":[{"accountName":"aliyun-main","domainId":"123"},{"accountName":"cloudflare-main","domainId":"zone-id"}]}
```

服务会为每个基础域名同时申请主域名和通配符 SAN，最多接受 50 个基础域名。域名必须是不带 `*.`、协议、端口或路径的合法 DNS 名称。

手工续期使用 `POST /certificate/:id/renew`。服务每日扫描一次剩余有效期不超过 30 天的当前证书，并为续期创建新版本；新版本成功前不会替换域名正在使用的旧证书。证书阶段为 `wait → challenging → issued → persisted → success`，重试会从 staging 文件恢复，避免 CA 已签发后重复申请。

每个任务的结构化 JSONL 日志保存在 `certificateData/logs/{taskId}.log`，可通过 `GET /certificate/task/{taskId}/log` 直接查询。日志包含任务、重试、stage、DNS provider、ACME、staging 和数据库提交事件，但不会记录云密钥、challenge 值、私钥或完整证书资源。

### 独立证书节点

主控可通过 `[grpc]` 基础设施配置中的 `Enabled=true` 启用独立 gRPC TLS 监听，默认关闭。启用时必须同时配置有效的 `TLSCertFile`、`TLSKeyFile` 和供节点连接的 `PublicURL`。管理员在“证书节点”页面创建节点组与一次性注册 Token；Token 默认 10 分钟失效且注册成功后立即作废。

```text
DomainSpriteNode install --name "node-a" --remark "华东节点" --controller "https://controller.example.com:2486" --token "一次性Token"
```

缺少参数时安装程序会交互询问。Linux 自动安装并启用 systemd 服务，Windows 注册为自动启动且失败重启的 Windows Service；另支持 `run`、`status`、`restart`、`uninstall`。节点身份配置位于 `/etc/domainsprite-node/node.json` 或 `%ProgramData%\DomainSprite\node.json`，权限必须仅限系统管理员。

节点主动建立 mTLS 双向流，因此可部署在 NAT 后。调度器只选择用户获授权组中在线、未 drain、provider 能力匹配且 ACME 邮箱额度未耗尽的节点，并按 `运行任务数 / 容量` 选择。签发开始前的过期 lease 可以重新分配；进入 `challenging` 后固定原节点，避免重复消耗 CA 配额。

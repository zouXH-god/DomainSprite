# DomainSprite v1.3.2

本次版本将 DomainSprite 从单一 DNS/证书工具升级为具备 Web 控制台、多用户权限、可恢复证书生命周期和独立签发节点的管理平台。升级前请备份 `database.db`、`config.toml`、`certificateData` 和快速 DDNS 数据目录。

该补丁版本增加快速 DDNS AccessSalt 的前端安全配置入口，并将证书完整包下载名称改为根据 SAN 域名列表生成。功能内容包含 v1.3.0 和 v1.3.1 的全部更新。

## v1.3.2 更新

- 管理员可在设置页查看快速 DDNS AccessSalt 是否已配置，手工更新或生成 256-bit 随机值。
- AccessSalt 不会从服务器回显，输入框留空不会覆盖现有配置，更新值至少需要 16 个字符。
- 证书完整包采用域名列表命名，例如 `example.com_wildcard.example.com.zip`。
- ZIP 下载名会自动清理非法字符、规范化通配符、移除重复域名，并安全缩短过长 SAN 列表。

## 管理控制台

- 新增 Vue 3 + TypeScript 管理控制台，并作为静态资源嵌入主控二进制，无需单独部署前端。
- 提供概览、DNS 解析、证书、任务日志、证书节点和设置页面，支持浅色、深色及跟随系统主题。
- DNS 页面支持账号与域名对象选择、记录 CRUD、状态切换，以及关键字、类型、状态和线路组合筛选。
- 设置页的 AccessKey、用户、DNS 账号、ACME Profile、域名权限节点和授权均改为信息列表与独立创建/编辑弹窗。
- 快速 DDNS 提供独立页面，不读取后台管理凭据。

## 多用户、AccessKey 与动态配置

- 新增首个管理员一次性 Setup Token、网页登录会话、CSRF 防护和 Argon2id 密码摘要。
- 支持多用户、多组可撤销 AccessKey，以及 DNS 读取、DNS 修改、证书申请、续期和下载等能力范围。
- 新增四级域名权限：仅查看、DNS 编辑、证书管理和节点所有者。
- 支持真实 Zone 下的二级、三级权限节点及子域继承，DNS 和证书操作均按域名授权校验。
- DNS 厂商账号、ACME Profile、快速 DDNS 和证书运行参数迁移至 SQLite 动态管理。
- 敏感凭据使用主密钥派生的 AES-256-GCM 加密；AccessKey Secret 仅创建或轮换时显示一次。

## 证书生命周期

- 证书申请重构为 `wait → challenging → issued → persisted → success/fail` 可恢复状态机。
- 支持同一证书跨 DNS 账号、跨阿里云、腾讯云和 Cloudflare 完成多域名 DNS-01。
- 手工申请始终创建新版本；自动续期在到期前 30 天进入队列，成功后才切换域名关联。
- CA 已签发的结果安全写入 staging，后续数据库或文件步骤重试不会重复调用 CA。
- challenge 记录按真实 Record ID 精确清理，清理失败会记录独立事件，不误删并发 TXT 记录。
- 每次任务使用结构化 JSONL 日志，按尝试次数分组，可独立定位账号、provider、域名、阶段和失败原因。
- 支持证书在线查看、复制、下载、删除、手动续期和版本链信息。

## 独立证书节点

- 新增轻量 `DomainSpriteNode` 二进制，与主控通过主动连接的双向 gRPC 流通信，适用于 NAT 环境。
- 支持一次性注册 Token、节点身份、心跳、分组、容量、drain/resume、撤销和任务 lease。
- 支持按用户分配节点组，并按健康状态、容量、当前负载和 ACME 邮箱额度调度。
- 节点提供 `install`、`run`、`status`、`restart` 和 `uninstall` 命令；Linux/Windows 支持系统服务与开机自启。
- CI 独立构建 Linux/Windows amd64、arm64 节点产物，并检查禁止引入 Gin、GORM、SQLite、Redis和 WebUI。

## DNS、快速 DDNS 与安全

- Provider 工厂对未知类型返回明确错误，三个厂商统一使用 context、实例级缓存及精确记录 ID。
- 快速 DDNS 存储加入并发锁、原子文件替换、256-bit 随机 Token 和恒定时间比较。
- 新增 `POST /fast/ip2a` 与 `PUT /fast/record`；旧 GET 接口暂时保留并返回弃用响应头。
- API 响应、鉴权错误、依赖故障和上游错误采用统一状态码与响应结构。
- 下载接口增加类型白名单、目录边界校验、私钥禁止缓存和安全文件权限。

## 启动、构建与兼容性

- `config.toml` 不存在时自动生成；主密钥未设置时生成并持久化到忽略提交的本地密钥文件。
- `start.bat` 在当前窗口管理前后端开发服务，支持原地重启。
- 主控可在缺少 gRPC TLS 文件时为本地开发生成所需身份材料。
- CI 使用 Node.js 22 和 Go 1.22，依次执行前端类型检查、测试、构建、Go 测试与 vet，并发布六平台主控和四平台节点单文件二进制。

## 升级提示

1. 停止旧进程并完整备份数据库、配置、证书和快速 DDNS 数据。
2. 保持 `DOMAINSPRITE_MASTER_KEY` 或生成的主密钥文件长期不变；丢失后无法解密数据库凭据。
3. 首次升级会导入旧 TOML 中的业务配置。确认成功后妥善删除包含明文凭据的旧备份。
4. 空用户库启动后，从日志取得 30 分钟有效的一次性 Setup Token 创建首个管理员。
5. 独立证书节点默认按配置启用；正式环境应使用可信 TLS 证书并限制 gRPC 端口访问。

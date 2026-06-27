# TokenLive Portal — 领域上下文术语表

Portal 是 TokenLive 平台的用户自助服务门户，仅在公共 API 平台模式（Admin + Gateway + Portal）下部署。面向外部终端 API 消费者（个人开发者和企业团队），通过 OAuth 或邮箱验证码注册登录，自助管理工作空间、API Key 和用量。在企业内部工具模式（仅 Admin + Gateway）下，Portal 不参与，终端消费者直接使用 Admin 系统。

## Language

### 用户与身份

**Portal User (门户用户)**:
终端 API 消费者，包括个人开发者和企业团队成员。通过 OAuth 或邮箱验证码注册，与 Admin User 完全隔离，属于不同的用户体系。
_Avoid_: 管理员、运维人员、Admin 用户

**Account Identity (账号身份)**:
Portal User 绑定的第三方登录身份（如 GitHub、Google），一个用户可绑定多个 Provider 的身份。

### 工作空间

**Workspace (工作空间)**:
用户自助管理的组织单元，承载 API Key、余额、成员管理和模型授权。同时支持 toC 和 toB 场景：
- **个人 Workspace**：用户注册后自动创建，owner 为自己，成员只有自己。
- **团队 Workspace**：企业用户手动创建，通过邀请机制加入多个成员，适用于 toB 场景。
_Avoid_: 租户（Tenant 是 Admin 侧的网关资源管理概念）

**Workspace Member (工作空间成员)**:
Workspace 中的用户，拥有角色（如 owner、admin、member）和状态。

**Workspace Invitation (工作空间邀请)**:
通过邮箱邀请用户加入 Workspace 的机制，包含令牌、过期时间和状态管理。

### 凭证与计费

**API Key (API 密钥)**:
绑定到 Workspace 的访问凭证，用于调用 Gateway API。具有独立的限额（日/月）和消费统计。每个 API Key 存储 hash，不明文保存。
_Avoid_: Tenant API Key（那是 Admin 轻量模式的概念）

**Workspace Balance (工作空间余额)**:
Workspace 级别的预付费余额，支持乐观锁并发控制。所有 API 调用消费从此余额扣减。

**Ledger Entry (账务流水)**:
记录 Workspace 余额变动的不可变流水，包含充值、消费等类型，保证幂等性。

### 模型目录

**Model Catalog (模型目录)**:
面向终端用户展示的模型信息（含多语言、价格、服务指标），与 Admin 的 Model 是同一模型在不同视角下的呈现。

## Relationships

- 一个 **Portal User** 可拥有多个 **Workspace**（作为 owner 或 member）
- 一个 **Workspace** 可关联到一个 Admin 的 **Tenant**（通过 tenant_code，可为空）
- 一个 **Workspace** 拥有多个 **API Key**，共享同一个 **Workspace Balance**
- 一个 **API Key** 调用 Gateway 时，Gateway 通过缓存解析出 (tenant_code, workspace_id) 身份信息

## Flagged ambiguities

- Workspace 与 Tenant 的绑定字段（`workspaces.tenant_code`）当前数据库中尚未添加，需后续迁移。
- Portal User 是否需要作为 `policy_binding.user_id` 的身份来源尚未确定，暂搁置。

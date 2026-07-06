# AGENTS.md — OIDC Playground (Go)

## 项目概述

类似 Auth0 OIDC Playground 的测试站点，用于调试自搭建 Keycloak SSO。
- 部署: Ubuntu 24 + Docker Compose + 外部 NGINX
- 用户: 1-5 人内部测试，非生产环境
- 用 Go

## 技术选型

| 层 | 选择 | 说明 |
|---|------|------|
| 语言 | Go 1.22+ | 编译为单一静态二进制 |
| HTTP | `net/http` 标准库 | 无外部框架 |
| 模板 | `html/template` 标准库 | 服务端渲染，零 JS 框架 |
| OIDC | **手动实现** | 自行构造 HTTP 请求，不依赖任何 OIDC 库 |
| JWT | 手动 base64 解码 | 仅展示 claims，不做签名验证 |
| 会话 | SQLite (modernc.org/sqlite) | 纯 Go，单文件，配置 + 历史持久化 |
| 容器 | 多阶段构建 | 最终镜像 ~15MB |
| 端口 | `61000` | |

## 核心原则

### 1. 最小依赖
仅允许一个外部 Go 模块：`modernc.org/sqlite`（纯 Go 实现的 SQLite，无需 CGO）。
其余全部使用 Go 标准库。
- `net/http` — HTTP 客户端和服务端
- `html/template` — 页面渲染
- `encoding/json` — JSON 解析
- `crypto/*` — PKCE code_challenge (SHA256)、随机数生成
- `encoding/base64` — JWT 解码
- `database/sql` — 数据库接口
- `modernc.org/sqlite` — 纯 Go SQLite 驱动（唯一允许的外部依赖）

### 2. 手动实现 OIDC
不要任何 OIDC 库。自己构造 HTTP 请求：
- 调用 `/.well-known/openid-configuration` 获取端点
- 构造 authorization URL
- POST token endpoint 交换 code
- GET userinfo endpoint
- 调用 end_session_endpoint

### 3. 步骤可视化调试
每一步记录并展示（摘要级，不展示原始 HTTP 报文）：
- 时间戳 + 步骤描述 + 耗时
- HTTP Method + URL
- 关键请求参数 (脱敏 secret)
- 响应状态码
- 在结果页按时间线展示

### 4. 简单可靠
- 不做 CSRF/rate limiting/helmet
- 不做防御性编程
- Docker Compose 一键部署
- Web 表单配置，不依赖 .env
- Keycloak 同为测试环境，不过度考虑异常场景

### 5. 必须遵守
- 不确定的内容请由人工决策，并附上解释
- 禁止直接进行 Git Push 等操作，需要人工决定
## 🚫 严格禁止

以下行为在任何情况下都**不允许**，违反即视为设计错误：

### 依赖类
- ❌ 引入 `modernc.org/sqlite` 之外的**任何**第三方 Go 模块（包括但不限于：gin, echo, chi, gorilla/mux, go-oidc, coreos/go-oidc, jwt-go, golang-jwt, crypto/jwt 以外的 JWT 库, axios, gorilla/sessions, redis, boltdb, badger）
- ❌ 使用 `.env` / `config.yaml` / `config.json` 等配置文件（全部配置通过 Web 表单）
- ❌ 引入 npm/pip/cargo 等任何非 Go 的依赖管理

### 框架类
- ❌ 使用任何 Web 框架（Gin, Echo, Chi, Fiber 等），只能用 `net/http`
- ❌ 使用任何前端框架或 CSS 库（React, Vue, Tailwind, Bootstrap 等）
- ❌ 使用任何 ORM（GORM 等），只能用 `database/sql` + 手写 SQL
- ❌ 使用 Passport 或任何认证中间件

### 协议类
- ❌ 使用任何 OIDC 库实现登录流程，必须自己构造 HTTP 请求
- ❌ 添加 SAML 协议支持（项目已明确仅 OIDC）
- ❌ 对 JWT 做签名验证（仅解码展示 claims）
- ❌ 对 ID Token 做 nonce/aud/iss 验证

### 安全类
- ❌ 添加 CSRF 防护
- ❌ 添加 Rate Limiting
- ❌ 添加 Helmet / Security Headers
- ❌ 添加输入校验（除必填非空检查外）
- ❌ 添加 Cookie Secure/SameSite 标记
- ❌ 添加 HTTPS 重定向

### 工程类
- ❌ 添加 TypeScript / Babel / Webpack / Vite 等构建工具
- ❌ 添加单元测试 / 集成测试 / E2E 测试
- ❌ 添加 ESLint / Prettier / golangci-lint
- ❌ 在未与用户确认的前提下，自主添加 CI/CD 配置（GitHub Actions 等
- ❌ 添加 Docker Healthcheck
- ❌ 添加请求超时 / 重试逻辑
- ❌ 添加并发锁（sync.Mutex 等）
- ❌ 将项目拆分为超过约 3-5 个 Go 文件（main.go + oidc.go + store.go 足够）
- ❌ 添加超过约 2-4 个 HTML 模板（index.html + result.html 足够）

### 数据类
- ❌ 添加 Redis / PostgreSQL / MySQL 等外部数据库
- ❌ SQLite 文件外挂任何其他持久化方式
- ❌ JSON 配置文件存储 OIDC 配置

### 设计类
- ❌ 添加命令行参数 (`flag` 包)
- ❌ 添加日志级别 / 日志格式配置
- ❌ 添加多用户 / 权限 / RBAC 概念
- ❌ 尝试将其设计为生产环境可用的应用

## 项目结构 (~10 文件)

```
KyleworksOidcTest/
├── main.go              # 入口 + HTTP 路由 + 模板渲染 (~250行)
├── oidc.go              # OIDC 手动实现 (~200行)
├── store.go             # SQLite 存储层 (~80行)
├── go.mod               # module KyleworksOidcTest / go 1.22
├── templates/
│   ├── index.html       # 配置表单 / 操作按钮（两种状态）
│   └── result.html      # 结果展示：Token 查看器 + 步骤调试时间线
├── data/                # SQLite 数据库文件挂载目录
├── Dockerfile           # 多阶段构建
├── docker-compose.yml
├── nginx-example.conf
└── .gitignore
```

## .gitignore

```
# 编译产物
KyleworksOidcTest

# 数据目录
data/

# 系统文件
.DS_Store
```

## 支持的 OIDC 流程

| 流程 | response_type | PKCE | 说明 |
|------|--------------|------|------|
| Auth Code + PKCE | `code` | ✅ | 最常用，推荐 |
| Auth Code (无 PKCE) | `code` | ❌ | 测试用 |
| Client Credentials | — | — | M2M，grant_type=client_credentials |

## 页面路由

| 路由 | 方法 | 功能 |
|------|------|------|
| `/` | GET | 首页：未配置→配置表单；已配置→操作按钮（登录 + Client Credentials） |
| `/config` | POST | 保存 OIDC 配置到会话 → 302 到 `/` |
| `/discover` | GET | 自动检测 OIDC 端点（`?issuer=...`），返回 JSON |
| `/login` | GET | 发起 OIDC 登录 → 302 到 Keycloak |
| `/callback` | GET | OIDC 回调处理 → 存结果 → 302 到 `/result`（仅支持 GET） |
| `/result` | GET | 结果页：Token 查看器 + 步骤调试时间线 |
| `/logout` | GET | 销毁会话 → 302 到 Keycloak end_session_endpoint |
| `/client-credentials` | GET | Client Credentials 流程 → 存结果 → 302 到 `/result` |

> 导航方式：**多页跳转**，配置页和结果页为独立页面

## OIDC 协议实现细节 (oidc.go)

### Discovery
```
GET {issuer}/.well-known/openid-configuration
→ 提取: authorization_endpoint, token_endpoint, userinfo_endpoint, end_session_endpoint
```

### Auth Code + PKCE 流程
1. 生成 `state` (32字节随机, base64url) — 存入 session 行
2. 生成 `code_verifier` (32字节随机, base64url)
3. 生成 `code_challenge` (SHA256(code_verifier) → base64url)
4. 构造 authorization URL（含 state） → 302 重定向
5. 回调 `/callback?code=xxx&state=xxx`:
   - 从 session 中取出 state，**比对验证**（不匹配则中止）
   - POST token endpoint 交换 code
   - 解析响应: access_token, id_token, refresh_token, expires_in
6. 解码 ID Token: 按 `.` 分割 → base64 解码 header 和 payload → JSON
7. GET userinfo endpoint (Authorization: Bearer {access_token})
8. 所有结果存入 SQLite → 302 到 `/result`

### Auth Code (无 PKCE) 流程
与上述流程相同，但**跳过步骤 2-3**（不生成 code_verifier/code_challenge），authorization URL 不含 `code_challenge` 参数，token 交换时不含 `code_verifier`。

### Client Credentials 流程
1. 生成 session ID → 写 Cookie → 创建 session 行
2. POST `{token_endpoint}`（scope 使用配置表单中的 Scopes 字段）:
   ```
   grant_type=client_credentials&
   client_id={client_id}&
   client_secret={client_secret}&
   scope={scopes}
   ```
3. 解析 access_token
4. 可选用 access_token 调用 userinfo (如果 Keycloak 支持)
5. 结果存入 SQLite → 302 到 `/result`

### 退出
```
{end_session_endpoint}?
  post_logout_redirect_uri={base_url}&
  id_token_hint={id_token}
```
同时销毁本地会话

## 会话结构 (Go struct)

```go
type Session struct {
    ID           string
    OIDCConfig   OIDCConfig
    State        string    // OIDC state 参数 (Auth Code 流程)
    CodeVerifier string    // PKCE code_verifier (Auth Code + PKCE 流程)
    DebugSteps   []DebugStep
    TokenResult  *TokenResult
    CreatedAt    time.Time
}

type OIDCConfig struct {
    Issuer       string
    ClientID     string
    ClientSecret string
    Scopes       string  // "openid profile email"
    Flow         string  // "authcode-pkce" | "authcode"
    BaseURL      string  // 自动检测: X-Forwarded-Proto + Host
}

type DebugStep struct {
    Timestamp    time.Time
    Name         string
    Method       string
    URL          string
    ReqBody      string
    StatusCode   int
    RespBody     string
    Error        string
}

type TokenResult struct {
    AccessToken  string
    TokenType    string
    ExpiresIn    int
    RefreshToken string
    IDToken      string
    IDTokenHeader  map[string]interface{}
    IDTokenClaims  map[string]interface{}
    UserInfo    map[string]interface{}
}
```

## 模板页面 (templates/)

### index.html
一个页面承载两种状态：

**状态 1: 未配置 → 显示配置表单**
- Issuer URL (必填)
- Client ID (必填)
- Client Secret (必填)
- Scopes (默认 "openid profile email")
- Flow (下拉: Auth Code + PKCE / Auth Code。Client Credentials 为独立按钮，不参与此下拉)
- Base URL (自动检测，可手动覆盖。从 `X-Forwarded-Proto` / `X-Forwarded-Host` 头读取，兜底使用 `Host`)

**状态 2: 已配置 → 显示操作按钮**
- 显示当前 Issuer
- "开始登录" 按钮
- "Client Credentials" 按钮
- "修改配置" 链接

### result.html
- Token 查看器：
  - ID Token: 原始 JWT 字符串 + 解码后的 Header 表格 + Payload 表格
  - Access Token: 原始值 + 解码后的 payload（如为 JWT）
  - Refresh Token: 原始值（如有）
- UserInfo 响应 JSON（格式化展示）
- 按时间线展示每个 DebugStep：
  - 每步标题 + 耗时 (ms)
  - HTTP Method + URL + 状态码
  - 错误步骤红色高亮
- 「退出登录」按钮（Auth Code 流程）
- 「返回首页」链接

## Dockerfile（多阶段构建）

```dockerfile
# Stage 1: build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o KyleworksOidcTest .

# Stage 2: run
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/KyleworksOidcTest .
COPY templates/ ./templates/
RUN mkdir -p /app/data
EXPOSE 61000
CMD ["./KyleworksOidcTest"]
```

> 注意：需要 `go.mod` 和 `go.sum` 文件（go.sum 通过 `go mod tidy` 本地生成后提交到 Git）：
> ```
> module KyleworksOidcTest
> go 1.22
>
> require modernc.org/sqlite v1.34.0
> ```
> 生成步骤：`go mod tidy` → 提交 `go.mod` + `go.sum`

## docker-compose.yml

```yaml
services:
  kyleworks-oidc-test:
    image: ghcr.io/alcaprophet/kyleworks-oidc-test:latest
    # 本地开发时注释 image 行，取消注释 build 行:
    # build: .
    container_name: kyleworks-oidc-test
    ports:
      - "61000:61000"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

## SQLite 存储设计 (store.go)

数据库文件：`/app/data/playground.db`

> **目录说明**：`data/` 由 Docker 在 `docker compose up` 时自动创建（通过 volume 挂载）。无需手动创建。host 上的 `./data` 与容器内 `/app/data` 一一对应。

```sql
-- 键值存储（OIDC 配置）
CREATE TABLE IF NOT EXISTS kv (
    k TEXT PRIMARY KEY,
    v TEXT NOT NULL
);

-- 登录会话历史
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    flow TEXT NOT NULL,           -- authcode-pkce | authcode | client-credentials
    config TEXT NOT NULL,         -- JSON: OIDCConfig
    result TEXT,                  -- JSON: TokenResult (nullable, 登录成功后有)
    steps TEXT,                   -- JSON: []DebugStep
    created_at TEXT NOT NULL
);
```

存储策略：
- `config` 键存储当前 OIDC 配置（JSON），每次 /config POST 更新
- `/login` 或 `/client-credentials` 触发时**创建新 session 行 + 写入 Cookie**（CC 流程同样需要 session 存储结果）
- 回调成功后更新 result + steps
- `/result` 页面从 Cookie 读取 `sid` → 查询 sessions 表 → 显示该 session 的记录
- 若无 Cookie 或无对应 session 记录，显示"暂无结果"提示 + 返回首页链接

## 会话与 Cookie

- Cookie 名: `sid`，值为随机生成的 session ID
- `/login` 发起时创建新 session 行，session ID 写入 Cookie
- `/callback` 通过 Cookie 中的 session ID 找到对应会话，更新 result + steps
- Cookie 属性: `Path=/`、`HttpOnly`，无 Secure（测试环境允许 HTTP）

## 启动流程 (main.go)

1. 初始化 SQLite（打开 `/app/data/playground.db`，自动建表）
2. 注册 `html/template` 自定义函数（`json` 格式化、`since` 时间差）
3. 注册所有 HTTP 路由
4. 监听 `:61000`，输出 `[OK] 服务已启动: http://0.0.0.0:61000`

## 待办任务清单

- [x] 创建 `go.mod` 并运行 `go mod tidy` (生成 go.sum)
- [x] 创建 `main.go` — HTTP 路由 + Cookie 会话 + 模板渲染
- [x] 创建 `oidc.go` — OIDC 协议手动实现
- [x] 创建 `store.go` — SQLite 存储层
- [x] 创建 `templates/index.html` — 配置表单 / 操作按钮
- [x] 创建 `templates/result.html` — Token 查看器 + 调试时间线
- [x] 创建 `Dockerfile` — 多阶段构建
- [x] 创建 `docker-compose.yml`
- [x] 创建 `nginx-example.conf` — 外部 NGINX 反代参考配置
- [x] 创建 `.gitignore`
- [x] `docker compose up -d --build` 验证

## 验证清单

1. `docker compose up -d` 容器正常运行
2. 访问 `http://localhost:61000` → 显示配置表单
3. 填写 Keycloak 信息 → 提交 → 显示操作按钮
4. OIDC 登录 → 跳转 Keycloak → 回调 → 显示 Token 结果和调试步骤
5. 退出登录 → Keycloak 退出页 → 回到首页未登录状态
6. Client Credentials 流程 → 显示 Access Token 结果
7. 外部 NGINX + HTTPS 全流程正常
8. 容器重启后配置仍保留（SQLite 持久化）

## 已知设计决策（审阅排除项）

以下条目已经过人工审查并确认保留，**AI 助手审阅时不应直接将其标记为问题** ， **但 AI 助手可以进行思考，确认忽略以下问题是否真实存在有显著性影响的问题，若有请详细解释列出**：

### 1. err 变量遮蔽（variable shadow）

`handleCallback` 中 `endpoints, err := Discover(...)` 使用 `:=` 遮蔽了外层 `sess, err := GetSession(...)` 的 `err`。这是 Go 惯用写法（因为有新变量 `endpoints`），Go 编译器正确处理，**不产生任何 bug**。且后续代码中 `err` 的引用正好就是 `Discover` 返回的 `err`，逻辑正确。

### 2. Client Credentials 流程 CreateSession 失败时丢失调试步骤

如果 `CreateSession`（SQLite 写入）失败，当前直接调用 `renderError` 返回简单错误页，不保留之前的调试步骤。**SQLite 写入失败概率极低**（磁盘满或权限错误），且此时尚未创建 session 行，无法通过 `/result` 页展示。保留当前处理方式。

### 3. State 验证失败时将完整 state 值写入错误信息

测试环境，AGENTS.md 已明确"不做防御性编程"。state 是随机 token，暴露在页面上不构成实际风险。保留当前行为。

### 4. handleLogin / handleClientCredentials 中 Discovery 失败不保留调试步骤

此时尚未创建 session，无法跳转到 `/result` 页面展示调试时间线。`renderError` 返回简单错误描述是可接受的降级方案。

### 5. DebugStep 时间戳使用 `time.Now()` 而非请求开始时间

每个步骤的 `Timestamp` 字段记录的是步骤记录创建时刻（请求完成后），而非 HTTP 请求发起时刻。对于调试目的，时间戳的精确顺序比绝对精度更重要。`DurationMs` 字段记录了实际网络耗时。

### 6. 使用 `http.DefaultClient` 发起 UserInfo 请求

`GetUserInfo` 使用 `http.DefaultClient.Do(req)` 而非创建专用 client。项目明确"不做请求超时"，`DefaultClient`（无超时）符合设计意图。

### 7. Client Secret 明文展示

配置表单中 Client Secret 使用 `<input type="text">` 而非 `type="password"`。项目为测试环境，明文展示方便验证复制是否正确。不设密码遮蔽。

### 8. 配置修改时预填已有值

`/?edit=1` 进入编辑模式时，表单自动填入已保存的 Issuer、Client ID、Client Secret、Scopes、Flow、Base URL。通过 `IsEditing` 标志 + `{{if .Config}}` 模板条件实现。

### 9. 调试时间线汇总行

结果页的调试时间线顶部显示步骤数、错误数、总耗时汇总。通过 `countErrors` 和 `sumDuration` 两个模板函数计算。

### 10. 结果页顶部操作按钮

结果页在顶部和底部均放置「返回首页」和「退出登录」按钮，避免长页面下需滚动到底部操作。

### 11. 已配置状态展示完整信息

已配置首页展示 Issuer、Client ID、Scopes、Base URL、流程，方便用户确认当前配置。


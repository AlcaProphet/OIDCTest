# OIDC Playground

[![Build and Push Docker Image](https://github.com/AlcaProphet/OIDCTest/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/AlcaProphet/OIDCTest/actions/workflows/docker-publish.yml)

轻量级 OIDC 调试工具，用于测试自搭建 Keycloak SSO 的 OIDC 登录流程。类似 Auth0 OIDC Playground，**零配置文件**，全部通过 Web 表单操作。

## 特性

- **零配置** — 全部设置通过 Web 表单完成，无需 `.env` 或配置文件
- **自动发现** — 输入 Issuer URL 一键检测 OIDC 端点，也支持直接粘贴 `.well-known/openid-configuration` 完整地址
- **Keycloak 助手** — 自动生成 Keycloak 客户端所需的 Root URL、Redirect URI、Web Origins 等配置值，点击即复制
- **步骤可视化** — 每次 HTTP 请求均记录方法、URL、状态码、耗时，按时间线展示
- **三种流程** — Authorization Code + PKCE / Authorization Code（无 PKCE）/ Client Credentials
- **Token 查看器** — 自动解码 JWT Header 和 Payload，结构化展示 UserInfo
- **持久化** — SQLite 存储，容器重启配置不丢失
- **轻量部署** — Go 编译为单一二进制，Docker 镜像约 15MB

## 快速开始

### 前提条件

- Docker 和 Docker Compose
- 一个已配置好的 Keycloak 实例（或其他兼容 OIDC 的 Provider）
- （可选）外部 NGINX 用于 HTTPS 反代

### 方式一：源码构建

```bash
git clone <your-repo-url> KyleworksOidcTest
cd KyleworksOidcTest
docker compose up -d --build
```

### 方式二：直接拉取镜像（推荐）

```bash
mkdir KyleworksOidcTest && cd KyleworksOidcTest
# 下载 docker-compose.yml 后
docker compose pull && docker compose up -d
```

访问 `http://<服务器IP>:61000` 即可看到配置页面。

### Keycloak 客户端配置

在 Keycloak 中创建客户端，参考以下值（也可在工具首页的「Keycloak 客户端配置参考」卡片中直接复制）：

| 配置项 | 值 |
|--------|-----|
| Protocol | OpenID Connect |
| Access Type | confidential |
| Standard Flow | ✅ 启用 |
| Valid Redirect URIs | `https://<你的域名>/callback` |
| Post Logout Redirect URIs | `https://<你的域名>/` |

## 使用流程

### 1. 填写配置

首次访问时显示配置表单：

- **Issuer URL** — 输入后点击「检测端点」自动获取 OIDC 端点信息
- **Client ID / Client Secret** — Keycloak 客户端凭据
- **Scopes** — 默认 `openid profile email roles groups`，支持复选框快速选择 + 自定义输入
- **流程** — 推荐 Authorization Code + PKCE
- **Base URL** — 自动检测，可手动覆盖

### 2. 测试登录

保存配置后，点击：

- **「开始登录」** — 发起 Authorization Code 流程，跳转 Keycloak 登录后回调展示完整 Token 信息
- **「Client Credentials」** — 直接获取 Access Token（M2M 场景）

### 3. 查看结果

结果页展示：

- **ID Token** — 原始 JWT + 解码后的 Header / Payload 表格
- **Access Token** — 原始值 + JWT 解码（如适用）
- **Refresh Token** — 原始值（如有）
- **UserInfo** — 格式化 JSON
- **调试时间线** — 每步的方法、URL、状态码、耗时（ms），错误步骤红色高亮

## 支持的 OIDC 流程

| 流程 | 说明 | PKCE |
|------|------|------|
| Authorization Code + PKCE | 授权码流程 + PKCE（推荐） | ✅ |
| Authorization Code（无 PKCE） | 授权码流程，用于对比测试 | ❌ |
| Client Credentials | 机器对机器，直接获取 Access Token | — |

## 路由

| 路由 | 方法 | 功能 |
|------|------|------|
| `/` | GET | 首页：未配置→配置表单；已配置→操作按钮（登录；CC 按钮登录后显示） |
| `/config` | POST | 保存 OIDC 配置 |
| `/discover` | GET | 自动检测 OIDC 端点（`?issuer=...`） |
| `/login` | GET | 发起 OIDC 登录 → 302 Keycloak |
| `/callback` | GET | OIDC 回调处理 |
| `/result` | GET | Token 查看器 + 调试时间线 |
| `/logout` | GET | 退出登录（含 Keycloak 单点登出） |
| `/client-credentials` | GET | Client Credentials 流程 |

## 外部 NGINX 反代

```nginx
server {
    listen 443 ssl;
    server_name oidc-test.example.com;

    location / {
        proxy_pass http://127.0.0.1:61000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 技术栈

| 层 | 选型 | 说明 |
|---|------|------|
| 语言 | Go 1.22+ | 编译为单一静态二进制 |
| HTTP | `net/http` 标准库 | 无第三方 Web 框架 |
| 模板 | `html/template` | 服务端渲染，零 JS 框架 |
| OIDC | **手动实现** | 自行构造 HTTP 请求，无 OIDC 库依赖 |
| 存储 | SQLite | `modernc.org/sqlite`，纯 Go 实现，无 CGO |
| 容器 | 多阶段构建 | 最终镜像约 15MB |

## 项目结构

```
KyleworksOidcTest/
├── main.go              # HTTP 路由 + 会话 + 模板渲染
├── oidc.go              # OIDC 协议手动实现
├── store.go             # SQLite 存储层
├── go.mod / go.sum      # Go 模块定义
├── templates/
│   ├── index.html       # 配置表单 / 操作按钮 + Keycloak 助手
│   └── result.html      # Token 查看器 + 调试时间线
├── Dockerfile           # 多阶段构建
├── docker-compose.yml   # Docker Compose 部署
├── .github/workflows/   # GitHub Actions 自动构建镜像（docker-publish）+ 创建 Release 归档（release）
└── nginx-example.conf   # NGINX 反代参考配置
```

## 注意事项

- 本项目为**测试调试工具**，面向非生产环境（1-5 人内部使用）
- Cookie 使用 `HttpOnly`，未设置 `Secure` 标记（支持 HTTP 环境测试）
- 不做 CSRF / Rate Limiting / Security Headers 等生产级安全防护
- JWT 仅做 base64 解码展示 Claims，**不验证签名**
- 不做并发锁 / 请求重试 / 优雅关闭

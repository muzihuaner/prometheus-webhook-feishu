# Prometheus Webhook for Feishu (Go 版)

一个用 **Go** 编写的高性能 Webhook 服务，接收 [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) 的告警通知，并将其格式化为**飞书（Lark）交互式卡片**推送到群里。同时提供带登录鉴权的 Web 管理界面，用于在线配置 Webhook URL、自定义卡片模板、并发送测试告警。


## 功能特性

- **Webhook 接收**：接收 Alertmanager 的 Webhook 通知（`/webhook`，`POST`）。
- **飞书卡片**：将告警格式化为美观的交互式卡片，支持 `firing` / `resolved` 两种状态配色。
- **Web 管理界面**：带登录鉴权的后台，可：
  - 在线编辑并保存飞书卡片模板（JSON）。
  - 配置飞书机器人 Webhook URL、告警/恢复标题。
  - 一键发送测试告警验证配置。
  - **告警历史**：完整记录每次接收到的告警及飞书推送结果（成功 / 失败 / 异常），支持按状态与关键字筛选、查看明细、一键清空。
- **环境变量覆盖**：支持用环境变量覆盖关键配置，方便容器 / K8s 部署，无需挂载配置文件。
- **单次构建、多平台运行**：纯标准库实现，`go:embed` 内嵌模板，编译为单个静态二进制文件。
- **健康检查**：提供 `/healthz` 端点，便于容器探针与负载均衡。
- **Docker 支持**：提供多阶段 `Dockerfile` 与 `docker-compose.yml`，镜像体积小（基于 alpine）。

## 目录结构

```
go/
├── cmd/
│   └── server/
│       └── main.go        # 入口：加载配置、初始化存储、组装路由、启动
├── internal/
│   ├── config/            # 配置加载与运行时保存（含环境变量覆盖）
│   ├── feishu/            # 卡片构建与飞书发送
│   ├── auth/              # 会话签名 Cookie 鉴权 + 一次性消息
│   ├── store/             # 告警历史存储（内存 + JSON 文件持久化）
│   └── web/               # HTTP 处理器、路由与内嵌模板
│       ├── handlers.go
│       ├── router.go
│       └── templates/     # 内嵌的 HTML 模板
│           ├── base.html
│           ├── index.html
│           ├── login.html
│           ├── admin.html
│           └── alerts.html
├── Dockerfile            # 多阶段构建镜像
├── docker-compose.yml    # 编排文件
├── go.mod
├── config.example.json   # 配置示例
├── config.json           # 实际配置（需自行创建）
├── alerts.json           # 告警历史持久化文件（自动生成）
└── README.md
```

## 快速开始

### 1. 准备配置

复制示例配置并重命名为 `config.json`：

```bash
cp config.example.json config.json
```

编辑 `config.json`，至少填写：

| 字段 | 说明 |
| --- | --- |
| `USERNAME` / `PASSWORD` | 管理后台登录凭据 |
| `FEISHU_WEBHOOK_URL` | 飞书机器人 Webhook 地址 |
| `FIRING_TITLE` / `RESOLVED_TITLE` | 告警 / 恢复时的卡片标题 |
| `FEISHU_CARD_TEMPLATE` | 飞书卡片模板（支持 `{alertname}`、`{severity}`、`{instance}`、`{description}`、`{start_time}`、`{card_color}`、`{header_title}` 占位符） |

### 2. 本地运行（需安装 Go 1.21+）

```bash
cd go
go run ./cmd/server
# 或编译后运行
go build -o app ./cmd/server
./app
```

### 3. 环境变量（可选，优先级高于 config.json）

| 环境变量 | 覆盖字段 |
| --- | --- |
| `FEISHU_USERNAME` | `USERNAME` |
| `FEISHU_PASSWORD` | `PASSWORD` |
| `FEISHU_WEBHOOK_URL` | `FEISHU_WEBHOOK_URL` |
| `FEISHU_FIRING_TITLE` | `FIRING_TITLE` |
| `FEISHU_RESOLVED_TITLE` | `RESOLVED_TITLE` |
| `CONFIG_FILE` | 配置文件路径（默认 `config.json`） |
| `ALERTS_FILE` | 告警历史持久化文件路径（默认 `alerts.json`，设为空则不持久化） |
| `PORT` | 监听端口（默认 `5000`） |
| `SESSION_SECRET` | 会话 Cookie 签名密钥（不设置则每次启动随机） |

示例：

```bash
FEISHU_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx" \
SESSION_SECRET="change-me" \
PORT=5000 \
./app
```

## Docker 部署

### 方式一：docker compose（推荐）

```bash
cd go
cp config.example.json config.json   # 先创建配置文件
docker compose up -d --build
```

服务将在 `http://<服务器IP>:5000` 提供访问。

> 也可不挂载 `config.json`，仅用 `docker-compose.yml` 中的环境变量覆盖配置。

### 方式二：手动构建运行

```bash
cd go
docker build -t prometheus-webhook-feishu .
docker run -d -p 5000:5000 \
  -v $(pwd)/config.json:/app/config.json \
  --name prometheus-webhook-feishu \
  prometheus-webhook-feishu
```

## 配置 Alertmanager

在 `alertmanager.yml` 中添加指向本服务的接收器：

```yaml
global:
  resolve_timeout: 1m
route:
  receiver: feishu-webhook
  group_by:
    - alertname
  group_wait: 5s
  group_interval: 1m
  repeat_interval: 30m
receivers:
  - name: feishu-webhook
    webhook_configs:
      - url: http://<服务器IP>:5000/webhook
        send_resolved: true
```

## 管理界面

浏览器打开 `http://<服务器IP>:5000`，点击「前往管理后台」进入登录页，使用 `config.json`（或环境变量）中配置的 `USERNAME` / `PASSWORD` 登录：

- **配置管理**（`/admin`）：在线修改飞书 Webhook URL、告警/恢复标题、卡片模板，并可一键发送测试告警。
- **告警历史**（`/alerts`）：查看每次接收到的告警与飞书推送结果（成功 / 失败 / 异常），支持按推送状态、告警类型筛选与关键字搜索，点击「查看」可展开单条告警的明细（含每条子告警的等级、实例与摘要），亦可一键清空历史。

> 告警历史默认持久化到 `alerts.json`，进程重启后依然保留（最多保留 500 条，可在 `store.New` 调整）。

## 健康检查

```bash
curl http://<服务器IP>:5000/healthz
# 返回 ok
```


## 许可证

MIT

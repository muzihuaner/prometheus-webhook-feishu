# Prometheus Webhook for Feishu

一个用 **Go** 编写的高性能 Webhook 服务，接收 [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) 的告警通知，并将其格式化为**飞书（Lark）交互式卡片**推送到群里。同时提供带登录鉴权的 Web 管理界面，用于在线配置 Webhook URL、自定义卡片模板、并发送测试告警。


## 功能特性

- **Webhook 接收**：接收 Alertmanager 的 Webhook 通知（`/webhook`，`POST`）。
- **飞书卡片**：将告警格式化为美观的交互式卡片，支持 `firing` / `resolved` 两种状态配色。
- **Web 管理界面**：带登录鉴权的后台，可：
  - 在线编辑并保存飞书卡片模板（JSON）。
  - 配置飞书机器人 Webhook URL、告警/恢复标题。
  - 一键发送测试告警验证配置。
  - **告警历史**：完整记录每次接收到的告警及飞书推送结果（成功 / 失败 / 异常），支持按状态与关键字筛选、查看明细、一键清空。
- **环境变量覆盖**：支持用环境变量覆盖关键配置，方便容器部署，无需挂载配置文件。
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
├── helm/prometheus-webhook-feishu  # Helm chart（K8s 部署）
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
| `FEISHU_CARD_TEMPLATE` | 飞书卡片模板（支持 `{alertname}`、`{severity}`、`{instance}`、`{description}`、`{start_time}`、`{card_color}`、`{header_title}` 占位符）；支持 `{占位符}` 与 Go template 双语法，详见下文 |

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
git clone https://github.com/muzihuaner/prometheus-webhook-feishu.git
cd prometheus-webhook-feishu

cp config.example.json config.json   # 先创建配置文件
docker compose up -d
```

服务将在 `http://<服务器IP>:5000` 提供访问。

> 也可不挂载 `config.json`，仅用 `docker-compose.yml` 中的环境变量覆盖配置。

### 方式二：手动构建运行

```bash
git clone https://github.com/muzihuaner/prometheus-webhook-feishu.git
cd prometheus-webhook-feishu


docker build -t prometheus-webhook-feishu .
docker run -d -p 5000:5000 \
  -v $(pwd)/config.json:/app/config.json \
  -v $(pwd)/alerts.json:/app/alerts.json \
  --name prometheus-webhook-feishu \
  prometheus-webhook-feishu
```

## Helm 部署（Kubernetes）

```bash
helm install prometheus-webhook-feishu helm/prometheus-webhook-feishu \
  --set config.FEISHU_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx" \
  --set config.PASSWORD="your-password"
```

说明：

- 配置以 values 中的 `config` 为唯一来源，渲染为 ConfigMap 并经 initContainer 拷贝到可写区；管理后台在线保存的修改仅在本 Pod 生命周期内生效，重启后恢复为 chart 配置。
- `helm upgrade` 修改 `config` 会触发 Pod 滚动重启以应用新配置。
- 告警历史（alerts.json）保存在 emptyDir 中，Pod 重建后清空。
- 建议设置 `sessionSecret` 固定值，避免 Pod 重启后登录会话失效。
- 开启 Ingress：`--set ingress.enabled=true --set 'ingress.hosts[0].host=feishu.example.com'`。
- 查看全部可配置项：`helm show values helm/prometheus-webhook-feishu`。
- 修改配置建议使用 values 文件（嵌套的 `FEISHU_CARD_TEMPLATE` 用 `--set` 不便），升级：`helm upgrade prometheus-webhook-feishu helm/prometheus-webhook-feishu -f my-values.yaml`；卸载：`helm uninstall prometheus-webhook-feishu`。

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

## 卡片模板：Go template 语法

`FEISHU_CARD_TEMPLATE` 中的**字符串值**除了支持传统的 `{占位符}` 替换，还支持 Go template（`text/template`）语法：

- 字符串中**含 `{{`**：先按 Go 模板渲染，渲染结果再执行 `{占位符}` 替换 —— 同一字符串里两种语法可以混用；
- 字符串中**不含 `{{`**：行为与旧版完全一致，纯 `{占位符}` 替换，**完全向后兼容**。

### 示例

贴近真实场景：按 `cluster` / `namespace` 条件映射「告警项目」名称（类似从钉钉机器人迁移过来的写法），并在卡片中统计 firing 数量、遍历当前告警的标签。注意 JSON 中模板写在**单行字符串**内，换行用 `\n` 转义，引号需转义为 `\"`：

```json
{
  "header": {
    "title": { "tag": "plain_text", "content": "{header_title}（当前 firing：{{ .Alerts.Firing | len }} 条）" }
  },
  "elements": [
    { "tag": "div", "text": { "tag": "lark_md", "content": "**告警项目**：{{ if and (eq .Labels.cluster \"prod-cluster\") (eq .Labels.namespace \"prod\") }}项目A-生产{{ else if eq .Labels.cluster \"prod-cluster\" }}项目A-其他环境{{ else }}默认项目{{ end }}" } },
    { "tag": "div", "text": { "tag": "lark_md", "content": "**标签**：{{ range .Labels.SortedPairs }}{{ .Name }}={{ .Value }} {{ end }}" } },
    { "tag": "div", "text": { "tag": "lark_md", "content": "**开始时间**：{{ .StartsAt | date \"2006-01-02 15:04:05\" }}" } }
  ]
}
```

### 上下文字段

模板中的 `.` 是「当前这条告警」，并附带通知级信息：

告警级字段（卡片 elements 会按告警逐条复制，每条告警各自渲染一次）：

| 字段 | 说明 |
| --- | --- |
| `.Status` | 当前告警状态：`firing` / `resolved` |
| `.Labels` | 告警标签（KV），支持 `.SortedPairs`（按名称排序的 `{Name, Value}` 列表）、`.Names`、`.Values`、`.Remove` |
| `.Annotations` | 告警注解（KV），方法同 `.Labels` |
| `.StartsAt` | 告警开始时间（RFC3339 字符串），常配合 `date` 函数格式化 |
| `.EndsAt` | 告警结束时间 |
| `.GeneratorURL` | 告警规则来源 URL |

通知级字段（整组告警的信息）：

| 字段 | 说明 |
| --- | --- |
| `.Alerts` | 本次通知包含的全部告警，`.Alerts.Firing` / `.Alerts.Resolved` 为按状态过滤的子集 |
| `.GroupLabels` | 分组标签（KV），方法同 `.Labels` |
| `.CommonLabels` | 所有告警共有的标签（KV），方法同 `.Labels` |
| `.ExternalURL` | Alertmanager 外部访问地址 |
| `.Receiver` | 接收器名称 |

### 函数

| 函数 | 说明 | 示例 |
| --- | --- | --- |
| `toUpper` / `toLower` | 大小写转换 | `{{ .Labels.severity | toUpper }}` |
| `join` | 用分隔符拼接字符串列表 | `{{ .Labels.Names | join "," }}` |
| `html` | HTML 转义 | `{{ .Annotations.description | html }}` |
| `markdown` | 转义飞书 lark_md 特殊字符 | `{{ .Annotations.summary | markdown }}` |
| `date` | 按布局格式化时间（转换为东八区 UTC+8） | `{{ .StartsAt | date "2006-01-02 15:04:05" }}` |

`eq`、`ne`、`lt`、`gt`、`and`、`or`、`not`、`range`、`len`、`index`、`if/else` 等为 Go 模板内置，直接使用即可。

### 书写约定

1. **单行书写**：JSON 字符串值内的模板必须写成单行，换行用 `\n` 转义（管理后台的模板编辑框同理）。
2. **面向当前告警**：模板默认作用于「当前这条告警」，卡片的 elements 会按告警逐条复制。`{{ range .Alerts }}` 仅用于**统计类**场景（如标题中 `{{ .Alerts.Firing | len }}`）；在 elements 里遍历全部告警会导致卡片内容 N×M 重复，请勿这样使用。

### 容错

模板写错（解析或渲染失败）时，该次推送会记为 **error** 历史（可在「告警历史」中查看原因），**不会**发出残缺卡片；修正模板后重试即可。

## 健康检查

```bash
curl http://<服务器IP>:5000/healthz
# 返回 ok
```


## 许可证

MIT

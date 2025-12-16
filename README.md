# Prometheus Webhook for Feishu (飞书)

该项目提供了一个 Webhook 服务，用于接收来自 Prometheus Alertmanager 的告警通知，并将其作为交互式卡片转发到飞书。它还提供了一个 Web 管理界面，用于自定义消息模板和配置 Webhook URL。

## 功能特性

- **Webhook 接收**: 接收 Prometheus Alertmanager 的 Webhook 通知。
- **飞书卡片**: 将告警格式化为美观的飞书交互式卡片。
- **Web 管理界面**: 提供一个带登录验证的管理页面，用于：
  - 自定义飞书卡片模板。
  - 配置飞书 Webhook URL。
  - 发送测试告警以验证配置。
- **灵活配置**: 所有配置（包括凭据）都通过 `config.json` 文件管理。
- **Docker 支持**: 提供 Dockerfile，方便容器化部署。

## 环境要求

- Python 3.6+
- Pip

## 安装与配置

1.  **克隆代码仓库：**

    ```bash
    git clone https://github.com/muzihuaner/prometheus-webhook-feishu.git
    cd prometheus-webhook-feishu
    ```

2.  **安装依赖：**

    ```bash
    pip install -r requirements.txt
    ```

3.  **创建配置文件：**

    复制 `config.example.json` 并重命名为 `config.json`。

    ```bash
    cp config.example.json config.json
    ```

4.  **编辑 `config.json`：**

    - `USERNAME`: 设置管理页面的登录用户名。
    - `PASSWORD`: 设置管理页面的登录密码。
    - `FEISHU_WEBHOOK_URL`: 你的飞书机器人 Webhook URL。
    - `FEISHU_CARD_TEMPLATE`: 自定义飞书卡片的消息模板。

## 使用方法

1.  **运行 Webhook 服务：**

    ```bash
    python fs.py
    ```

    服务将在 `0.0.0.0:5000` 上启动。

2.  **访问管理界面：**

    在浏览器中打开 `http://<你的服务器IP>:5000`，你将看到应用的首页。点击链接进入登录页面，使用 `config.json` 中配置的用户名和密码登录。

3.  **配置 Prometheus Alertmanager：**

    在你的 Alertmanager 配置文件 (`alertmanager.yml`) 中，添加一个指向该服务的 Webhook 接收器。

    ```yaml
    receivers:
      - name: 'feishu-webhook'
        webhook_configs:
          - url: 'http://<你的服务器IP>:5000/webhook'
    ```

## Docker 部署

1.  **构建 Docker 镜像：**

    ```bash
    docker build -t prometheus-webhook-feishu .
    ```

2.  **运行 Docker 容器：**

    在运行容器时，你需要将 `config.json` 文件挂载到容器中。

    ```bash
    docker run -d -p 5000:5000 \
      -v $(pwd)/prometheus-webhook-feishu/config.json:/app/config.json \
      --name prometheus-webhook-feishu \
      prometheus-webhook-feishu
    ```

    服务将在你的 Docker 主机的 `5000` 端口上访问。

## 许可证

该项目根据 MIT 许可证授权。
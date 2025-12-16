from flask import Flask, request, jsonify, render_template, redirect, url_for, flash
from flask_login import LoginManager, UserMixin, login_user, login_required, logout_user
import requests
import json
import datetime
import os
import logging

# --- Configuration Loading ---

def load_config():
    try:
        with open('config.json', 'r', encoding='utf-8') as f:
            return json.load(f)
    except FileNotFoundError:
        logging.error("错误：未找到 config.json 文件。请根据 config.example.json 创建一个。")
        exit(1)
    except json.JSONDecodeError:
        logging.error("错误：config.json 文件格式不正确。")
        exit(1)

config = load_config()

# --- Flask App Initialization ---

app = Flask(__name__)
app.secret_key = os.urandom(24)

# --- Logging Configuration ---

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

# --- User Authentication ---

login_manager = LoginManager()
login_manager.init_app(app)
login_manager.login_view = 'login'
login_manager.login_message = "请登录以访问此页面。"

class User(UserMixin):
    def __init__(self, id):
        self.id = id

@login_manager.user_loader
def load_user(user_id):
    return User(user_id)

# --- Feishu Notification Logic ---

def send_feishu_card(alerts, status):
    webhook_url = config.get('FEISHU_WEBHOOK_URL')
    template = config.get('FEISHU_CARD_TEMPLATE')

    if not webhook_url or 'your-webhook-id' in webhook_url:
        logging.error("错误：飞书 Webhook URL 未在 config.json 中正确配置。")
        flash("发送测试通知失败: 飞书 Webhook URL 未正确配置。", 'error')
        return

    card_color = "red" if status == "firing" else "green"
    firing_title = config.get('FIRING_TITLE', '🚨 告警已触发')
    resolved_title = config.get('RESOLVED_TITLE', '✅ 告警已恢复')
    header_title = firing_title if status == "firing" else resolved_title

    all_elements = []
    for alert in alerts:
        labels = alert.get('labels', {})
        annotations = alert.get('annotations', {})
        start_time_str = alert.get('startsAt', 'N/A')

        try:
            start_time_dt = datetime.datetime.fromisoformat(start_time_str.replace('Z', '+00:00'))
            local_tz = datetime.timezone(datetime.timedelta(hours=8))
            start_time_local = start_time_dt.astimezone(local_tz)
            start_time = start_time_local.strftime('%Y-%m-%d %H:%M:%S')
        except (ValueError, TypeError):
            start_time = start_time_str

        # Deep copy the template elements to avoid modifying the original config
        alert_elements = json.loads(json.dumps(template['card']['elements']))

        for element in alert_elements:
            # Recursively replace placeholders in the element
            def replace_placeholders(obj):
                if isinstance(obj, dict):
                    return {k: replace_placeholders(v) for k, v in obj.items()}
                elif isinstance(obj, list):
                    return [replace_placeholders(i) for i in obj]
                elif isinstance(obj, str):
                    return obj.format(
                        alertname=labels.get('alertname', '未知'),
                        severity=labels.get('severity', 'info'),
                        instance=labels.get('instance', 'N/A'),
                        description=annotations.get('description', '无描述'),
                        start_time=start_time,
                        card_color=card_color,
                        header_title=header_title
                    )
                return obj

            all_elements.append(replace_placeholders(element))

    payload = {
        "msg_type": "interactive",
        "card": {
            "config": template['card']['config'],
            "header": {
                "template": card_color,
                "title": {
                    "content": header_title,
                    "tag": "plain_text"
                }
            },
            "elements": all_elements
        }
    }

    try:
        response = requests.post(webhook_url, json=payload, headers={'Content-Type': 'application/json'})
        response.raise_for_status()
        logging.info(f"成功发送飞书通知，状态码：{response.status_code}")
        flash("测试通知已成功发送！", 'success')
    except requests.exceptions.RequestException as e:
        logging.error(f"发送飞书通知失败：{e}")
        flash(f"发送飞书通知失败: {e}", 'error')

# --- Web Routes ---

@app.route('/webhook', methods=['POST'])
def prometheus_webhook():
    try:
        data = request.json
        alerts = data.get('alerts', [])
        status = data.get('status', 'firing')

        if alerts:
            send_feishu_card(alerts, status)

        return jsonify({"status": "success"}), 200
    except Exception as e:
        logging.error(f"处理 webhook 请求失败: {e}")
        return jsonify({"status": "error", "message": str(e)}), 500

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/login', methods=['GET', 'POST'])
def login():
    if request.method == 'POST':
        username = request.form['username']
        password = request.form['password']
        if username == config.get('USERNAME') and password == config.get('PASSWORD'):
            user = User(id=1)
            login_user(user)
            return redirect(url_for('admin'))
        flash("无效的凭据")
    return render_template('login.html')

@app.route('/logout')
@login_required
def logout():
    logout_user()
    return redirect(url_for('login'))

@app.route('/admin')
@login_required
def admin():
    return render_template(
        'admin.html',
        webhook_url=config.get('FEISHU_WEBHOOK_URL'),
        firing_title=config.get('FIRING_TITLE', '🚨 生产环境告警触发'),
        resolved_title=config.get('RESOLVED_TITLE', '✅ 告警已恢复'),
        template_content=json.dumps(config.get('FEISHU_CARD_TEMPLATE'), indent=4, ensure_ascii=False)
    )

@app.route('/save', methods=['POST'])
@login_required
def save():
    try:
        config['FEISHU_WEBHOOK_URL'] = request.form['webhook_url']
        config['FIRING_TITLE'] = request.form['firing_title']
        config['RESOLVED_TITLE'] = request.form['resolved_title']
        config['FEISHU_CARD_TEMPLATE'] = json.loads(request.form['template'])

        with open('config.json', 'w', encoding='utf-8') as f:
            json.dump(config, f, indent=4, ensure_ascii=False)

        flash("配置已成功保存！")
    except json.JSONDecodeError:
        flash("模板的JSON格式无效。")
    except Exception as e:
        flash(f"保存配置时出错: {e}")

    return redirect(url_for('admin'))

@app.route('/test', methods=['POST'])
@login_required
def test_notification():
    try:
        test_alerts = [
            {
                "labels": {
                    "alertname": "测试告警",
                    "severity": "critical",
                    "instance": "localhost:9090"
                },
                "annotations": {
                    "description": "这是一个来自管理页面的测试告警。"
                },
                "startsAt": datetime.datetime.utcnow().isoformat() + "Z"
            }
        ]
        send_feishu_card(test_alerts, "firing")
        flash("测试通知已发送！")
    except Exception as e:
        flash(f"发送测试通知失败: {e}")

    return redirect(url_for('admin'))

# --- Main Execution ---

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
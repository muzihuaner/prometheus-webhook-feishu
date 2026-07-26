package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/muzihuaner/prometheus-webhook-feishu/internal/config"
)

// Alert / WebhookPayload 对应 Prometheus Alertmanager 的 webhook 数据结构。
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

type WebhookPayload struct {
	Status       string            `json:"status"`
	Alerts       []Alert           `json:"alerts"`
	CommonLabels map[string]string `json:"commonLabels"`
	ExternalURL  string            `json:"externalURL"`
}

// placeholderRe 匹配 {key} 形式的占位符（单词字符）。
var placeholderRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// fillTemplate 递归遍历模板结构，将其中的 {key} 占位符替换为 values 中的值。
func fillTemplate(tmpl interface{}, values map[string]string) interface{} {
	switch v := tmpl.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, val := range v {
			out[k] = fillTemplate(val, values)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, val := range v {
			out[i] = fillTemplate(val, values)
		}
		return out
	case string:
		return replacePlaceholders(v, values)
	default:
		return tmpl
	}
}

func replacePlaceholders(s string, values map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		key := m[1 : len(m)-1]
		if val, ok := values[key]; ok {
			return val
		}
		return m
	})
}

// formatTime 将 ISO8601 时间转换为 UTC+8（东八区）可读字符串。
func formatTime(s string) string {
	if s == "" {
		return "N/A"
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// 兼容无时区后缀的格式
		if t2, err2 := time.Parse("2006-01-02T15:04:05", s); err2 == nil {
			t = t2
		} else {
			return s
		}
	}
	cst := time.FixedZone("CST", 8*3600)
	return t.In(cst).Format("2006-01-02 15:04:05")
}

// BuildCard 依据告警列表与状态构建飞书交互式卡片 payload。
func BuildCard(alerts []Alert, status string, cfg *config.ConfigState) (map[string]interface{}, error) {
	raw := cfg.Template()
	tmpl, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("卡片模板格式不正确")
	}
	card, ok := tmpl["card"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("卡片模板缺少 card 字段")
	}

	cardColor := "red"
	if status == "resolved" {
		cardColor = "green"
	}
	firingTitle := cfg.Get("FIRING_TITLE")
	if firingTitle == "" {
		firingTitle = "🚨 告警已触发"
	}
	resolvedTitle := cfg.Get("RESOLVED_TITLE")
	if resolvedTitle == "" {
		resolvedTitle = "✅ 告警已恢复"
	}
	headerTitle := firingTitle
	if status == "resolved" {
		headerTitle = resolvedTitle
	}

	elementsSrc, _ := card["elements"].([]interface{})
	allElements := make([]interface{}, 0, len(elementsSrc)*len(alerts))
	for _, alert := range alerts {
		labels := alert.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		annotations := alert.Annotations
		if annotations == nil {
			annotations = map[string]string{}
		}
		values := map[string]string{
			"alertname":    labels["alertname"],
			"severity":     labels["severity"],
			"instance":     labels["instance"],
			"description":  annotations["description"],
			"start_time":   formatTime(alert.StartsAt),
			"card_color":   cardColor,
			"header_title": headerTitle,
		}
		for _, el := range elementsSrc {
			allElements = append(allElements, fillTemplate(el, values))
		}
	}

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": card["config"],
			"header": map[string]interface{}{
				"template": cardColor,
				"title": map[string]interface{}{
					"content": headerTitle,
					"tag":     "plain_text",
				},
			},
			"elements": allElements,
		},
	}, nil
}

// SendFeishu 将卡片 payload 发送到飞书机器人 Webhook。
func SendFeishu(payload map[string]interface{}, cfg *config.ConfigState) error {
	webhookURL := cfg.Get("FEISHU_WEBHOOK_URL")
	if webhookURL == "" || strings.Contains(webhookURL, "your-webhook-id") {
		return fmt.Errorf("飞书 Webhook URL 未正确配置")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("飞书返回非预期状态码: %d", resp.StatusCode)
	}
	log.Printf("成功发送飞书通知，状态码：%d", resp.StatusCode)
	return nil
}

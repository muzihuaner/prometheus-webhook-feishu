package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/muzihuaner/prometheus-webhook-feishu/internal/auth"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/config"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/feishu"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/store"
)

// Handlers 持有共享的依赖（配置、告警历史存储与模板）。
type Handlers struct {
	cfg   *config.ConfigState
	store *store.Store
	tmpl  *templateStore
}

func (h *Handlers) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("渲染模板 %s 失败: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h *Handlers) writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- 公开页面 ----

func (h *Handlers) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, "index.html", nil)
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		msg, kind := auth.ConsumeFlash(r, w)
		h.render(w, "login.html", map[string]interface{}{"FlashMessage": msg, "FlashKind": kind})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == h.cfg.Get("USERNAME") && password == h.cfg.Get("PASSWORD") {
		auth.SetSessionCookie(w)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	auth.SetFlash(w, "无效的凭据", "error")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ---- 管理后台 ----

func (h *Handlers) admin(w http.ResponseWriter, r *http.Request) {
	msg, kind := auth.ConsumeFlash(r, w)
	tmplJSON, err := json.MarshalIndent(h.cfg.Template(), "", "    ")
	if err != nil {
		tmplJSON = []byte("{}")
	}
	h.render(w, "admin.html", map[string]interface{}{
		"Active":          "config",
		"FlashMessage":    msg,
		"FlashKind":       kind,
		"WebhookURL":      h.cfg.Get("FEISHU_WEBHOOK_URL"),
		"FiringTitle":     h.cfg.Get("FIRING_TITLE"),
		"ResolvedTitle":   h.cfg.Get("RESOLVED_TITLE"),
		"TemplateContent": string(tmplJSON),
	})
}

func (h *Handlers) save(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.cfg.Set("FEISHU_WEBHOOK_URL", strings.TrimSpace(r.FormValue("webhook_url")))
	h.cfg.Set("FIRING_TITLE", strings.TrimSpace(r.FormValue("firing_title")))
	h.cfg.Set("RESOLVED_TITLE", strings.TrimSpace(r.FormValue("resolved_title")))

	var tpl interface{}
	if err := json.Unmarshal([]byte(r.FormValue("template")), &tpl); err != nil {
		auth.SetFlash(w, "模板的 JSON 格式无效: "+err.Error(), "error")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	h.cfg.SetTemplate(tpl)
	if err := h.cfg.Save(); err != nil {
		auth.SetFlash(w, "保存配置失败: "+err.Error(), "error")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	auth.SetFlash(w, "配置已成功保存！", "success")
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handlers) test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	testAlerts := []feishu.Alert{
		{
			Labels: map[string]string{
				"alertname": "测试告警",
				"severity":  "critical",
				"instance":  "localhost:9090",
			},
			Annotations: map[string]string{
				"description": "这是一个来自管理页面的测试告警。",
			},
			StartsAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
	payload := feishu.WebhookPayload{Status: "firing", Alerts: testAlerts}
	rec := h.recordAndSend(payload, "测试告警")
	if rec.PushStatus == store.PushSuccess {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "测试通知已成功发送"})
	} else {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "error", "message": "测试通知发送失败: " + rec.Detail})
	}
}

// ---- 告警历史 ----

func (h *Handlers) alerts(w http.ResponseWriter, r *http.Request) {
	msg, kind := auth.ConsumeFlash(r, w)
	success, failed, errCount := h.store.Counts()
	h.render(w, "alerts.html", map[string]interface{}{
		"Active":        "alerts",
		"FlashMessage":  msg,
		"FlashKind":     kind,
		"Total":         success + failed + errCount,
		"SuccessCount":  success,
		"FailedCount":   failed,
		"ErrorCount":    errCount,
	})
}

func (h *Handlers) apiAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": h.store.List(),
	})
}

func (h *Handlers) clearAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.store.Clear(); err != nil {
		auth.SetFlash(w, "清空失败: "+err.Error(), "error")
	} else {
		auth.SetFlash(w, "告警历史已清空。", "success")
	}
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

// ---- Webhook ----

// recordAndSend 构建卡片、推送到飞书，并记录一条历史（无论成败），返回该记录。
func (h *Handlers) recordAndSend(payload feishu.WebhookPayload, defaultName string) *store.Record {
	status := payload.Status
	if status == "" {
		status = "firing"
	}

	summaries := make([]store.AlertSummary, 0, len(payload.Alerts))
	for _, a := range payload.Alerts {
		labels := a.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		ann := a.Annotations
		if ann == nil {
			ann = map[string]string{}
		}
		summaries = append(summaries, store.AlertSummary{
			AlertName: labels["alertname"],
			Severity:  labels["severity"],
			Instance:  labels["instance"],
			Summary:   ann["summary"],
		})
	}

	rec := store.Record{
		Status: status,
		Count:  len(payload.Alerts),
		Alerts: summaries,
	}
	if len(summaries) > 0 {
		rec.AlertName = summaries[0].AlertName
		rec.Severity = summaries[0].Severity
	} else {
		rec.AlertName = defaultName
	}

	card, err := feishu.BuildCard(payload.Alerts, status, h.cfg)
	if err != nil {
		rec.PushStatus = store.PushError
		rec.Detail = "构建卡片失败: " + err.Error()
		h.store.Add(rec)
		return &rec
	}
	if err := feishu.SendFeishu(card, h.cfg); err != nil {
		rec.PushStatus = store.PushFailed
		rec.Detail = err.Error()
		h.store.Add(rec)
		return &rec
	}
	rec.PushStatus = store.PushSuccess
	rec.Detail = "已成功推送至飞书"
	h.store.Add(rec)
	return &rec
}

func (h *Handlers) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload feishu.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("解析 webhook 请求失败: %v", err)
		h.recordParseError(err)
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "invalid json"})
		return
	}

	if len(payload.Alerts) == 0 {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "no alerts"})
		return
	}

	rec := h.recordAndSend(payload, "未知告警")
	if rec.PushStatus == store.PushSuccess {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
		return
	}
	h.writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": rec.Detail})
}

// recordParseError 当请求体无法解析时，也记录一条 error 状态的历史。
func (h *Handlers) recordParseError(err error) {
	h.store.Add(store.Record{
		Status:     "firing",
		Count:      0,
		AlertName:  "请求解析失败",
		PushStatus: store.PushError,
		Detail:     err.Error(),
	})
}

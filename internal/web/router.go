package web

import (
	"log"
	"net/http"

	"github.com/muzihuaner/prometheus-webhook-feishu/internal/auth"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/config"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/store"
)

// NewRouter 构建 HTTP 路由并返回 *http.ServeMux。
func NewRouter(cfg *config.ConfigState, alertStore *store.Store) *http.ServeMux {
	ts, err := newTemplateStore()
	if err != nil {
		log.Fatalf("解析模板失败: %v", err)
	}
	h := &Handlers{cfg: cfg, store: alertStore, tmpl: ts}

	mux := http.NewServeMux()
	mux.HandleFunc("/", h.index)
	mux.HandleFunc("/login", h.login)
	mux.HandleFunc("/logout", h.logout)
	mux.HandleFunc("/admin", auth.RequireAuth(h.admin))
	mux.HandleFunc("/save", auth.RequireAuth(h.save))
	mux.HandleFunc("/test", auth.RequireAuth(h.test))
	mux.HandleFunc("/alerts", auth.RequireAuth(h.alerts))
	mux.HandleFunc("/api/alerts", auth.RequireAuth(h.apiAlerts))
	mux.HandleFunc("/api/alerts/clear", auth.RequireAuth(h.clearAlerts))
	mux.HandleFunc("/webhook", h.webhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

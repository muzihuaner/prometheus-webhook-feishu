package main

import (
	"log"
	"net/http"
	"os"

	"github.com/muzihuaner/prometheus-webhook-feishu/internal/auth"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/config"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/store"
	"github.com/muzihuaner/prometheus-webhook-feishu/internal/web"
)

func main() {
	auth.InitSecret()

	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.json"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件 %s 失败: %v", configPath, err)
	}

	alertsPath := os.Getenv("ALERTS_FILE")
	if alertsPath == "" {
		alertsPath = "alerts.json"
	}
	alertStore := store.New(alertsPath, 0)

	mux := web.NewRouter(cfg, alertStore)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Prometheus Webhook for Feishu 已启动，监听 :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

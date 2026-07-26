package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// ConfigState 线程安全地保存运行时配置（来自 config.json）。
// 通过环境变量可在启动时覆盖关键字段，便于容器化部署。
type ConfigState struct {
	mu       sync.RWMutex
	data     map[string]interface{}
	filePath string
}

// LoadConfig 读取 config.json，并应用环境变量覆盖。
func LoadConfig(path string) (*ConfigState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	c := &ConfigState{data: m, filePath: path}
	c.applyEnvOverrides()
	return c, nil
}

// applyEnvOverrides 允许通过环境变量覆盖配置（便于 Docker / K8s 部署）。
func (c *ConfigState) applyEnvOverrides() {
	overrides := map[string]string{
		"FEISHU_USERNAME":       "USERNAME",
		"FEISHU_PASSWORD":       "PASSWORD",
		"FEISHU_WEBHOOK_URL":    "FEISHU_WEBHOOK_URL",
		"FEISHU_FIRING_TITLE":   "FIRING_TITLE",
		"FEISHU_RESOLVED_TITLE": "RESOLVED_TITLE",
	}
	for env, key := range overrides {
		if v := os.Getenv(env); v != "" {
			c.Set(key, v)
			log.Printf("配置项 %s 已被环境变量 %s 覆盖", key, env)
		}
	}
}

func (c *ConfigState) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.data[key].(string); ok {
		return v
	}
	return ""
}

func (c *ConfigState) Set(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = val
}

// Template 返回解析后的卡片模板（interface{} 结构）。
func (c *ConfigState) Template() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data["FEISHU_CARD_TEMPLATE"]
}

func (c *ConfigState) SetTemplate(t interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data["FEISHU_CARD_TEMPLATE"] = t
}

// Save 将当前配置写回 config.json（管理页面保存时调用）。
func (c *ConfigState) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, err := json.MarshalIndent(c.data, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, b, 0644)
}

package feishu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muzihuaner/prometheus-webhook-feishu/internal/config"
)

func testCtx() *tmplCtx {
	return &tmplCtx{
		tmplAlert: tmplAlert{
			Status: "firing",
			Labels: KV{
				"alertname": "HighCPU",
				"severity":  "warning",
				"cluster":   "prod-cluster",
				"namespace": "middleware",
			},
			Annotations: KV{"description": "CPU 过高"},
			StartsAt:    "2026-09-09T01:02:03Z",
			EndsAt:      "0001-01-01T00:00:00Z",
		},
		Alerts: alertList{
			{Status: "firing", Labels: KV{"a": "1"}},
			{Status: "resolved", Labels: KV{"b": "2"}},
			{Status: "firing", Labels: KV{"c": "3"}},
		},
		GroupLabels:  KV{"alertname": "HighCPU"},
		CommonLabels: KV{"job": "node"},
		ExternalURL:  "https://am.example.com",
		Receiver:     "feishu",
	}
}

func TestKVMethods(t *testing.T) {
	kv := KV{"severity": "warning", "alertname": "HighCPU", "instance": "1.2.3.4"}
	wantNames := []string{"alertname", "instance", "severity"}
	got := kv.Names()
	if len(got) != len(wantNames) {
		t.Fatalf("Names() = %v, want %v", got, wantNames)
	}
	for i, n := range got {
		if n != wantNames[i] {
			t.Fatalf("Names()[%d] = %s, want %s", i, n, wantNames[i])
		}
	}
	vals := kv.Values()
	if vals[0] != "HighCPU" || vals[1] != "1.2.3.4" || vals[2] != "warning" {
		t.Fatalf("Values() = %v", vals)
	}
	pairs := kv.SortedPairs()
	if len(pairs) != 3 || pairs[0].Name != "alertname" || pairs[0].Value != "HighCPU" {
		t.Fatalf("SortedPairs() = %v", pairs)
	}
	removed := kv.Remove("severity", "summary")
	if _, ok := removed["severity"]; ok {
		t.Fatal("Remove 未移除 severity")
	}
	if len(removed) != 2 {
		t.Fatalf("Remove 后长度 = %d, want 2", len(removed))
	}
}

func TestAlertListSubsets(t *testing.T) {
	list := alertList{
		{Status: "firing"},
		{Status: "resolved"},
		{Status: "firing"},
		{Status: "resolved"},
	}
	if got := len(list.Firing()); got != 2 {
		t.Fatalf("Firing() 长度 = %d, want 2", got)
	}
	if got := len(list.Resolved()); got != 2 {
		t.Fatalf("Resolved() 长度 = %d, want 2", got)
	}
}

func TestRenderString(t *testing.T) {
	ctx := testCtx()
	values := map[string]string{"alertname": "HighCPU", "severity": "warning"}
	cases := []struct {
		name, src, want string
		ctx             *tmplCtx // 为 nil 时使用 ctx
		values          map[string]string // 为 nil 时使用 values
		wantErr         bool
	}{
		{name: "占位符替换", src: "{alertname}", want: "HighCPU"},
		{name: "标签访问", src: "{{ .Labels.cluster }}", want: "prod-cluster"},
		{
			name: "条件分支-正式",
			src:  `{{ if and (eq .Labels.cluster "prod-cluster") (eq .Labels.namespace "middleware") }}项目A{{ else if and (eq .Labels.cluster "staging-cluster") (eq .Labels.namespace "middleware") }}项目B{{ else }}未知{{ end }}`,
			want: "项目A",
		},
		{
			name: "条件分支-测试",
			src:  `{{ if and (eq .Labels.cluster "prod-cluster") (eq .Labels.namespace "middleware") }}项目A{{ else if and (eq .Labels.cluster "staging-cluster") (eq .Labels.namespace "middleware") }}项目B{{ else }}未知{{ end }}`,
			want: "项目B",
			ctx: &tmplCtx{tmplAlert: tmplAlert{Labels: KV{"cluster": "staging-cluster", "namespace": "middleware"}}},
		},
		{
			name: "条件分支-其他",
			src:  `{{ if and (eq .Labels.cluster "prod-cluster") (eq .Labels.namespace "middleware") }}项目A{{ else if and (eq .Labels.cluster "staging-cluster") (eq .Labels.namespace "middleware") }}项目B{{ else }}未知{{ end }}`,
			want: "未知",
			ctx:  &tmplCtx{tmplAlert: tmplAlert{Labels: KV{"cluster": "other", "namespace": "x"}}},
		},
		{name: "模板与占位符混合", src: "{{ .Labels.severity }}/{alertname}", want: "warning/HighCPU"},
		// 模板输出本身构成占位符形状文本时，占位符替换在模板渲染之后执行，
		// 因此模板产出的 {instance} 也会被替换（先模板后占位符）。
		{
			name: "模板输出引入占位符形状文本",
			src:  "{{ .Labels.name }} {alertname}",
			want: "Y X",
			ctx:  &tmplCtx{tmplAlert: tmplAlert{Labels: KV{"name": "{instance}"}}},
			values: map[string]string{"alertname": "X", "instance": "Y"},
		},
		{name: "Firing 计数", src: "{{ .Alerts.Firing | len }}", want: "2"},
		{name: "SortedPairs 遍历", src: "{{ range .Labels.SortedPairs }}{{ .Name }}={{ .Value }};{{ end }}", want: "alertname=HighCPU;cluster=prod-cluster;namespace=middleware;severity=warning;"},
		{name: "toUpper", src: "{{ .Labels.severity | toUpper }}", want: "WARNING"},
		{name: "通知级字段", src: "{{ .GroupLabels.alertname }}/{{ .CommonLabels.job }}/{{ .ExternalURL }}/{{ .Receiver }}", want: "HighCPU/node/https://am.example.com/feishu"},
		{name: "date 转东八区", src: `{{ .StartsAt | date "2006-01-02 15:04:05" }}`, want: "2026-09-09 09:02:03"},
		{name: "非法模板", src: "{{ .Labels", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.ctx
			if c == nil {
				c = ctx
			}
			v := tc.values
			if v == nil {
				v = values
			}
			got, err := renderString(tc.src, v, c)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望错误，实际得到 %q", got)
				}
				if !strings.Contains(err.Error(), "模板解析失败") {
					t.Fatalf("错误信息 = %v, 应包含 模板解析失败", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("renderString 错误: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	// 同一非法模板执行两次都应报错（错误不缓存）
	if _, err := execTemplate("{{ .Labels", ctx); err == nil {
		t.Fatal("第一次 execTemplate 应报错")
	}
	if _, err := execTemplate("{{ .Labels", ctx); err == nil {
		t.Fatal("第二次 execTemplate 应报错")
	}
}

func TestFillTemplateRecursive(t *testing.T) {
	ctx := testCtx()
	values := map[string]string{"alertname": "HighCPU"}
	in := map[string]interface{}{
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": "{{ .Labels.cluster }}/{alertname}",
				},
				"count":  3,
				"enable": true,
			},
			"plain-{alertname}",
		},
	}
	out, err := fillTemplate(in, values, ctx)
	if err != nil {
		t.Fatalf("fillTemplate 错误: %v", err)
	}
	els := out.(map[string]interface{})["elements"].([]interface{})
	text := els[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if text != "prod-cluster/HighCPU" {
		t.Fatalf("嵌套内容 = %q, want prod-cluster/HighCPU", text)
	}
	if els[0].(map[string]interface{})["count"] != 3 {
		t.Fatal("int 类型不应被修改")
	}
	if els[0].(map[string]interface{})["enable"] != true {
		t.Fatal("bool 类型不应被修改")
	}
	if els[1] != "plain-HighCPU" {
		t.Fatalf("切片元素 = %q", els[1])
	}
}

func newTestCfg(t *testing.T) *config.ConfigState {
	t.Helper()
	dir := t.TempDir()
	cfgJSON := `{"FEISHU_CARD_TEMPLATE":{"card":{"elements":[{"tag":"div","text":{"tag":"lark_md","content":"{{ .Labels.cluster }}/{alertname}"}}]}}}`
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(cfgJSON), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestBuildCard(t *testing.T) {
	cfg := newTestCfg(t)
	payload := &WebhookPayload{
		Status: "firing",
		Alerts: []Alert{
			{
				Status:   "firing",
				Labels:   map[string]string{"alertname": "A1", "cluster": "prod-cluster"},
				StartsAt: "2026-09-09T01:02:03Z",
			},
			{
				Status:   "resolved",
				Labels:   map[string]string{"alertname": "A2", "cluster": "staging-cluster"},
				StartsAt: "2026-09-09T01:02:03Z",
			},
		},
	}
	card, err := BuildCard(payload, cfg)
	if err != nil {
		t.Fatalf("BuildCard 错误: %v", err)
	}
	els := card["card"].(map[string]interface{})["elements"].([]interface{})
	if len(els) != 2 {
		t.Fatalf("elements 长度 = %d, want 2", len(els))
	}
	c1 := els[0].(map[string]interface{})["text"].(map[string]interface{})["content"]
	c2 := els[1].(map[string]interface{})["text"].(map[string]interface{})["content"]
	if c1 != "prod-cluster/A1" {
		t.Fatalf("第一条 = %q, want prod-cluster/A1", c1)
	}
	if c2 != "staging-cluster/A2" {
		t.Fatalf("第二条 = %q, want staging-cluster/A2", c2)
	}
}

// TestBuildCardNilLabels 验证告警与通知级标签均为 nil 时不 panic：
// 模板中对 nil KV 的字段访问返回空字符串（走 else 分支输出 无cluster）。
// 注意实际行为：values map 中 alertname 键恒存在（nil Labels 时值为 ""），
// replacePlaceholders 对"键存在但值为空串"的情况会替换为空串，
// 因此 {alertname} 最终被替换为 "" 而非原样保留。
func TestBuildCardNilLabels(t *testing.T) {
	cfg := newTestCfg(t)
	cfg.SetTemplate(map[string]interface{}{
		"card": map[string]interface{}{
			"elements": []interface{}{
				map[string]interface{}{
					"tag": "div",
					"text": map[string]interface{}{
						"content": `{{ if .Labels.cluster }}{{ .Labels.cluster }}{{ else }}无cluster{{ end }}/{alertname}`,
					},
				},
			},
		},
	})
	payload := &WebhookPayload{Status: "firing", Alerts: []Alert{{Status: "firing"}}}
	card, err := BuildCard(payload, cfg)
	if err != nil {
		t.Fatalf("BuildCard 错误: %v", err)
	}
	content := card["card"].(map[string]interface{})["elements"].([]interface{})[0].
		(map[string]interface{})["text"].(map[string]interface{})["content"]
	if content != "无cluster/" {
		t.Fatalf("content = %q, want 无cluster/ （{alertname} 被替换为空串）", content)
	}
}

func TestBuildCardTemplateError(t *testing.T) {
	cfg := newTestCfg(t)
	cfg.SetTemplate(map[string]interface{}{
		"card": map[string]interface{}{
			"elements": []interface{}{
				map[string]interface{}{"content": "{{ broken"},
			},
		},
	})
	payload := &WebhookPayload{
		Status: "firing",
		Alerts: []Alert{{Status: "firing", Labels: map[string]string{"alertname": "A1"}}},
	}
	if _, err := BuildCard(payload, cfg); err == nil {
		t.Fatal("模板非法时应返回错误")
	}
}

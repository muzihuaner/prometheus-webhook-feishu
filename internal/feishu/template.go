package feishu

import (
	"bytes"
	"fmt"
	"html"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
)

// KV / Pair / alertList / tmplCtx 提供 Alertmanager 兼容的模板上下文。
type Pair struct{ Name, Value string }
type KV map[string]string

// Names 返回按名称排序的键列表。
func (kv KV) Names() []string {
	pairs := kv.SortedPairs()
	names := make([]string, len(pairs))
	for i, p := range pairs {
		names[i] = p.Name
	}
	return names
}

// Values 返回按 Names 顺序排列的值列表。
func (kv KV) Values() []string {
	pairs := kv.SortedPairs()
	vals := make([]string, len(pairs))
	for i, p := range pairs {
		vals[i] = p.Value
	}
	return vals
}

// SortedPairs 返回按 Name 排序的键值对。
func (kv KV) SortedPairs() []Pair {
	names := make([]string, 0, len(kv))
	for k := range kv {
		names = append(names, k)
	}
	sort.Strings(names)
	pairs := make([]Pair, 0, len(names))
	for _, n := range names {
		pairs = append(pairs, Pair{Name: n, Value: kv[n]})
	}
	return pairs
}

// Remove 返回不含指定键的新 KV。
func (kv KV) Remove(names ...string) KV {
	drop := make(map[string]struct{}, len(names))
	for _, n := range names {
		drop[n] = struct{}{}
	}
	out := make(KV, len(kv))
	for k, v := range kv {
		if _, ok := drop[k]; !ok {
			out[k] = v
		}
	}
	return out
}

type tmplAlert struct {
	Status       string
	StartsAt     string
	EndsAt       string
	GeneratorURL string
	Labels       KV
	Annotations  KV
}

type alertList []tmplAlert

// Firing 返回状态为 firing 的子集。
func (l alertList) Firing() alertList { return l.filter("firing") }

// Resolved 返回状态为 resolved 的子集。
func (l alertList) Resolved() alertList { return l.filter("resolved") }

func (l alertList) filter(status string) alertList {
	out := make(alertList, 0, len(l))
	for _, a := range l {
		if a.Status == status {
			out = append(out, a)
		}
	}
	return out
}

// tmplCtx 是模板渲染上下文：嵌入当前告警，并附带通知级信息。
type tmplCtx struct {
	tmplAlert
	Alerts       alertList
	GroupLabels  KV
	CommonLabels KV
	ExternalURL  string
	Receiver     string
}

// funcMap 提供模板辅助函数（对齐 Alertmanager 常用用法）。
var funcMap = template.FuncMap{
	"toUpper":  strings.ToUpper,
	"toLower":  strings.ToLower,
	"join":     func(sep string, s []string) string { return strings.Join(s, sep) },
	"html":     html.EscapeString,
	"markdown": func(s string) string { return mdEscaper.Replace(s) },
	"date": func(layout string, v interface{}) string {
		var t time.Time
		switch x := v.(type) {
		case time.Time:
			t = x
		case string:
			parsed, err := time.Parse(time.RFC3339, x)
			if err != nil {
				if p2, err2 := time.Parse("2006-01-02T15:04:05", x); err2 == nil {
					t = p2
				} else {
					return x // 解析失败原样返回
				}
			} else {
				t = parsed
			}
		default:
			return fmt.Sprintf("%v", v)
		}
		cst := time.FixedZone("CST", 8*3600)
		return t.In(cst).Format(layout)
	},
}

// mdEscaper 转义飞书 lark_md 的特殊字符。
var mdEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"`", "\\`",
	"*", "\\*",
	"_", "\\_",
	"~", "\\~",
	"[", "\\[",
	"]", "\\]",
)

// tmplCache 缓存已解析的模板，避免重复 Parse。
var tmplCache sync.Map // string -> *template.Template

// execTemplate 解析并执行模板（解析结果缓存，解析错误不缓存）。
func execTemplate(src string, ctx *tmplCtx) (string, error) {
	var tmpl *template.Template
	if cached, ok := tmplCache.Load(src); ok {
		tmpl = cached.(*template.Template)
	} else {
		var err error
		tmpl, err = template.New("card").Funcs(funcMap).Parse(src)
		if err != nil {
			return "", fmt.Errorf("模板解析失败: %w", err)
		}
		tmplCache.Store(src, tmpl)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("模板渲染失败: %w", err)
	}
	return buf.String(), nil
}

// renderString 先渲染 Go 模板（若包含 {{），再做 {key} 占位符替换，保证向后兼容。
func renderString(s string, values map[string]string, ctx *tmplCtx) (string, error) {
	if strings.Contains(s, "{{") {
		var err error
		s, err = execTemplate(s, ctx)
		if err != nil {
			return "", err
		}
	}
	return replacePlaceholders(s, values), nil
}

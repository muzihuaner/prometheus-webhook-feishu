package store

import (
	"encoding/json"
	"math/rand"
	"os"
	"sync"
	"time"
)

// PushStatus 表示一次告警发送到飞书的结果状态。
type PushStatus string

const (
	PushSuccess PushStatus = "success" // 发送成功
	PushFailed  PushStatus = "failed"  // 发送失败（飞书侧返回错误）
	PushError   PushStatus = "error"   // 程序异常（构建/解析失败等）
)

// AlertSummary 是一条告警的精简摘要，用于历史记录与详情展示。
type AlertSummary struct {
	AlertName string `json:"alertname"`
	Severity  string `json:"severity"`
	Instance  string `json:"instance"`
	Summary   string `json:"summary"`
}

// Record 表示一次接收到的 webhook 事件及其推送结果。
type Record struct {
	ID         string        `json:"id"`
	ReceivedAt time.Time     `json:"received_at"`
	Status     string        `json:"status"`      // firing / resolved
	Count      int           `json:"count"`       // 本次包含的告警条数
	AlertName  string        `json:"alertname"`   // 主告警名称（便于列表查看）
	Severity   string        `json:"severity"`    // 主告警等级
	PushStatus PushStatus    `json:"push_status"` // success / failed / error
	Detail     string        `json:"detail"`      // 错误信息或成功描述
	Alerts     []AlertSummary `json:"alerts"`     // 本次所有告警的摘要
}

// Store 在内存中保存告警历史，并异步持久化到 JSON 文件，进程重启后不丢失。
type Store struct {
	mu       sync.RWMutex
	pMu      sync.Mutex // 保护文件写入，避免并发写冲突
	records  []Record
	maxSize  int
	filePath string
}

// New 创建告警历史存储；filePath 为空时不持久化，maxSize<=0 时默认保留 500 条。
func New(filePath string, maxSize int) *Store {
	if maxSize <= 0 {
		maxSize = 500
	}
	s := &Store{maxSize: maxSize, filePath: filePath}
	s.load()
	return s
}

func (s *Store) load() {
	if s.filePath == "" {
		return
	}
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var recs []Record
	if json.Unmarshal(b, &recs) == nil {
		s.records = recs
	}
}

func genID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return time.Now().Format("20060102150405") + "-" + string(b)
}

// Add 追加一条告警历史记录，并在后台持久化。
func (s *Store) Add(r Record) {
	s.mu.Lock()
	if r.ID == "" {
		r.ID = genID()
	}
	if r.ReceivedAt.IsZero() {
		r.ReceivedAt = time.Now()
	}
	s.records = append(s.records, r)
	if len(s.records) > s.maxSize {
		s.records = s.records[len(s.records)-s.maxSize:]
	}
	snapshot := make([]Record, len(s.records))
	copy(snapshot, s.records)
	s.mu.Unlock()

	s.persist(snapshot)
}

func (s *Store) persist(snapshot []Record) {
	if s.filePath == "" {
		return
	}
	s.pMu.Lock()
	defer s.pMu.Unlock()
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.filePath)
}

// List 返回全部历史记录（最新在前）。
func (s *Store) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	// 反转，使最新记录在最前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Counts 返回各推送状态的数量统计，用于仪表盘展示。
func (s *Store) Counts() (success, failed, errCount int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.records {
		switch r.PushStatus {
		case PushSuccess:
			success++
		case PushFailed:
			failed++
		case PushError:
			errCount++
		}
	}
	return
}

// Clear 清空全部历史记录。
func (s *Store) Clear() error {
	s.mu.Lock()
	s.records = nil
	s.mu.Unlock()
	if s.filePath == "" {
		return nil
	}
	s.pMu.Lock()
	defer s.pMu.Unlock()
	return os.WriteFile(s.filePath, []byte("[]"), 0644)
}

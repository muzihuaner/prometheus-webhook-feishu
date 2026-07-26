package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
)

const sessionCookieName = "fw_session"
const flashCookieName = "fw_flash"

var sessionSecret []byte

// InitSecret 初始化会话签名密钥：优先使用环境变量 SESSION_SECRET，否则随机生成。
func InitSecret() {
	if s := os.Getenv("SESSION_SECRET"); s != "" {
		sessionSecret = []byte(s)
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退化为固定密钥（仅开发用）
		sessionSecret = []byte("prometheus-webhook-feishu-dev-secret")
		return
	}
	sessionSecret = b
}

func signSession(value string) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(value))
	return value + "." + base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

func verifySession(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(parts[0]))
	expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

// SetSessionCookie 写入已签名的登录会话 Cookie。
func SetSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signSession("authenticated"),
		Path:     "/",
		HttpOnly: true,
		MaxAge:   60 * 60 * 24 * 7, // 7 天
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func IsAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return verifySession(c.Value)
}

// SetFlash 通过 Cookie 设置一次性提示消息。
func SetFlash(w http.ResponseWriter, message, kind string) {
	val := kind + "|" + message
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   10,
	})
}

// ConsumeFlash 读取并清除一次性提示消息。
func ConsumeFlash(r *http.Request, w http.ResponseWriter) (message, kind string) {
	c, err := r.Cookie(flashCookieName)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(c.Value, "|", 2)
	kind = parts[0]
	if len(parts) == 2 {
		message = parts[1]
	}
	// 立即清除
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	return message, kind
}

// RequireAuth 是一个中间件，要求登录后才能访问。
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

package svc

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// isTrustedProxy 判断请求直连方是否为可信代理（本机/内网），与 utiles.IsTrustedProxyPeer 逻辑一致。
func isTrustedProxy(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

const (
	rateLimitWindow      = 1 * time.Minute
	rateLimitMax         = 120 // 每个窗口最大请求数
	rateLimiterCleanup   = 5 * time.Minute
	rateLimiterMaxEntries = 4096
)

type rateEntry struct {
	count    int
	windowStart time.Time
}

// RateLimiter 滑动窗口全局限流器，按客户端 IP 限速。
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{entries: make(map[string]*rateEntry)}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	if rl == nil {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	// 超过上限时清理过期条目
	if len(rl.entries) >= rateLimiterMaxEntries {
		now := time.Now()
		for k, e := range rl.entries {
			if now.Sub(e.windowStart) > rateLimitWindow {
				delete(rl.entries, k)
			}
		}
	}
	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.Sub(e.windowStart) > rateLimitWindow {
		rl.entries[ip] = &rateEntry{count: 1, windowStart: now}
		return true
	}
	if e.count >= rateLimitMax {
		return false
	}
	e.count++
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rateLimiterCleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, e := range rl.entries {
			if now.Sub(e.windowStart) > rateLimitWindow {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware 全局限流中间件，超限返回 429。
func RateLimitMiddleware(rl *RateLimiter) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			// 仅当直连方为可信代理时才信任 X-Forwarded-For，与 clientIP 逻辑一致
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" && isTrustedProxy(r.RemoteAddr) {
				// 取最后一个由代理追加的地址（最远客户端）
				if idx := strings.LastIndex(fwd, ","); idx >= 0 {
					ip = strings.TrimSpace(fwd[idx+1:])
				} else {
					ip = strings.TrimSpace(fwd)
				}
			}
			if !rl.Allow(ip) {
				logx.Infof("rate limit exceeded for %s", ip)
				httpx.WriteJson(w, http.StatusTooManyRequests, map[string]interface{}{
					"code": 429,
					"msg":  "请求过于频繁，请稍后再试",
				})
				return
			}
			next(w, r)
		}
	}
}

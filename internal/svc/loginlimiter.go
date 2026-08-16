package svc

import (
	"sync"
	"time"
)

const (
	loginMaxFailures       = 5
	loginFailureWindow     = 10 * time.Minute
	loginLockDuration      = 15 * time.Minute
	loginLimiterMaxEntries = 4096
)

type loginFailEntry struct {
	failCount int
	firstFail time.Time
	lockUntil time.Time
}

// LoginLimiter 登录失败限速器：按客户端 IP 记录失败次数，超限后锁定一段时间。
// 零值可直接使用。
type LoginLimiter struct {
	mu    sync.Mutex
	fails map[string]*loginFailEntry
}

// Allow 返回该 IP 当前是否允许继续尝试登录。
func (l *LoginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fails == nil {
		return true
	}
	e, ok := l.fails[ip]
	if !ok {
		return true
	}
	return time.Now().After(e.lockUntil)
}

// RecordFailure 记录一次失败；连续失败达到阈值后锁定。
func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fails == nil {
		l.fails = make(map[string]*loginFailEntry)
	}
	if len(l.fails) >= loginLimiterMaxEntries {
		l.pruneStale()
	}
	now := time.Now()
	e, ok := l.fails[ip]
	if !ok || now.Sub(e.firstFail) > loginFailureWindow {
		l.fails[ip] = &loginFailEntry{failCount: 1, firstFail: now}
		return
	}
	e.failCount++
	if e.failCount >= loginMaxFailures {
		e.lockUntil = now.Add(loginLockDuration)
	}
}

// Reset 登录成功后清零该 IP 的失败记录。
func (l *LoginLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fails != nil {
		delete(l.fails, ip)
	}
}

func (l *LoginLimiter) pruneStale() {
	now := time.Now()
	cutoff := loginFailureWindow + loginLockDuration
	for ip, e := range l.fails {
		if now.Sub(e.firstFail) > cutoff && now.After(e.lockUntil) {
			delete(l.fails, ip)
		}
	}
}

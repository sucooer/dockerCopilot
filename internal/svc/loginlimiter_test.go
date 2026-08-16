package svc

import (
	"testing"
	"time"
)

func TestLoginLimiterLockout(t *testing.T) {
	var limiter LoginLimiter
	ip := "192.168.1.1"

	for i := 0; i < loginMaxFailures; i++ {
		if !limiter.Allow(ip) {
			t.Fatalf("attempt %d should be allowed before lockout", i+1)
		}
		limiter.RecordFailure(ip)
	}
	if limiter.Allow(ip) {
		t.Fatal("expected lockout after max failures")
	}

	limiter.Reset(ip)
	if !limiter.Allow(ip) {
		t.Fatal("expected allow after reset")
	}
}

func TestLoginLimiterWindowReset(t *testing.T) {
	var limiter LoginLimiter
	ip := "192.168.1.2"

	limiter.RecordFailure(ip)
	e := limiter.fails[ip]
	e.firstFail = time.Now().Add(-loginFailureWindow - time.Minute)

	if !limiter.Allow(ip) {
		t.Fatal("stale entry should not cause lockout")
	}
	limiter.RecordFailure(ip)
	if limiter.fails[ip].failCount != 1 {
		t.Fatalf("expected failure window to reset count, got %d", limiter.fails[ip].failCount)
	}
}

func TestLoginLimiterIndependentIPs(t *testing.T) {
	var limiter LoginLimiter
	for i := 0; i < loginMaxFailures; i++ {
		limiter.RecordFailure("1.1.1.1")
	}
	if limiter.Allow("1.1.1.1") {
		t.Fatal("1.1.1.1 should be locked")
	}
	if !limiter.Allow("2.2.2.2") {
		t.Fatal("2.2.2.2 should not be affected")
	}
}

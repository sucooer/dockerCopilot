package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresXFFFromPublicPeer(t *testing.T) {
	// 公网直连：伪造 XFF 也不应被信任
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "203.0.113.10:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := clientIP(r); got != "203.0.113.10" {
		t.Errorf("expected RemoteAddr, got %q", got)
	}
}

func TestClientIPTrustsXFFFromPrivatePeer(t *testing.T) {
	// 内网代理（如 nginx）：取 XFF 最后一个由代理追加的地址
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "172.18.0.2:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.5")
	if got := clientIP(r); got != "203.0.113.5" {
		t.Errorf("expected last XFF entry, got %q", got)
	}
}

func TestClientIPLoopbackPeer(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("expected XFF for loopback proxy, got %q", got)
	}
}

func TestClientIPNoXFF(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "10.0.0.8:5555"
	if got := clientIP(r); got != "10.0.0.8" {
		t.Errorf("expected RemoteAddr, got %q", got)
	}
}

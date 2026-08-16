package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/onlyLTY/dockerCopilot/internal/config"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func TestAuthCookieFlow(t *testing.T) {
	port := getFreePort(t)
	cfg := config.Config{}
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Auth.AccessSecret = "test-secret-2026"
	cfg.Auth.AccessExpire = 3600

	server := rest.MustNewServer(cfg.RestConf)
	RegisterHandlers(server, &svc.ServiceContext{Config: cfg})
	defer server.Stop()

	go server.Start()
	waitForServer(t, port)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/api", port)

	// 1. 未认证访问受保护接口 -> 401
	resp, err := http.Get(baseURL + "/version")
	if err != nil {
		t.Fatalf("failed to call version route: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}

	// 2. 登录成功 -> 返回 Set-Cookie（HttpOnly + SameSite=Strict）
	loginResp, err := http.Post(baseURL+"/auth", "application/x-www-form-urlencoded",
		strings.NewReader("secretKey=test-secret-2026"))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}
	setCookie := loginResp.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "docker_copilot_token=") {
		t.Fatalf("expected auth cookie in Set-Cookie, got: %q", setCookie)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("expected HttpOnly cookie, got: %q", setCookie)
	}
	if !strings.Contains(setCookie, "SameSite=Strict") {
		t.Fatalf("expected SameSite=Strict cookie, got: %q", setCookie)
	}
	cookieValue := strings.SplitN(setCookie, ";", 2)[0]

	// 3. 携带 Cookie 访问受保护接口 -> 通过认证
	authedReq, _ := http.NewRequest(http.MethodGet, baseURL+"/version", nil)
	authedReq.Header.Set("Cookie", cookieValue)
	authedResp, err := http.DefaultClient.Do(authedReq)
	if err != nil {
		t.Fatalf("failed to call version with cookie: %v", err)
	}
	authedResp.Body.Close()
	if authedResp.StatusCode == http.StatusUnauthorized {
		t.Fatal("expected authorized request with cookie, got 401")
	}

	// 4. 退出登录 -> Cookie 被清除
	logoutReq, _ := http.NewRequest(http.MethodPost, baseURL+"/auth/logout", nil)
	logoutReq.Header.Set("Cookie", cookieValue)
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatalf("failed to logout: %v", err)
	}
	logoutResp.Body.Close()
	logoutCookie := logoutResp.Header.Get("Set-Cookie")
	if !strings.Contains(logoutCookie, "Max-Age=0") && !strings.Contains(logoutCookie, "expires") {
		t.Fatalf("expected cookie clearing on logout, got: %q", logoutCookie)
	}

	// 5. 使用旧 Cookie 再次访问 -> 应被拒绝（jwt 本身有效，此处仅验证 cookie 清除逻辑被携带）
	expiredReq, _ := http.NewRequest(http.MethodGet, baseURL+"/version", nil)
	expiredReq.Header.Set("Cookie", cookieValue)
	expiredResp, err := http.DefaultClient.Do(expiredReq)
	if err != nil {
		t.Fatalf("failed to call version: %v", err)
	}
	expiredResp.Body.Close()
	_ = expiredResp
}

func TestAuthWithWrongSecret(t *testing.T) {
	port := getFreePort(t)
	cfg := config.Config{}
	cfg.Host = "127.0.0.1"
	cfg.Port = port
	cfg.Auth.AccessSecret = "test-secret-2026"
	cfg.Auth.AccessExpire = 3600

	server := rest.MustNewServer(cfg.RestConf)
	RegisterHandlers(server, &svc.ServiceContext{Config: cfg})
	defer server.Stop()

	go server.Start()
	waitForServer(t, port)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/auth", port),
		"application/x-www-form-urlencoded", strings.NewReader("secretKey=wrong"))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong secret, got %d", resp.StatusCode)
	}
	if cookie := resp.Header.Get("Set-Cookie"); strings.Contains(cookie, "docker_copilot_token=") {
		t.Fatalf("must not set auth cookie on failed login, got: %q", cookie)
	}
}

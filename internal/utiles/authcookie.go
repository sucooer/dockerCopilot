package utiles

import (
	"net"
	"net/http"
)

const JwtCookieName = "docker_copilot_token"

// IsTrustedProxyPeer 判断请求直连方是否为可信代理（本机/内网）。
// 公网直连时 X-Forwarded-* 头可被客户端伪造，不得信任。
func IsTrustedProxyPeer(remoteAddr string) bool {
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

// isHTTPSRequest 判断客户端是否为 HTTPS 请求。
// 仅当直连方为可信代理时才信任 X-Forwarded-Proto，防止公网直连伪造。
func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https" && IsTrustedProxyPeer(r.RemoteAddr)
}

// SetAuthCookie 签发 HttpOnly + SameSite=Strict 的 JWT Cookie。
func SetAuthCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     JwtCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge),
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearAuthCookie 清除 JWT Cookie，用于退出登录。
// 需要传入 request 以判断是否使用 Secure 标志（与 SetAuthCookie 一致）。
func ClearAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     JwtCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

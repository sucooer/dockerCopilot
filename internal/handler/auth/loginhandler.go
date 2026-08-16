package auth

import (
	"github.com/onlyLTY/dockerCopilot/internal/logic/auth"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net"
	"net/http"
	"strings"
)

func LoginHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			var resp types.Resp
			resp.Code = 400
			resp.Msg = "错误的请求"
			httpx.WriteJson(w, 400, resp)
			return
		}
		ip := clientIP(r)
		if !ctx.LoginLimiter.Allow(ip) {
			httpx.WriteJson(w, http.StatusTooManyRequests, types.Resp{
				Code: 429,
				Msg:  "登录失败次数过多，请稍后再试",
				Data: map[string]interface{}{},
			})
			return
		}
		l := auth.NewLoginLogic(r.Context(), ctx)
		resp, err := l.Login(&req)
		if err != nil {
			// 仅对"无效密钥"这类业务失败计数，内部错误（500）不计数，
			// 避免服务故障时触发自锁
			if resp.Code == 401 {
				ctx.LoginLimiter.RecordFailure(ip)
			}
			httpx.WriteJson(w, resp.Code, resp)
			return
		}
		ctx.LoginLimiter.Reset(ip)
		if resp.Code == 200 {
			if jr, ok := resp.Data.(auth.JwtResponse); ok && jr.Jwt != "" {
				utiles.SetAuthCookie(w, r, jr.Jwt, ctx.Config.Auth.AccessExpire)
			}
		}
		httpx.OkJson(w, resp)
	}
}

// LogoutHandler 清除 JWT Cookie，实现退出登录。
func LogoutHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utiles.ClearAuthCookie(w, r)
		httpx.OkJson(w, types.Resp{
			Code: 200,
			Msg:  "success",
			Data: map[string]interface{}{},
		})
	}
}

// clientIP 获取客户端真实 IP。
// 仅当直连方为可信代理（本机/内网，如 docker 网络中的 nginx）时才信任
// X-Forwarded-For，并取最后一个由代理追加的地址，防止公网直连时伪造绕过限流。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" && utiles.IsTrustedProxyPeer(r.RemoteAddr) {
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
				return last
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

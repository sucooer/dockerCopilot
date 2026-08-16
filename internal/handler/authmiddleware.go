package handler

import (
	"net/http"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/handler"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type UnauthorizedResponse struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data map[string]interface{} `json:"data"`
}

func unauthorizedCallback(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteJson(w, http.StatusUnauthorized, UnauthorizedResponse{
		Code: http.StatusUnauthorized,
		Msg:  "未授权",
		Data: map[string]interface{}{},
	})
}

// CookieJwtMiddleware 优先从 Authorization 头读取 JWT，缺失时回退到 HttpOnly Cookie。
func CookieJwtMiddleware(ctx *svc.ServiceContext) rest.Middleware {
	authorize := handler.Authorize(ctx.Config.Auth.AccessSecret,
		handler.WithUnauthorizedCallback(unauthorizedCallback))
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				if c, err := r.Cookie(utiles.JwtCookieName); err == nil && c.Value != "" {
					r.Header.Set("Authorization", "Bearer "+c.Value)
				}
			}
			authorize(next).ServeHTTP(w, r)
		}
	}
}

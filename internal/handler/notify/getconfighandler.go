package notify

import (
	"net/http"

	"github.com/onlyLTY/dockerCopilot/internal/logic/notify"
	"github.com/onlyLTY/dockerCopilot/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := notify.NewGetConfigLogic(r.Context(), svcCtx)
		resp, err := l.GetConfig()
		if err != nil {
			httpx.WriteJson(w, resp.Code, resp)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

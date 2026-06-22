package notify

import (
	"net/http"

	"github.com/onlyLTY/dockerCopilot/internal/logic/notify"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.NotifyConfig
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := notify.NewUpdateConfigLogic(r.Context(), svcCtx)
		resp, err := l.UpdateConfig(&req)
		if err != nil {
			httpx.WriteJson(w, resp.Code, resp)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

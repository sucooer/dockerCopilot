package restartschedule

import (
	"net/http"

	logic "github.com/onlyLTY/dockerCopilot/internal/logic/restartschedule"
	"github.com/onlyLTY/dockerCopilot/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func RunHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewRunLogic(r.Context(), svcCtx)
		resp, err := l.Run()
		if err != nil {
			httpx.WriteJson(w, resp.Code, resp)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}